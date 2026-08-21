-- 000008_resource_revisions: adds a dedicated monotonic `revision` column to
-- `recursos` and every one of the 11 catalog-definition parent tables, for
-- future optimistic-concurrency (CAS) writes (design "Resource revisions and
-- identity"). Purely additive: DEFAULT 1 backfills every existing row in the
-- same statement that adds the column, no trigger increments it (every
-- authoritative SQL mutation increments it explicitly in a later slice), and
-- `identity_key`/business data are never touched. `resource_attribute_rules`
-- deliberately receives no independent revision — its lifecycle is owned by
-- its parent `resource_attributes` row. garfex_app's existing table-level
-- GRANTs from 000002/000003 already cover this new column; no additional
-- GRANT is required (see catalog_admin_grants_integration_test.go).

BEGIN;

ALTER TABLE public.recursos
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);

ALTER TABLE public.resource_classes
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);

ALTER TABLE public.resource_families
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);

ALTER TABLE public.resource_types
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);

ALTER TABLE public.attribute_definitions
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);

ALTER TABLE public.resource_option_sets
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);

ALTER TABLE public.attribute_options
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);

ALTER TABLE public.attribute_option_relations
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);

ALTER TABLE public.unit_definitions
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);

ALTER TABLE public.resource_unit_policies
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);

ALTER TABLE public.resource_attributes
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);

ALTER TABLE public.resource_type_presentation_fields
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0);

COMMIT;
