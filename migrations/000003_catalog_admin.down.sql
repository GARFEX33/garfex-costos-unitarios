-- Reverse of 000003_catalog_admin.up.sql, in reverse order.

-- 1f (reverse). Revoke garfex_app write grants added by this migration.
REVOKE SELECT ON public.resource_attribute_rules FROM garfex_app;
REVOKE USAGE, SELECT ON SEQUENCE
    public.resource_classes_id_seq, public.resource_families_id_seq, public.resource_types_id_seq,
    public.unit_definitions_id_seq, public.attribute_definitions_id_seq,
    public.resource_attributes_id_seq, public.attribute_option_relations_id_seq,
    public.resource_attribute_rules_id_seq FROM garfex_app;
REVOKE INSERT, UPDATE, DELETE ON
    public.resource_classes, public.resource_option_sets, public.resource_families,
    public.resource_types, public.unit_definitions, public.resource_unit_policies,
    public.attribute_definitions, public.attribute_options, public.attribute_option_relations,
    public.resource_attributes, public.resource_type_presentation_fields,
    public.resource_attribute_rules FROM garfex_app;

-- 1e (reverse). Restore the inline condition_* columns and their CHECKs,
-- backfill them from resource_attribute_rules (lossless: only 2 rows, both
-- single-rule), then drop the child table.
ALTER TABLE public.resource_attributes
    ADD COLUMN condition_definition_id BIGINT REFERENCES public.attribute_definitions(id),
    ADD COLUMN condition_operator TEXT CHECK (condition_operator IN ('EQUALS','NOT_EQUALS')),
    ADD COLUMN condition_value TEXT,
    ADD COLUMN not_applicable_when_condition BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE public.resource_attributes ra
   SET condition_definition_id = r.when_definition_id,
       condition_operator = 'EQUALS',
       condition_value = r.when_equals,
       not_applicable_when_condition = r.not_applicable
  FROM public.resource_attribute_rules r
 WHERE r.resource_attribute_id = ra.id
   AND r.display_order = 0;

ALTER TABLE public.resource_attributes
    ADD CONSTRAINT resource_attributes_check
        CHECK ((mode = 'CONDITIONAL') = (condition_definition_id IS NOT NULL)),
    ADD CONSTRAINT resource_attributes_check1
        CHECK (condition_definition_id IS NULL OR
               (condition_operator IS NOT NULL AND condition_value IS NOT NULL));

DROP TABLE public.resource_attribute_rules;

-- 1d (reverse). Drop ordering columns and their unique constraints/indexes.
DROP INDEX public.resource_attributes_type_display_order_key;
DROP INDEX public.resource_attributes_family_display_order_key;
ALTER TABLE public.attribute_options DROP CONSTRAINT attribute_options_display_order_key;
ALTER TABLE public.resource_attributes DROP COLUMN display_order;
ALTER TABLE public.attribute_options DROP COLUMN display_order;

-- 1c (reverse). Drop class aliases/keywords.
ALTER TABLE public.resource_classes
    DROP COLUMN aliases,
    DROP COLUMN keywords;

-- 1b (reverse). Drop the shared trigger from every table, then the function,
-- then the audit columns.
DROP TRIGGER resource_classes_set_updated_at ON public.resource_classes;
DROP TRIGGER resource_option_sets_set_updated_at ON public.resource_option_sets;
DROP TRIGGER resource_families_set_updated_at ON public.resource_families;
DROP TRIGGER resource_types_set_updated_at ON public.resource_types;
DROP TRIGGER unit_definitions_set_updated_at ON public.unit_definitions;
DROP TRIGGER resource_unit_policies_set_updated_at ON public.resource_unit_policies;
DROP TRIGGER attribute_definitions_set_updated_at ON public.attribute_definitions;
DROP TRIGGER attribute_options_set_updated_at ON public.attribute_options;
DROP TRIGGER attribute_option_relations_set_updated_at ON public.attribute_option_relations;
DROP TRIGGER resource_attributes_set_updated_at ON public.resource_attributes;
DROP TRIGGER resource_type_presentation_fields_set_updated_at ON public.resource_type_presentation_fields;

DROP FUNCTION public.set_updated_at();

ALTER TABLE public.resource_classes DROP COLUMN created_at, DROP COLUMN updated_at;
ALTER TABLE public.resource_option_sets DROP COLUMN created_at, DROP COLUMN updated_at;
ALTER TABLE public.resource_families DROP COLUMN created_at, DROP COLUMN updated_at;
ALTER TABLE public.resource_types DROP COLUMN created_at, DROP COLUMN updated_at;
ALTER TABLE public.unit_definitions DROP COLUMN created_at, DROP COLUMN updated_at;
ALTER TABLE public.resource_unit_policies DROP COLUMN created_at, DROP COLUMN updated_at;
ALTER TABLE public.attribute_definitions DROP COLUMN created_at, DROP COLUMN updated_at;
ALTER TABLE public.attribute_options DROP COLUMN created_at, DROP COLUMN updated_at;
ALTER TABLE public.attribute_option_relations DROP COLUMN created_at, DROP COLUMN updated_at;
ALTER TABLE public.resource_attributes DROP COLUMN created_at, DROP COLUMN updated_at;
ALTER TABLE public.resource_type_presentation_fields DROP COLUMN created_at, DROP COLUMN updated_at;

-- 1a (reverse). Drop the 6 active columns this migration added.
ALTER TABLE public.attribute_definitions DROP COLUMN active;
ALTER TABLE public.resource_attributes DROP COLUMN active;
ALTER TABLE public.resource_unit_policies DROP COLUMN active;
ALTER TABLE public.resource_type_presentation_fields DROP COLUMN active;
ALTER TABLE public.resource_option_sets DROP COLUMN active;
ALTER TABLE public.attribute_option_relations DROP COLUMN active;
