REVOKE ALL ON
    public.resource_classes, public.resource_option_sets, public.resource_families, public.resource_types,
    public.unit_definitions, public.resource_unit_policies, public.attribute_definitions, public.resource_attributes,
    public.attribute_options, public.attribute_option_relations, public.resource_type_presentation_fields,
    public.recursos, public.resource_attribute_values FROM garfex_app;
REVOKE ALL ON SEQUENCE
    public.resource_classes_id_seq, public.resource_families_id_seq, public.resource_types_id_seq,
    public.unit_definitions_id_seq, public.attribute_definitions_id_seq, public.resource_attributes_id_seq,
    public.recursos_id_seq, public.resource_attribute_values_id_seq FROM garfex_app;

DROP TRIGGER IF EXISTS resource_attribute_values_validate_type
    ON public.resource_attribute_values;
DROP FUNCTION IF EXISTS public.validate_resource_attribute_value();
DROP TABLE IF EXISTS public.resource_attribute_values;
DROP TABLE IF EXISTS public.recursos;
DROP TABLE IF EXISTS public.resource_type_presentation_fields;
DROP TABLE IF EXISTS public.attribute_option_relations;
DROP TABLE IF EXISTS public.resource_attributes;
DROP TABLE IF EXISTS public.attribute_options;
DROP TABLE IF EXISTS public.attribute_definitions;
DROP TABLE IF EXISTS public.resource_unit_policies;
DROP TABLE IF EXISTS public.unit_definitions;
DROP TABLE IF EXISTS public.resource_types;
DROP TABLE IF EXISTS public.resource_families;
DROP TABLE IF EXISTS public.resource_option_sets;
DROP TABLE IF EXISTS public.resource_classes;
