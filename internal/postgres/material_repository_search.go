package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
)

// defaultSearchLimit is applied when SearchCriteria.Limit is not a positive
// value, so that "0 means default" never silently degrades to LIMIT 0
// (which would return zero rows).
const defaultSearchLimit = 50

// clampLimitOffset normalizes the pagination inputs of a Search: a
// non-positive limit falls back to defaultSearchLimit and a negative offset
// clamps to zero.
func clampLimitOffset(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// Search returns active materials matching criteria. It reuses Get per
// matched row to reconstruct the full domain.Material with attributes,
// instead of duplicating attribute-reconstruction SQL. This N+1-style
// pattern is an intentional v1 simplicity choice, bounded by Limit
// (default defaultSearchLimit).
func (r *materialRepository) Search(ctx context.Context, criteria domain.SearchCriteria) ([]domain.Material, error) {
	if r.pool == nil {
		return nil, errors.New("material repository: nil pool")
	}
	limit, offset := clampLimitOffset(criteria.Limit, criteria.Offset)

	var conditions []string
	var args []any
	arg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	conditions = append(conditions, "m.active")
	if criteria.FamilyCode != "" {
		conditions = append(conditions, "f.code = "+arg(criteria.FamilyCode))
	}
	if criteria.Text != "" {
		placeholder := arg("%" + criteria.Text + "%")
		conditions = append(conditions, fmt.Sprintf("(m.identity_key ILIKE %s OR f.code ILIKE %s OR f.name ILIKE %s)", placeholder, placeholder, placeholder))
	}
	for _, filter := range criteria.Filters {
		payload, err := encodeValue(filter)
		if err != nil {
			return nil, fmt.Errorf("encode search filter %q: %w", filter.AttributeCode, err)
		}
		codeArg := arg(filter.AttributeCode)
		var valueCond string
		switch filter.Type {
		case domain.ValueTypeControlledOption:
			valueCond = "v.option_code = " + arg(payload.option)
		case domain.ValueTypeInteger:
			valueCond = "v.integer_value = " + arg(payload.integer)
		case domain.ValueTypeDecimal:
			valueCond = "v.decimal_value = " + arg(payload.decimal)
		case domain.ValueTypeQuantity:
			valueCond = fmt.Sprintf("v.quantity_value = %s AND qu.code = %s", arg(payload.quantity), arg(payload.quantityUnit))
		case domain.ValueTypeBoolean:
			valueCond = "v.boolean_value = " + arg(payload.boolean)
		case domain.ValueTypeControlledText:
			valueCond = "v.text_value = " + arg(payload.text)
		default:
			return nil, fmt.Errorf("unsupported filter value type %q", filter.Type)
		}
		conditions = append(conditions, fmt.Sprintf(
			`EXISTS (SELECT 1 FROM public.material_attribute_values v
			 JOIN public.family_attributes fa ON fa.id = v.family_attribute_id
			 JOIN public.attribute_definitions d ON d.id = fa.definition_id
			 LEFT JOIN public.unit_definitions qu ON qu.id = v.quantity_unit_id
			 WHERE v.material_id = m.id AND d.code = %s AND v.value_state = 'SET' AND %s)`,
			codeArg, valueCond))
	}

	limitArg, offsetArg := arg(limit), arg(offset)
	query := fmt.Sprintf(`
		SELECT f.code, u.code, m.identity_key
		FROM public.materiales m
		JOIN public.material_families f ON f.id = m.family_id
		JOIN public.unit_definitions u ON u.id = m.natural_unit_id
		WHERE %s
		ORDER BY m.identity_key ASC
		LIMIT %s OFFSET %s`, strings.Join(conditions, " AND "), limitArg, offsetArg)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search materials: %w", err)
	}
	defer rows.Close()

	var results []domain.Material
	for rows.Next() {
		var familyCode, naturalUnit, identityKey string
		if err := rows.Scan(&familyCode, &naturalUnit, &identityKey); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		material, err := r.Get(ctx, familyCode, identityKey)
		if err != nil {
			return nil, fmt.Errorf("load matched material %s/%s: %w", familyCode, identityKey, err)
		}
		results = append(results, material)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read search results: %w", err)
	}
	return results, nil
}
