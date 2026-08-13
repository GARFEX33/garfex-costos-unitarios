CREATE TABLE public.product_type_presentation_fields (
    id BIGSERIAL PRIMARY KEY,
    product_type_id BIGINT NOT NULL REFERENCES public.product_types(id),
    family_attribute_id BIGINT NOT NULL REFERENCES public.family_attributes(id),
    position INTEGER NOT NULL CHECK (position > 0),
    UNIQUE (product_type_id, family_attribute_id),
    UNIQUE (product_type_id, position)
);

CREATE FUNCTION public.validate_presentation_field_applicability()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    fa_product_type_id BIGINT;
    fa_family_id BIGINT;
    pt_family_id BIGINT;
BEGIN
    SELECT product_type_id, family_id INTO fa_product_type_id, fa_family_id
      FROM public.family_attributes WHERE id = NEW.family_attribute_id;
    SELECT family_id INTO pt_family_id
      FROM public.product_types WHERE id = NEW.product_type_id;

    IF fa_family_id IS DISTINCT FROM pt_family_id THEN
        RAISE EXCEPTION 'presentation field attribute does not belong to the same family as its ProductType'
            USING ERRCODE = '23514';
    END IF;

    IF fa_product_type_id IS NOT NULL AND fa_product_type_id IS DISTINCT FROM NEW.product_type_id THEN
        RAISE EXCEPTION 'presentation field attribute is scoped to a different ProductType'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER product_type_presentation_fields_validate_applicability
BEFORE INSERT OR UPDATE ON public.product_type_presentation_fields
FOR EACH ROW EXECUTE FUNCTION public.validate_presentation_field_applicability();

ALTER TABLE public.product_type_presentation_fields OWNER TO garfex_admin;
ALTER FUNCTION public.validate_presentation_field_applicability() OWNER TO garfex_admin;

INSERT INTO public.product_type_presentation_fields (product_type_id, family_attribute_id, position)
SELECT pt.id, fa.id, x.position
FROM public.product_types pt
JOIN public.material_families f ON f.id = pt.family_id
JOIN public.family_attributes fa ON fa.family_id = f.id
JOIN public.attribute_definitions d ON d.id = fa.definition_id
CROSS JOIN (VALUES
    ('CABLE', 'insulation', 1),
    ('CABLE', 'gauge', 2),
    ('CABLE', 'color', 3),
    ('TUBERIA', 'tipo', 1),
    ('TUBERIA', 'diameter_inch', 2)
) AS x(product_type_code, attribute_code, position)
WHERE pt.code = x.product_type_code AND d.code = x.attribute_code;

GRANT SELECT ON public.product_type_presentation_fields TO garfex_app;
