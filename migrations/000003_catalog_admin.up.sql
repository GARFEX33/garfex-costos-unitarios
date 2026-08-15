-- 000003_catalog_admin: makes catalog *structure* administrable at runtime.
-- See docs/architecture/catalog-source-of-truth.md for the D10 supersession.

-- 1a. active columns on the 6 catalog-definition tables that don't already
-- have one (resource_classes/resource_families/resource_types/
-- unit_definitions/attribute_options already carry `active` from 000002).
ALTER TABLE public.attribute_definitions           ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE public.resource_attributes             ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE public.resource_unit_policies          ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE public.resource_type_presentation_fields ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE public.resource_option_sets            ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE public.attribute_option_relations       ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE;

-- 1b. audit columns on all 11 catalog-definition tables, plus a shared
-- updated_at trigger (mirrors the existing validate_resource_attribute_value
-- trigger precedent).
ALTER TABLE public.resource_classes
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE public.resource_option_sets
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE public.resource_families
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE public.resource_types
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE public.unit_definitions
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE public.resource_unit_policies
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE public.attribute_definitions
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE public.attribute_options
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE public.attribute_option_relations
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE public.resource_attributes
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE public.resource_type_presentation_fields
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE FUNCTION public.set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;
ALTER FUNCTION public.set_updated_at() OWNER TO garfex_admin;

CREATE TRIGGER resource_classes_set_updated_at
    BEFORE UPDATE ON public.resource_classes
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER resource_option_sets_set_updated_at
    BEFORE UPDATE ON public.resource_option_sets
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER resource_families_set_updated_at
    BEFORE UPDATE ON public.resource_families
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER resource_types_set_updated_at
    BEFORE UPDATE ON public.resource_types
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER unit_definitions_set_updated_at
    BEFORE UPDATE ON public.unit_definitions
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER resource_unit_policies_set_updated_at
    BEFORE UPDATE ON public.resource_unit_policies
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER attribute_definitions_set_updated_at
    BEFORE UPDATE ON public.attribute_definitions
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER attribute_options_set_updated_at
    BEFORE UPDATE ON public.attribute_options
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER attribute_option_relations_set_updated_at
    BEFORE UPDATE ON public.attribute_option_relations
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER resource_attributes_set_updated_at
    BEFORE UPDATE ON public.resource_attributes
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER resource_type_presentation_fields_set_updated_at
    BEFORE UPDATE ON public.resource_type_presentation_fields
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- 1c. class aliases/keywords (empty today; columns exist so staff-authored
-- values survive going forward).
ALTER TABLE public.resource_classes
    ADD COLUMN aliases  TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN keywords TEXT[] NOT NULL DEFAULT '{}';

-- 1d. ordering columns. Neither attribute_options nor resource_attributes
-- has a serial id usable as an order fallback (composite PK); backfill by
-- physical insertion order (ctid), which is the only order-preserving
-- evidence available for the rows 000002 already inserted, then enforce a
-- per-parent UNIQUE going forward.
ALTER TABLE public.attribute_options   ADD COLUMN display_order INTEGER NOT NULL DEFAULT 0;
ALTER TABLE public.resource_attributes ADD COLUMN display_order INTEGER NOT NULL DEFAULT 0;

WITH ordered AS (
    SELECT ctid,
           ROW_NUMBER() OVER (PARTITION BY option_set, attribute_definition_id ORDER BY ctid) - 1 AS rn
      FROM public.attribute_options
)
UPDATE public.attribute_options ao
   SET display_order = ordered.rn
  FROM ordered
 WHERE ao.ctid = ordered.ctid;

WITH ordered AS (
    SELECT ctid,
           ROW_NUMBER() OVER (PARTITION BY family_id, type_id ORDER BY ctid) - 1 AS rn
      FROM public.resource_attributes
)
UPDATE public.resource_attributes ra
   SET display_order = ordered.rn
  FROM ordered
 WHERE ra.ctid = ordered.ctid;

ALTER TABLE public.attribute_options
    ADD CONSTRAINT attribute_options_display_order_key
    UNIQUE (option_set, attribute_definition_id, display_order);

-- version-independent replacement for a NULLS NOT DISTINCT unique key,
-- mirroring resource_attributes_family_definition_key /
-- resource_attributes_type_definition_key from 000002.
CREATE UNIQUE INDEX resource_attributes_family_display_order_key
    ON public.resource_attributes (family_id, display_order) WHERE type_id IS NULL;
CREATE UNIQUE INDEX resource_attributes_type_display_order_key
    ON public.resource_attributes (family_id, type_id, display_order) WHERE type_id IS NOT NULL;

-- 1e. resource_attribute_rules: replaces the single inline condition_*
-- columns on resource_attributes with a proper child table, matching
-- domain.AttributeRule's []AttributeRule shape (multi-rule support,
-- Mode + IdentityParticipates per rule — neither was representable inline).
CREATE TABLE public.resource_attribute_rules (
    id BIGSERIAL PRIMARY KEY,
    resource_attribute_id BIGINT NOT NULL
        REFERENCES public.resource_attributes(id) ON DELETE CASCADE,
    display_order INTEGER NOT NULL,
    when_definition_id BIGINT NOT NULL REFERENCES public.attribute_definitions(id),
    when_equals TEXT NOT NULL CHECK (btrim(when_equals) <> ''),
    mode TEXT NOT NULL CHECK (mode IN ('REQUIRED','OPTIONAL','CONDITIONAL','FORBIDDEN')),
    identity_participates BOOLEAN NOT NULL DEFAULT FALSE,
    not_applicable BOOLEAN NOT NULL DEFAULT FALSE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (resource_attribute_id, display_order)
);
ALTER TABLE public.resource_attribute_rules OWNER TO garfex_admin;

CREATE TRIGGER resource_attribute_rules_set_updated_at
    BEFORE UPDATE ON public.resource_attribute_rules
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- Backfill: one rule row per existing inline condition (today exactly the 2
-- color/voltage-under-DESNUDO-insulation rows). condition_operator was
-- always 'EQUALS' (NOT_EQUALS was never representable in Go and unused).
INSERT INTO public.resource_attribute_rules
    (resource_attribute_id, display_order, when_definition_id, when_equals, mode, not_applicable)
SELECT ra.id, 0, ra.condition_definition_id, ra.condition_value, 'FORBIDDEN', ra.not_applicable_when_condition
  FROM public.resource_attributes ra
 WHERE ra.condition_definition_id IS NOT NULL;

-- Drop the two CHECKs referencing the inline columns, then the inline
-- columns themselves. resource_attributes.mode stays untouched (still
-- 'CONDITIONAL' where it was) — the CONDITIONAL <=> Rules invariant moves to
-- domain.Validate() (validateAttributeRules()), since it becomes a
-- cross-table invariant a table CHECK cannot express.
ALTER TABLE public.resource_attributes
    DROP CONSTRAINT resource_attributes_check,
    DROP CONSTRAINT resource_attributes_check1,
    DROP CONSTRAINT resource_attributes_condition_operator_check,
    DROP CONSTRAINT resource_attributes_condition_definition_id_fkey;

ALTER TABLE public.resource_attributes
    DROP COLUMN condition_definition_id,
    DROP COLUMN condition_operator,
    DROP COLUMN condition_value,
    DROP COLUMN not_applicable_when_condition;

-- 1f. garfex_app GRANTs — blocking fix. 000002 REVOKEs ALL then GRANTs
-- SELECT only on every catalog-definition table; without this, every admin
-- write fails at runtime with 42501 permission denied, invisible to unit
-- tests (guarded by internal/postgres's GARFEX_TEST_DSN integration test).
GRANT INSERT, UPDATE, DELETE ON
    public.resource_classes, public.resource_option_sets, public.resource_families,
    public.resource_types, public.unit_definitions, public.resource_unit_policies,
    public.attribute_definitions, public.attribute_options, public.attribute_option_relations,
    public.resource_attributes, public.resource_type_presentation_fields,
    public.resource_attribute_rules TO garfex_app;
GRANT USAGE, SELECT ON SEQUENCE
    public.resource_classes_id_seq, public.resource_families_id_seq, public.resource_types_id_seq,
    public.unit_definitions_id_seq, public.attribute_definitions_id_seq,
    public.resource_attributes_id_seq, public.attribute_option_relations_id_seq,
    public.resource_attribute_rules_id_seq TO garfex_app;
GRANT SELECT ON public.resource_attribute_rules TO garfex_app;
