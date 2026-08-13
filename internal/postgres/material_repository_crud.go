package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type materialRepository struct{ pool *pgxpool.Pool }

// NewMaterialRepository returns a repository for the general Materials Master schema.
func NewMaterialRepository(pool *pgxpool.Pool) domain.MaterialRepository {
	return &materialRepository{pool: pool}
}

func (r *materialRepository) Create(ctx context.Context, material domain.Material) error {
	if r.pool == nil {
		return errors.New("material repository: nil pool")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin material create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var materialID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO public.materiales (family_id, product_type_id, natural_unit_id, display_name, identity_key)
		SELECT f.id, pt.id, u.id, $4, $5
		FROM public.material_families f
		JOIN public.product_types pt ON pt.family_id = f.id AND pt.code = $2
		JOIN public.family_unit_policies p ON p.family_id = f.id AND p.allowed
		JOIN public.unit_definitions u ON u.id = p.unit_id AND u.code = $3
		WHERE f.code = $1
		RETURNING id`, material.FamilyCode, material.ProductTypeCode, material.NaturalUnit, material.FamilyCode, material.IdentityKey).Scan(&materialID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: family %q, product type %q, or unit %q", domain.ErrMaterialReference, material.FamilyCode, material.ProductTypeCode, material.NaturalUnit)
		}
		return mapRepositoryError(fmt.Errorf("insert material: %w", err))
	}
	for _, value := range material.Attributes {
		payload, err := encodeValue(value)
		if err != nil {
			return fmt.Errorf("encode attribute %q: %w", value.AttributeCode, err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO public.material_attribute_values
			(material_id, family_id, family_attribute_id, attribute_definition_id, value_state,
			 option_code, integer_value, decimal_value, quantity_value, quantity_unit_id, boolean_value, text_value)
			SELECT $1, f.id, fa.id, d.id, $4, $5, $6, $7, $8, qu.id, $9, $10
			FROM public.material_families f
			JOIN public.family_attributes fa ON fa.family_id = f.id
			JOIN public.attribute_definitions d ON d.id = fa.definition_id AND d.code = $3
			LEFT JOIN public.unit_definitions qu ON qu.code = $11
			WHERE f.code = $2`, materialID, material.FamilyCode, value.AttributeCode, payload.state, nullableString(payload.option), payload.integer, payload.decimal, nullableQuantity(payload), payload.boolean, nullableString(payload.text), payload.quantityUnit)
		if err != nil {
			return mapRepositoryError(fmt.Errorf("insert material attribute %q: %w", value.AttributeCode, err))
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit material create: %w", err)
	}
	return nil
}

func (r *materialRepository) Get(ctx context.Context, familyCode, identityKey string) (domain.Material, error) {
	if r.pool == nil {
		return domain.Material{}, errors.New("material repository: nil pool")
	}
	var materialID int64
	var naturalUnit, storedFamily, productTypeCode string
	err := r.pool.QueryRow(ctx, `
		SELECT m.id, f.code, pt.code, u.code FROM public.materiales m
		JOIN public.material_families f ON f.id = m.family_id
		JOIN public.product_types pt ON pt.id = m.product_type_id
		JOIN public.unit_definitions u ON u.id = m.natural_unit_id
		WHERE f.code = $1 AND m.identity_key = $2`, familyCode, identityKey).Scan(&materialID, &storedFamily, &productTypeCode, &naturalUnit)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Material{}, fmt.Errorf("%w: material %s/%s", domain.ErrMaterialNotFound, familyCode, identityKey)
	}
	if err != nil {
		return domain.Material{}, fmt.Errorf("load material: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT d.code, d.value_type, v.value_state, v.option_code, v.integer_value, v.decimal_value,
		       v.quantity_value, qu.code, v.boolean_value, v.text_value
		FROM public.material_attribute_values v
		JOIN public.family_attributes fa ON fa.id = v.family_attribute_id
		JOIN public.attribute_definitions d ON d.id = fa.definition_id
		LEFT JOIN public.unit_definitions qu ON qu.id = v.quantity_unit_id
		WHERE v.material_id = $1 ORDER BY d.code`, materialID)
	if err != nil {
		return domain.Material{}, fmt.Errorf("load material attributes: %w", err)
	}
	defer rows.Close()
	material := domain.Material{ID: materialID, FamilyCode: storedFamily, ProductTypeCode: productTypeCode, NaturalUnit: naturalUnit, IdentityKey: identityKey}
	for rows.Next() {
		var code, valueType, state string
		var option, unit, text *string
		var integer *int64
		var dec, quantity *string
		var boolean *bool
		if err := rows.Scan(&code, &valueType, &state, &option, &integer, &dec, &quantity, &unit, &boolean, &text); err != nil {
			return domain.Material{}, fmt.Errorf("scan material attribute: %w", err)
		}
		value, err := decodeValue(code, domain.AttributeValueType(valueType), state, option, integer, dec, quantity, unit, boolean, text)
		if err != nil {
			return domain.Material{}, err
		}
		material.Attributes = append(material.Attributes, value)
	}
	if err := rows.Err(); err != nil {
		return domain.Material{}, fmt.Errorf("read material attributes: %w", err)
	}
	return material, nil
}

func (r *materialRepository) Update(ctx context.Context, material domain.Material) error {
	if r.pool == nil {
		return errors.New("material repository: nil pool")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin material update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var familyID, productTypeID, unitID int64
	err = tx.QueryRow(ctx, `
		SELECT f.id, pt.id, u.id
		FROM public.material_families f
		JOIN public.product_types pt ON pt.family_id = f.id AND pt.code = $2
		JOIN public.family_unit_policies p ON p.family_id = f.id AND p.allowed
		JOIN public.unit_definitions u ON u.id = p.unit_id AND u.code = $3
		WHERE f.code = $1`, material.FamilyCode, material.ProductTypeCode, material.NaturalUnit).Scan(&familyID, &productTypeID, &unitID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: family %q, product type %q, or unit %q", domain.ErrMaterialReference, material.FamilyCode, material.ProductTypeCode, material.NaturalUnit)
		}
		return fmt.Errorf("resolve material references: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE public.materiales
		SET family_id = $1, product_type_id = $2, natural_unit_id = $3, display_name = $4, identity_key = $5, updated_at = NOW()
		WHERE id = $6`, familyID, productTypeID, unitID, material.FamilyCode, material.IdentityKey, material.ID)
	if err != nil {
		return mapRepositoryError(fmt.Errorf("update material: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: material id %d", domain.ErrMaterialNotFound, material.ID)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM public.material_attribute_values WHERE material_id = $1`, material.ID); err != nil {
		return fmt.Errorf("clear material attributes: %w", err)
	}
	for _, value := range material.Attributes {
		payload, err := encodeValue(value)
		if err != nil {
			return fmt.Errorf("encode attribute %q: %w", value.AttributeCode, err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO public.material_attribute_values
			(material_id, family_id, family_attribute_id, attribute_definition_id, value_state,
			 option_code, integer_value, decimal_value, quantity_value, quantity_unit_id, boolean_value, text_value)
			SELECT $1, f.id, fa.id, d.id, $4, $5, $6, $7, $8, qu.id, $9, $10
			FROM public.material_families f
			JOIN public.family_attributes fa ON fa.family_id = f.id
			JOIN public.attribute_definitions d ON d.id = fa.definition_id AND d.code = $3
			LEFT JOIN public.unit_definitions qu ON qu.code = $11
			WHERE f.code = $2`, material.ID, material.FamilyCode, value.AttributeCode, payload.state, nullableString(payload.option), payload.integer, payload.decimal, nullableQuantity(payload), payload.boolean, nullableString(payload.text), payload.quantityUnit)
		if err != nil {
			return mapRepositoryError(fmt.Errorf("insert material attribute %q: %w", value.AttributeCode, err))
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit material update: %w", err)
	}
	return nil
}

func (r *materialRepository) SetActive(ctx context.Context, id int64, active bool) error {
	if r.pool == nil {
		return errors.New("material repository: nil pool")
	}
	tag, err := r.pool.Exec(ctx, `UPDATE public.materiales SET active = $2, updated_at = NOW() WHERE id = $1`, id, active)
	if err != nil {
		return mapRepositoryError(fmt.Errorf("set material active: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: material id %d", domain.ErrMaterialNotFound, id)
	}
	return nil
}
