-- 000008_resource_revisions (down): drops only the revision columns and
-- their CHECK constraints added by the up migration. `identity_key` and
-- every other identity/sequence value is never touched. Safe to leave the
-- database at migration 7 whenever no revision-aware code is deployed
-- (design "Resource revisions and identity").

BEGIN;

ALTER TABLE public.resource_type_presentation_fields DROP COLUMN IF EXISTS revision;
ALTER TABLE public.resource_attributes DROP COLUMN IF EXISTS revision;
ALTER TABLE public.resource_unit_policies DROP COLUMN IF EXISTS revision;
ALTER TABLE public.unit_definitions DROP COLUMN IF EXISTS revision;
ALTER TABLE public.attribute_option_relations DROP COLUMN IF EXISTS revision;
ALTER TABLE public.attribute_options DROP COLUMN IF EXISTS revision;
ALTER TABLE public.resource_option_sets DROP COLUMN IF EXISTS revision;
ALTER TABLE public.attribute_definitions DROP COLUMN IF EXISTS revision;
ALTER TABLE public.resource_types DROP COLUMN IF EXISTS revision;
ALTER TABLE public.resource_families DROP COLUMN IF EXISTS revision;
ALTER TABLE public.resource_classes DROP COLUMN IF EXISTS revision;
ALTER TABLE public.recursos DROP COLUMN IF EXISTS revision;

COMMIT;
