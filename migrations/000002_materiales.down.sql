DROP TABLE IF EXISTS public.attribute_option_relations;

REVOKE ALL ON
    public.material_categories, public.material_families, public.unit_definitions,
    public.family_unit_policies, public.attribute_definitions, public.family_attributes,
    public.attribute_options, public.materiales, public.material_attribute_values FROM garfex_app;
REVOKE ALL ON SEQUENCE
    public.material_categories_id_seq, public.material_families_id_seq, public.unit_definitions_id_seq,
    public.attribute_definitions_id_seq, public.family_attributes_id_seq,
    public.materiales_id_seq, public.material_attribute_values_id_seq FROM garfex_app;

DROP TRIGGER IF EXISTS material_attribute_values_validate_type
    ON public.material_attribute_values;
DROP FUNCTION IF EXISTS public.validate_material_attribute_value();
DROP TABLE IF EXISTS public.material_attribute_values;
DROP TABLE IF EXISTS public.materiales;
DROP TABLE IF EXISTS public.attribute_options;
DROP TABLE IF EXISTS public.family_attributes;
DROP TABLE IF EXISTS public.attribute_definitions;
DROP TABLE IF EXISTS public.family_unit_policies;
DROP TABLE IF EXISTS public.unit_definitions;
DROP TABLE IF EXISTS public.material_families;
DROP TABLE IF EXISTS public.material_categories;
