package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type resourceRepository struct{ pool *pgxpool.Pool }

// NewResourceRepository returns a repository for the unified Resource Master
// schema (recursos-maestro; formerly Materials Master). Every query is
// scoped by the owning class (design R1): Get's first argument is a class
// code, not a family code — the two are never interchangeable.
func NewResourceRepository(pool *pgxpool.Pool) domain.ResourceRepository {
	return &resourceRepository{pool: pool}
}

func (r *resourceRepository) Create(ctx context.Context, resource domain.Resource) error {
	if r.pool == nil {
		return errors.New("resource repository: nil pool")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin resource create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var resourceID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO public.recursos (class_id, family_id, type_id, natural_unit_id, display_name, identity_key)
		SELECT cl.id, f.id, t.id, u.id, $5, $6
		FROM public.resource_classes cl
		JOIN public.resource_families f ON f.class_id = cl.id AND f.code = $2
		JOIN public.resource_types t ON t.family_id = f.id AND t.code = $3
		JOIN public.resource_unit_policies p ON p.family_id = f.id AND p.allowed
		JOIN public.unit_definitions u ON u.id = p.unit_id AND u.code = $4
		WHERE cl.code = $1 AND cl.active AND f.active AND t.active AND u.active
		RETURNING id`, resource.ClassCode, resource.FamilyCode, resource.TypeCode, resource.NaturalUnit, resource.FamilyCode, resource.IdentityKey).Scan(&resourceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: class %q, family %q, type %q, or unit %q", domain.ErrResourceReference, resource.ClassCode, resource.FamilyCode, resource.TypeCode, resource.NaturalUnit)
		}
		return mapRepositoryError(fmt.Errorf("insert resource: %w", err))
	}
	for _, value := range resource.Attributes {
		payload, err := encodeValue(value)
		if err != nil {
			return fmt.Errorf("encode attribute %q: %w", value.AttributeCode, err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO public.resource_attribute_values
			(resource_id, family_id, resource_attribute_id, attribute_definition_id, option_set, value_state,
			 option_code, integer_value, decimal_value, quantity_value, quantity_unit_id, boolean_value, text_value)
			SELECT $1, f.id, ra.id, d.id, ra.option_set, $4, $5, $6, $7, $8, qu.id, $9, $10
			FROM public.resource_families f
			JOIN public.resource_attributes ra ON ra.family_id = f.id
			JOIN public.attribute_definitions d ON d.id = ra.definition_id AND d.code = $3
			LEFT JOIN public.unit_definitions qu ON qu.code = $11
			WHERE f.code = $2`, resourceID, resource.FamilyCode, value.AttributeCode, payload.state, nullableString(payload.option), payload.integer, payload.decimal, nullableQuantity(payload), payload.boolean, nullableString(payload.text), payload.quantityUnit)
		if err != nil {
			return mapRepositoryError(fmt.Errorf("insert resource attribute %q: %w", value.AttributeCode, err))
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit resource create: %w", err)
	}
	return nil
}

func (r *resourceRepository) Get(ctx context.Context, classCode, identityKey string) (domain.Resource, error) {
	if r.pool == nil {
		return domain.Resource{}, errors.New("resource repository: nil pool")
	}
	var resourceID int64
	var naturalUnit, storedClass, storedFamily, typeCode string
	err := r.pool.QueryRow(ctx, `
		SELECT r.id, cl.code, f.code, t.code, u.code FROM public.recursos r
		JOIN public.resource_classes cl ON cl.id = r.class_id
		JOIN public.resource_families f ON f.id = r.family_id
		JOIN public.resource_types t ON t.id = r.type_id
		JOIN public.unit_definitions u ON u.id = r.natural_unit_id
		WHERE cl.code = $1 AND r.identity_key = $2`, classCode, identityKey).Scan(&resourceID, &storedClass, &storedFamily, &typeCode, &naturalUnit)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Resource{}, fmt.Errorf("%w: resource %s/%s", domain.ErrResourceNotFound, classCode, identityKey)
	}
	if err != nil {
		return domain.Resource{}, fmt.Errorf("load resource: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT d.code, d.value_type, v.value_state, v.option_code, v.integer_value, v.decimal_value,
		       v.quantity_value, qu.code, v.boolean_value, v.text_value
		FROM public.resource_attribute_values v
		JOIN public.resource_attributes ra ON ra.id = v.resource_attribute_id
		JOIN public.attribute_definitions d ON d.id = ra.definition_id
		LEFT JOIN public.unit_definitions qu ON qu.id = v.quantity_unit_id
		WHERE v.resource_id = $1 ORDER BY d.code`, resourceID)
	if err != nil {
		return domain.Resource{}, fmt.Errorf("load resource attributes: %w", err)
	}
	defer rows.Close()
	resource := domain.Resource{ID: resourceID, ClassCode: storedClass, FamilyCode: storedFamily, TypeCode: typeCode, NaturalUnit: naturalUnit, IdentityKey: identityKey}
	for rows.Next() {
		var code, valueType, state string
		var option, unit, text *string
		var integer *int64
		var dec, quantity *string
		var boolean *bool
		if err := rows.Scan(&code, &valueType, &state, &option, &integer, &dec, &quantity, &unit, &boolean, &text); err != nil {
			return domain.Resource{}, fmt.Errorf("scan resource attribute: %w", err)
		}
		value, err := decodeValue(code, domain.AttributeValueType(valueType), state, option, integer, dec, quantity, unit, boolean, text)
		if err != nil {
			return domain.Resource{}, err
		}
		resource.Attributes = append(resource.Attributes, value)
	}
	if err := rows.Err(); err != nil {
		return domain.Resource{}, fmt.Errorf("read resource attributes: %w", err)
	}
	return resource, nil
}

func (r *resourceRepository) Update(ctx context.Context, resource domain.Resource) error {
	if r.pool == nil {
		return errors.New("resource repository: nil pool")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin resource update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var classID, familyID, typeID, unitID int64
	err = tx.QueryRow(ctx, `
		SELECT cl.id, f.id, t.id, u.id
		FROM public.resource_classes cl
		JOIN public.resource_families f ON f.class_id = cl.id AND f.code = $2
		JOIN public.resource_types t ON t.family_id = f.id AND t.code = $3
		JOIN public.resource_unit_policies p ON p.family_id = f.id AND p.allowed
		JOIN public.unit_definitions u ON u.id = p.unit_id AND u.code = $4
		WHERE cl.code = $1 AND cl.active AND f.active AND t.active AND u.active`, resource.ClassCode, resource.FamilyCode, resource.TypeCode, resource.NaturalUnit).Scan(&classID, &familyID, &typeID, &unitID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: class %q, family %q, type %q, or unit %q", domain.ErrResourceReference, resource.ClassCode, resource.FamilyCode, resource.TypeCode, resource.NaturalUnit)
		}
		return fmt.Errorf("resolve resource references: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE public.recursos
		SET class_id = $1, family_id = $2, type_id = $3, natural_unit_id = $4, display_name = $5, identity_key = $6, updated_at = NOW()
		WHERE id = $7`, classID, familyID, typeID, unitID, resource.FamilyCode, resource.IdentityKey, resource.ID)
	if err != nil {
		return mapRepositoryError(fmt.Errorf("update resource: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: resource id %d", domain.ErrResourceNotFound, resource.ID)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM public.resource_attribute_values WHERE resource_id = $1`, resource.ID); err != nil {
		return fmt.Errorf("clear resource attributes: %w", err)
	}
	for _, value := range resource.Attributes {
		payload, err := encodeValue(value)
		if err != nil {
			return fmt.Errorf("encode attribute %q: %w", value.AttributeCode, err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO public.resource_attribute_values
			(resource_id, family_id, resource_attribute_id, attribute_definition_id, option_set, value_state,
			 option_code, integer_value, decimal_value, quantity_value, quantity_unit_id, boolean_value, text_value)
			SELECT $1, f.id, ra.id, d.id, ra.option_set, $4, $5, $6, $7, $8, qu.id, $9, $10
			FROM public.resource_families f
			JOIN public.resource_attributes ra ON ra.family_id = f.id
			JOIN public.attribute_definitions d ON d.id = ra.definition_id AND d.code = $3
			LEFT JOIN public.unit_definitions qu ON qu.code = $11
			WHERE f.code = $2`, resource.ID, resource.FamilyCode, value.AttributeCode, payload.state, nullableString(payload.option), payload.integer, payload.decimal, nullableQuantity(payload), payload.boolean, nullableString(payload.text), payload.quantityUnit)
		if err != nil {
			return mapRepositoryError(fmt.Errorf("insert resource attribute %q: %w", value.AttributeCode, err))
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit resource update: %w", err)
	}
	return nil
}

func (r *resourceRepository) SetActive(ctx context.Context, id int64, active bool) error {
	if r.pool == nil {
		return errors.New("resource repository: nil pool")
	}
	tag, err := r.pool.Exec(ctx, `UPDATE public.recursos SET active = $2, updated_at = NOW() WHERE id = $1`, id, active)
	if err != nil {
		return mapRepositoryError(fmt.Errorf("set resource active: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: resource id %d", domain.ErrResourceNotFound, id)
	}
	return nil
}
