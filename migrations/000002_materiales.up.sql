CREATE TABLE public.material_categories (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE CHECK (btrim(code) <> ''),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE public.material_families (
    id BIGSERIAL PRIMARY KEY,
    category_id BIGINT NOT NULL REFERENCES public.material_categories(id),
    code TEXT NOT NULL UNIQUE CHECK (btrim(code) <> ''),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    description TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE public.unit_definitions (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE CHECK (btrim(code) <> ''),
    symbol TEXT NOT NULL UNIQUE CHECK (btrim(symbol) <> ''),
    dimension TEXT NOT NULL CHECK (btrim(dimension) <> ''),
    active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE public.family_unit_policies (
    family_id BIGINT NOT NULL REFERENCES public.material_families(id),
    unit_id BIGINT NOT NULL REFERENCES public.unit_definitions(id),
    allowed BOOLEAN NOT NULL DEFAULT TRUE,
    suggested BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (family_id, unit_id),
    CHECK (NOT suggested OR allowed)
);

CREATE TABLE public.attribute_definitions (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE CHECK (btrim(code) <> ''),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    value_type TEXT NOT NULL CHECK (value_type IN
        ('CONTROLLED_OPTION', 'INTEGER', 'DECIMAL', 'QUANTITY', 'BOOLEAN', 'CONTROLLED_TEXT')),
    dimension TEXT,
    default_identity_participates BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE public.family_attributes (
    id BIGSERIAL PRIMARY KEY,
    family_id BIGINT NOT NULL REFERENCES public.material_families(id),
    definition_id BIGINT NOT NULL REFERENCES public.attribute_definitions(id),
    mode TEXT NOT NULL CHECK (mode IN ('REQUIRED', 'OPTIONAL', 'CONDITIONAL', 'FORBIDDEN')),
    identity_participates BOOLEAN NOT NULL DEFAULT FALSE,
    condition_definition_id BIGINT REFERENCES public.attribute_definitions(id),
    condition_operator TEXT CHECK (condition_operator IN ('EQUALS', 'NOT_EQUALS')),
    condition_value TEXT,
    not_applicable_when_condition BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (family_id, definition_id),
    CHECK ((mode = 'CONDITIONAL') = (condition_definition_id IS NOT NULL)),
    CHECK (condition_definition_id IS NULL OR
           (condition_operator IS NOT NULL AND condition_value IS NOT NULL))
);

ALTER TABLE public.family_attributes
    ADD CONSTRAINT family_attributes_id_family_definition_key
    UNIQUE (id, family_id, definition_id);

CREATE TABLE public.attribute_options (
    attribute_definition_id BIGINT NOT NULL REFERENCES public.attribute_definitions(id),
    code TEXT NOT NULL CHECK (btrim(code) <> ''),
    label TEXT NOT NULL CHECK (btrim(label) <> ''),
    numeric_value NUMERIC,
    unit_id BIGINT REFERENCES public.unit_definitions(id),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (attribute_definition_id, code),
    CHECK ((numeric_value IS NULL) = (unit_id IS NULL))
);

CREATE TABLE public.attribute_option_relations (
    id BIGSERIAL PRIMARY KEY,
    from_attribute_definition_id BIGINT NOT NULL REFERENCES public.attribute_definitions(id),
    from_option_code TEXT NOT NULL CHECK (btrim(from_option_code) <> ''),
    to_attribute_definition_id BIGINT NOT NULL REFERENCES public.attribute_definitions(id),
    to_option_code TEXT NOT NULL CHECK (btrim(to_option_code) <> ''),
    UNIQUE (from_attribute_definition_id, from_option_code, to_attribute_definition_id, to_option_code),
    FOREIGN KEY (from_attribute_definition_id, from_option_code)
        REFERENCES public.attribute_options(attribute_definition_id, code),
    FOREIGN KEY (to_attribute_definition_id, to_option_code)
        REFERENCES public.attribute_options(attribute_definition_id, code),
    CHECK (from_attribute_definition_id <> to_attribute_definition_id)
);

CREATE TABLE public.materiales (
    id BIGSERIAL PRIMARY KEY,
    family_id BIGINT NOT NULL REFERENCES public.material_families(id),
    natural_unit_id BIGINT NOT NULL REFERENCES public.unit_definitions(id),
    display_name TEXT NOT NULL CHECK (btrim(display_name) <> ''),
    identity_key TEXT NOT NULL CHECK (btrim(identity_key) <> ''),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (family_id, identity_key)
);

ALTER TABLE public.materiales
    ADD CONSTRAINT materiales_id_family_key UNIQUE (id, family_id);

CREATE TABLE public.material_attribute_values (
    id BIGSERIAL PRIMARY KEY,
    material_id BIGINT NOT NULL,
    family_id BIGINT NOT NULL,
    family_attribute_id BIGINT NOT NULL,
    attribute_definition_id BIGINT NOT NULL,
    value_state TEXT NOT NULL DEFAULT 'SET' CHECK (value_state IN ('SET', 'NOT_APPLICABLE')),
    option_code TEXT,
    integer_value BIGINT,
    decimal_value NUMERIC,
    quantity_value NUMERIC,
    quantity_unit_id BIGINT REFERENCES public.unit_definitions(id),
    boolean_value BOOLEAN,
    text_value TEXT,
    UNIQUE (material_id, family_attribute_id),
    FOREIGN KEY (material_id, family_id)
        REFERENCES public.materiales(id, family_id) ON DELETE CASCADE,
    FOREIGN KEY (family_attribute_id, family_id, attribute_definition_id)
        REFERENCES public.family_attributes(id, family_id, definition_id),
    FOREIGN KEY (attribute_definition_id, option_code)
        REFERENCES public.attribute_options(attribute_definition_id, code),
    CHECK (value_state = 'NOT_APPLICABLE' OR
           num_nonnulls(option_code, integer_value, decimal_value, quantity_value,
                        boolean_value, text_value) = 1),
    CHECK (value_state = 'SET' OR
           num_nonnulls(option_code, integer_value, decimal_value, quantity_value,
                        boolean_value, text_value) = 0),
    CHECK (value_state = 'NOT_APPLICABLE' OR quantity_value IS NULL OR quantity_unit_id IS NOT NULL),
    CHECK (value_state = 'NOT_APPLICABLE' OR text_value IS NULL OR btrim(text_value) <> '')
);

CREATE FUNCTION public.validate_material_attribute_value()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    definition_value_type TEXT;
BEGIN
    SELECT d.value_type
      INTO definition_value_type
      FROM public.family_attributes fa
      JOIN public.attribute_definitions d ON d.id = fa.definition_id
     WHERE fa.id = NEW.family_attribute_id
       AND fa.family_id = NEW.family_id
       AND fa.definition_id = NEW.attribute_definition_id;

    IF NEW.value_state = 'NOT_APPLICABLE' THEN
        RETURN NEW;
    END IF;

    IF definition_value_type = 'CONTROLLED_OPTION' THEN
        IF NEW.option_code IS NULL THEN
            RAISE EXCEPTION 'SET CONTROLLED_OPTION values require an official option'
                USING ERRCODE = '23514';
        END IF;
    ELSIF NEW.option_code IS NOT NULL THEN
        RAISE EXCEPTION 'only CONTROLLED_OPTION values may reference an option'
            USING ERRCODE = '23514';
    END IF;

    IF definition_value_type = 'CONTROLLED_OPTION'
       AND num_nonnulls(NEW.option_code, NEW.integer_value, NEW.decimal_value,
                        NEW.quantity_value, NEW.boolean_value, NEW.text_value) <> 1 THEN
        RAISE EXCEPTION 'SET CONTROLLED_OPTION values require exactly one option payload'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER material_attribute_values_validate_type
BEFORE INSERT OR UPDATE ON public.material_attribute_values
FOR EACH ROW EXECUTE FUNCTION public.validate_material_attribute_value();

ALTER TABLE public.material_categories OWNER TO garfex_admin;
ALTER TABLE public.material_families OWNER TO garfex_admin;
ALTER TABLE public.unit_definitions OWNER TO garfex_admin;
ALTER TABLE public.family_unit_policies OWNER TO garfex_admin;
ALTER TABLE public.attribute_definitions OWNER TO garfex_admin;
ALTER TABLE public.family_attributes OWNER TO garfex_admin;
ALTER TABLE public.attribute_options OWNER TO garfex_admin;
ALTER TABLE public.attribute_option_relations OWNER TO garfex_admin;
ALTER TABLE public.materiales OWNER TO garfex_admin;
ALTER TABLE public.material_attribute_values OWNER TO garfex_admin;
ALTER FUNCTION public.validate_material_attribute_value() OWNER TO garfex_admin;

INSERT INTO public.material_categories (code, name) VALUES ('ELECTRICAL', 'Electrical materials');
INSERT INTO public.material_families (category_id, code, name, description)
SELECT id, 'CONDUCTORES', 'Conductors', 'Electrical conductors' FROM public.material_categories WHERE code = 'ELECTRICAL';
INSERT INTO public.material_families (category_id, code, name, description)
SELECT id, 'TUBERIAS', 'Tuberías', 'Technical tubes/conduit used as APU materials' FROM public.material_categories WHERE code = 'ELECTRICAL';
INSERT INTO public.unit_definitions (code, symbol, dimension) VALUES
    ('M', 'M', 'LENGTH'), ('V', 'V', 'VOLTAGE'), ('KV', 'KV', 'VOLTAGE'), ('PZA', 'PZA', 'PIECE');
INSERT INTO public.family_unit_policies (family_id, unit_id, allowed, suggested)
SELECT f.id, u.id, TRUE, TRUE FROM public.material_families f, public.unit_definitions u
WHERE f.code = 'CONDUCTORES' AND u.code = 'M';
INSERT INTO public.family_unit_policies (family_id, unit_id, allowed, suggested)
SELECT f.id, u.id, TRUE, TRUE FROM public.material_families f, public.unit_definitions u
WHERE f.code = 'TUBERIAS' AND u.code = 'PZA';

INSERT INTO public.attribute_definitions (code, name, value_type, dimension, default_identity_participates) VALUES
    ('conductor_material', 'Conductor material', 'CONTROLLED_OPTION', NULL, TRUE),
    ('gauge', 'Gauge', 'CONTROLLED_OPTION', NULL, TRUE),
    ('insulation', 'Insulation', 'CONTROLLED_OPTION', NULL, TRUE),
    ('color', 'Color', 'CONTROLLED_OPTION', NULL, TRUE),
    ('voltage', 'Voltage', 'QUANTITY', 'VOLTAGE', TRUE),
    ('tipo', 'Tipo', 'CONTROLLED_OPTION', NULL, TRUE),
    ('diameter_inch', 'Diameter inch', 'CONTROLLED_OPTION', NULL, TRUE),
    ('diameter_mm', 'Diameter mm', 'CONTROLLED_OPTION', NULL, FALSE);
INSERT INTO public.family_attributes (family_id, definition_id, mode, identity_participates)
SELECT f.id, d.id, 'REQUIRED', TRUE FROM public.material_families f
JOIN public.attribute_definitions d ON d.code IN ('conductor_material', 'gauge', 'insulation')
WHERE f.code = 'CONDUCTORES';
INSERT INTO public.family_attributes (family_id, definition_id, mode, identity_participates, condition_definition_id, condition_operator, condition_value, not_applicable_when_condition)
SELECT f.id, d.id, 'CONDITIONAL', TRUE, i.id, 'EQUALS', 'DESNUDO', TRUE
FROM public.material_families f
JOIN public.attribute_definitions d ON d.code IN ('color', 'voltage')
JOIN public.attribute_definitions i ON i.code = 'insulation'
WHERE f.code = 'CONDUCTORES';
INSERT INTO public.family_attributes (family_id, definition_id, mode, identity_participates)
SELECT f.id, d.id, 'REQUIRED', TRUE FROM public.material_families f
JOIN public.attribute_definitions d ON d.code IN ('tipo', 'diameter_inch')
WHERE f.code = 'TUBERIAS';
INSERT INTO public.family_attributes (family_id, definition_id, mode, identity_participates)
SELECT f.id, d.id, 'REQUIRED', FALSE FROM public.material_families f
JOIN public.attribute_definitions d ON d.code = 'diameter_mm'
WHERE f.code = 'TUBERIAS';

INSERT INTO public.attribute_options (attribute_definition_id, code, label)
SELECT d.id, x.code, x.code FROM public.attribute_definitions d
CROSS JOIN (VALUES
    ('COBRE'), ('ALUMINIO')) AS x(code) WHERE d.code = 'conductor_material';
INSERT INTO public.attribute_options (attribute_definition_id, code, label)
SELECT d.id, x.code, x.code FROM public.attribute_definitions d
CROSS JOIN (VALUES ('14 AWG'), ('12 AWG'), ('10 AWG'), ('8 AWG'), ('6 AWG'), ('4 AWG'),
    ('2 AWG'), ('1 AWG'), ('1/0 AWG'), ('2/0 AWG'), ('3/0 AWG'), ('4/0 AWG')) AS x(code)
WHERE d.code = 'gauge';
INSERT INTO public.attribute_options (attribute_definition_id, code, label)
SELECT d.id, x.code, x.code FROM public.attribute_definitions d
CROSS JOIN (VALUES ('DESNUDO'), ('THW'), ('THW-LS'), ('THHN'), ('THHN/THWN-2'), ('XHHW-2'), ('RHH/RHW-2')) AS x(code)
WHERE d.code = 'insulation';
INSERT INTO public.attribute_options (attribute_definition_id, code, label)
SELECT d.id, x.code, x.code FROM public.attribute_definitions d
CROSS JOIN (VALUES ('NEGRO'), ('BLANCO'), ('ROJO'), ('AZUL'), ('VERDE')) AS x(code)
WHERE d.code = 'color';
INSERT INTO public.attribute_options (attribute_definition_id, code, label)
SELECT d.id, x.code, x.code FROM public.attribute_definitions d
CROSS JOIN (VALUES ('CONDUIT PARED DELGADA'), ('CONDUIT PARED GRUESA'), ('PVC CONDUIT')) AS x(code)
WHERE d.code = 'tipo';
INSERT INTO public.attribute_options (attribute_definition_id, code, label)
SELECT d.id, x.code, x.code FROM public.attribute_definitions d
CROSS JOIN (VALUES ('1/2"'), ('3/4"'), ('1"'), ('1 1/4"'), ('1 1/2"'), ('2"'), ('2 1/2"'), ('3"'), ('4"')) AS x(code)
WHERE d.code = 'diameter_inch';
INSERT INTO public.attribute_options (attribute_definition_id, code, label)
SELECT d.id, x.code, x.code FROM public.attribute_definitions d
CROSS JOIN (VALUES ('13 mm'), ('19 mm'), ('25 mm'), ('32 mm'), ('38 mm'), ('50 mm'), ('60 mm'), ('75 mm'), ('100 mm')) AS x(code)
WHERE d.code = 'diameter_mm';

INSERT INTO public.attribute_option_relations (from_attribute_definition_id, from_option_code, to_attribute_definition_id, to_option_code)
SELECT i.id, x.inch, m.id, x.mm
FROM public.attribute_definitions i, public.attribute_definitions m
CROSS JOIN (VALUES
    ('1/2"', '13 mm'),
    ('3/4"', '19 mm'),
    ('1"', '25 mm'),
    ('1 1/4"', '32 mm'),
    ('1 1/2"', '38 mm'),
    ('2"', '50 mm'),
    ('2 1/2"', '60 mm'),
    ('3"', '75 mm'),
    ('4"', '100 mm')
) AS x(inch, mm)
WHERE i.code = 'diameter_inch' AND m.code = 'diameter_mm';

GRANT USAGE ON SCHEMA public TO garfex_app;
REVOKE ALL ON
    public.material_categories, public.material_families, public.unit_definitions,
    public.family_unit_policies, public.attribute_definitions, public.family_attributes,
    public.attribute_options, public.attribute_option_relations, public.materiales, public.material_attribute_values FROM garfex_app;
REVOKE ALL ON SEQUENCE
    public.material_categories_id_seq, public.material_families_id_seq, public.unit_definitions_id_seq,
    public.attribute_definitions_id_seq, public.family_attributes_id_seq, public.attribute_option_relations_id_seq,
    public.materiales_id_seq, public.material_attribute_values_id_seq FROM garfex_app;
GRANT SELECT ON
    public.material_categories, public.material_families, public.unit_definitions,
    public.family_unit_policies, public.attribute_definitions, public.family_attributes,
    public.attribute_options, public.attribute_option_relations TO garfex_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON
    public.materiales, public.material_attribute_values TO garfex_app;
GRANT USAGE, SELECT ON SEQUENCE
    public.materiales_id_seq, public.material_attribute_values_id_seq TO garfex_app;
