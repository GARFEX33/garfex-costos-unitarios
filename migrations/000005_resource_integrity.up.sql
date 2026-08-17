BEGIN;

CREATE TABLE public.resource_integrity_identity_map (
    resource_id BIGINT PRIMARY KEY REFERENCES public.recursos(id) ON DELETE CASCADE,
    class_id BIGINT NOT NULL REFERENCES public.resource_classes(id),
    legacy_identity_key TEXT NOT NULL,
    v1_identity_key TEXT NOT NULL,
    mapped_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (class_id, legacy_identity_key),
    UNIQUE (class_id, v1_identity_key)
);

CREATE FUNCTION public.resource_identity_component(value TEXT)
RETURNS TEXT LANGUAGE SQL IMMUTABLE STRICT AS $$
    SELECT octet_length(convert_to(value, 'UTF8'))::TEXT || ':' || value
$$;

CREATE TEMP TABLE resource_integrity_identity_audit ON COMMIT DROP AS
WITH source AS (
    SELECT r.id, r.class_id, r.identity_key,
           upper(regexp_replace(btrim(cl.code), '\s+', ' ', 'g')) AS class_code,
           upper(regexp_replace(btrim(f.code), '\s+', ' ', 'g')) AS family_code,
           upper(regexp_replace(btrim(t.code), '\s+', ' ', 'g')) AS type_code,
           lower(regexp_replace(btrim(d.code), '\s+', ' ', 'g')) AS attribute_code,
           d.value_type,
           CASE d.value_type
               WHEN 'CONTROLLED_OPTION' THEN v.option_code
               WHEN 'INTEGER' THEN v.integer_value::TEXT
               WHEN 'DECIMAL' THEN v.decimal_value::TEXT
               WHEN 'QUANTITY' THEN v.quantity_value::TEXT || ' ' || upper(qu.code)
               WHEN 'BOOLEAN' THEN v.boolean_value::TEXT
               ELSE upper(regexp_replace(btrim(v.text_value), '\s+', ' ', 'g'))
           END AS canonical_value,
           ra.identity_participates, v.value_state
      FROM public.recursos r
      JOIN public.resource_classes cl ON cl.id = r.class_id
      JOIN public.resource_families f ON f.id = r.family_id
      JOIN public.resource_types t ON t.id = r.type_id
      LEFT JOIN public.resource_attribute_values v ON v.resource_id = r.id
      LEFT JOIN public.resource_attributes ra ON ra.id = v.resource_attribute_id
      LEFT JOIN public.attribute_definitions d ON d.id = ra.definition_id
      LEFT JOIN public.unit_definitions qu ON qu.id = v.quantity_unit_id
), assembled AS (
    SELECT id, class_id, identity_key, class_code, family_code, type_code,
           string_agg(attribute_code || '=' || canonical_value, '|' ORDER BY attribute_code)
             FILTER (WHERE identity_participates AND value_state = 'SET') AS legacy_parts,
           string_agg(
               public.resource_identity_component(attribute_code) ||
               public.resource_identity_component(value_type) ||
               public.resource_identity_component(canonical_value), '' ORDER BY attribute_code
           ) FILTER (WHERE identity_participates AND value_state = 'SET') AS v1_parts
      FROM source
     GROUP BY id, class_id, identity_key, class_code, family_code, type_code
)
SELECT id, class_id, identity_key AS legacy_identity_key,
       'v1|' || public.resource_identity_component(class_code) ||
       public.resource_identity_component(family_code) ||
       public.resource_identity_component(type_code) || COALESCE(v1_parts, '') AS v1_identity_key,
       class_code || '|' || family_code || '|' || type_code || '|' || COALESCE(legacy_parts, '') AS recomputed_legacy_key
  FROM assembled;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM resource_integrity_identity_audit WHERE legacy_identity_key LIKE 'v1|%') THEN
        RAISE EXCEPTION 'resource integrity migration found mixed identity encodings';
    END IF;
    IF EXISTS (SELECT 1 FROM resource_integrity_identity_audit GROUP BY class_id, v1_identity_key HAVING count(*) > 1) THEN
        RAISE EXCEPTION 'resource integrity migration found canonical identity collisions';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM public.resource_attributes broad
          JOIN public.resource_attributes specific
            ON specific.family_id = broad.family_id
           AND specific.definition_id = broad.definition_id
           AND broad.type_id IS NULL
           AND specific.type_id IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'resource integrity migration found overlapping attribute applicability';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM public.resource_attributes ra
          JOIN public.resource_families f ON f.id = ra.family_id
         WHERE ra.class_id <> f.class_id
            OR (ra.type_id IS NOT NULL AND NOT EXISTS (
                SELECT 1 FROM public.resource_types t
                 WHERE t.id = ra.type_id AND t.family_id = ra.family_id AND t.class_id = ra.class_id))
    ) THEN
        RAISE EXCEPTION 'resource integrity migration found invalid attribute scope';
    END IF;
    IF EXISTS (
        SELECT 1
          FROM public.resource_attribute_values v
          JOIN public.recursos r ON r.id = v.resource_id
          JOIN public.resource_attributes ra ON ra.id = v.resource_attribute_id
         WHERE ra.type_id IS NOT NULL AND ra.type_id <> r.type_id
    ) THEN
        RAISE EXCEPTION 'resource integrity migration found type-inapplicable attribute values';
    END IF;
END $$;

INSERT INTO public.resource_integrity_identity_map
    (resource_id, class_id, legacy_identity_key, v1_identity_key)
SELECT id, class_id, legacy_identity_key, v1_identity_key
  FROM resource_integrity_identity_audit;

ALTER TABLE public.resource_integrity_identity_map OWNER TO garfex_admin;
ALTER FUNCTION public.resource_identity_component(TEXT) OWNER TO garfex_admin;

COMMIT;
