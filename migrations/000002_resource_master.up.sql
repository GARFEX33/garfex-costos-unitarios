CREATE TABLE public.resource_classes (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE CHECK (btrim(code) <> ''),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    plural TEXT NOT NULL CHECK (btrim(plural) <> ''),
    slug TEXT NOT NULL UNIQUE CHECK (btrim(slug) <> ''),
    display_order INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE public.resource_option_sets (
    code TEXT PRIMARY KEY CHECK (btrim(code) <> ''),
    name TEXT NOT NULL CHECK (btrim(name) <> '')
);
INSERT INTO public.resource_option_sets (code, name) VALUES ('DEFAULT', 'Conjunto por defecto');

CREATE TABLE public.resource_families (
    id BIGSERIAL PRIMARY KEY,
    class_id BIGINT NOT NULL REFERENCES public.resource_classes(id),
    code TEXT NOT NULL CHECK (btrim(code) <> ''),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    description TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE (class_id, code)
);
ALTER TABLE public.resource_families
    ADD CONSTRAINT resource_families_id_class_key UNIQUE (id, class_id);

CREATE TABLE public.resource_types (
    id BIGSERIAL PRIMARY KEY,
    class_id BIGINT NOT NULL,
    family_id BIGINT NOT NULL,
    code TEXT NOT NULL CHECK (btrim(code) <> ''),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE (family_id, code),
    FOREIGN KEY (family_id, class_id) REFERENCES public.resource_families(id, class_id)
);
ALTER TABLE public.resource_types
    ADD CONSTRAINT resource_types_id_family_key UNIQUE (id, family_id);

CREATE TABLE public.unit_definitions (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE CHECK (btrim(code) <> ''),
    symbol TEXT NOT NULL UNIQUE CHECK (btrim(symbol) <> ''),
    dimension TEXT NOT NULL CHECK (btrim(dimension) <> ''),
    active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE public.resource_unit_policies (
    family_id BIGINT NOT NULL REFERENCES public.resource_families(id),
    unit_id   BIGINT NOT NULL REFERENCES public.unit_definitions(id),
    allowed   BOOLEAN NOT NULL DEFAULT TRUE,
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

CREATE TABLE public.attribute_options (
    option_set TEXT NOT NULL DEFAULT 'DEFAULT' REFERENCES public.resource_option_sets(code),
    attribute_definition_id BIGINT NOT NULL REFERENCES public.attribute_definitions(id),
    code TEXT NOT NULL CHECK (btrim(code) <> ''),
    label TEXT NOT NULL CHECK (btrim(label) <> ''),
    numeric_value NUMERIC,
    unit_id BIGINT REFERENCES public.unit_definitions(id),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (option_set, attribute_definition_id, code),
    CHECK ((numeric_value IS NULL) = (unit_id IS NULL))
);

CREATE TABLE public.attribute_option_relations (
    id BIGSERIAL PRIMARY KEY,
    option_set TEXT NOT NULL DEFAULT 'DEFAULT' REFERENCES public.resource_option_sets(code),
    from_attribute_definition_id BIGINT NOT NULL,
    from_option_code TEXT NOT NULL CHECK (btrim(from_option_code) <> ''),
    to_attribute_definition_id BIGINT NOT NULL,
    to_option_code TEXT NOT NULL CHECK (btrim(to_option_code) <> ''),
    UNIQUE (option_set, from_attribute_definition_id, from_option_code,
            to_attribute_definition_id, to_option_code),
    FOREIGN KEY (option_set, from_attribute_definition_id, from_option_code)
        REFERENCES public.attribute_options(option_set, attribute_definition_id, code),
    FOREIGN KEY (option_set, to_attribute_definition_id, to_option_code)
        REFERENCES public.attribute_options(option_set, attribute_definition_id, code),
    CHECK (from_attribute_definition_id <> to_attribute_definition_id)
);

CREATE TABLE public.resource_attributes (
    id BIGSERIAL PRIMARY KEY,
    class_id BIGINT NOT NULL,
    family_id BIGINT NOT NULL,
    type_id BIGINT,
    definition_id BIGINT NOT NULL REFERENCES public.attribute_definitions(id),
    option_set TEXT NOT NULL DEFAULT 'DEFAULT' REFERENCES public.resource_option_sets(code),
    mode TEXT NOT NULL CHECK (mode IN ('REQUIRED','OPTIONAL','CONDITIONAL','FORBIDDEN')),
    identity_participates BOOLEAN NOT NULL DEFAULT FALSE,
    condition_definition_id BIGINT REFERENCES public.attribute_definitions(id),
    condition_operator TEXT CHECK (condition_operator IN ('EQUALS','NOT_EQUALS')),
    condition_value TEXT,
    not_applicable_when_condition BOOLEAN NOT NULL DEFAULT FALSE,
    FOREIGN KEY (family_id, class_id) REFERENCES public.resource_families(id, class_id),
    FOREIGN KEY (type_id, family_id)  REFERENCES public.resource_types(id, family_id),
    CHECK ((mode = 'CONDITIONAL') = (condition_definition_id IS NOT NULL)),
    CHECK (condition_definition_id IS NULL OR
           (condition_operator IS NOT NULL AND condition_value IS NOT NULL))
);
-- version-independent replacement for a NULLS NOT DISTINCT unique key
CREATE UNIQUE INDEX resource_attributes_family_definition_key
    ON public.resource_attributes (family_id, definition_id) WHERE type_id IS NULL;
CREATE UNIQUE INDEX resource_attributes_type_definition_key
    ON public.resource_attributes (family_id, type_id, definition_id) WHERE type_id IS NOT NULL;
ALTER TABLE public.resource_attributes
    ADD CONSTRAINT resource_attributes_id_family_definition_key UNIQUE (id, family_id, definition_id);

CREATE TABLE public.resource_type_presentation_fields (
    type_id BIGINT NOT NULL REFERENCES public.resource_types(id),
    attribute_definition_id BIGINT NOT NULL REFERENCES public.attribute_definitions(id),
    position INTEGER NOT NULL,
    PRIMARY KEY (type_id, attribute_definition_id),
    UNIQUE (type_id, position)
);

CREATE TABLE public.recursos (
    id BIGSERIAL PRIMARY KEY,
    class_id BIGINT NOT NULL REFERENCES public.resource_classes(id),
    family_id BIGINT NOT NULL,
    type_id BIGINT NOT NULL,
    natural_unit_id BIGINT NOT NULL REFERENCES public.unit_definitions(id),
    display_name TEXT NOT NULL CHECK (btrim(display_name) <> ''),
    identity_key TEXT NOT NULL CHECK (btrim(identity_key) <> ''),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (class_id, identity_key),
    FOREIGN KEY (family_id, class_id) REFERENCES public.resource_families(id, class_id),
    FOREIGN KEY (type_id, family_id)  REFERENCES public.resource_types(id, family_id)
);
ALTER TABLE public.recursos ADD CONSTRAINT recursos_id_family_key UNIQUE (id, family_id);
CREATE INDEX recursos_class_id_idx ON public.recursos (class_id);

CREATE TABLE public.resource_attribute_values (
    id BIGSERIAL PRIMARY KEY,
    resource_id BIGINT NOT NULL,
    family_id BIGINT NOT NULL,
    resource_attribute_id BIGINT NOT NULL,
    attribute_definition_id BIGINT NOT NULL,
    option_set TEXT NOT NULL DEFAULT 'DEFAULT' REFERENCES public.resource_option_sets(code),
    value_state TEXT NOT NULL DEFAULT 'SET' CHECK (value_state IN ('SET', 'NOT_APPLICABLE')),
    option_code TEXT,
    integer_value BIGINT,
    decimal_value NUMERIC,
    quantity_value NUMERIC,
    quantity_unit_id BIGINT REFERENCES public.unit_definitions(id),
    boolean_value BOOLEAN,
    text_value TEXT,
    UNIQUE (resource_id, resource_attribute_id),
    FOREIGN KEY (resource_id, family_id)
        REFERENCES public.recursos(id, family_id) ON DELETE CASCADE,
    FOREIGN KEY (resource_attribute_id, family_id, attribute_definition_id)
        REFERENCES public.resource_attributes(id, family_id, definition_id),
    FOREIGN KEY (option_set, attribute_definition_id, option_code)
        REFERENCES public.attribute_options(option_set, attribute_definition_id, code),
    CHECK (value_state = 'NOT_APPLICABLE' OR
           num_nonnulls(option_code, integer_value, decimal_value, quantity_value,
                        boolean_value, text_value) = 1),
    CHECK (value_state = 'SET' OR
           num_nonnulls(option_code, integer_value, decimal_value, quantity_value,
                        boolean_value, text_value) = 0),
    CHECK (value_state = 'NOT_APPLICABLE' OR quantity_value IS NULL OR quantity_unit_id IS NOT NULL),
    CHECK (value_state = 'NOT_APPLICABLE' OR text_value IS NULL OR btrim(text_value) <> '')
);

CREATE FUNCTION public.validate_resource_attribute_value()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    definition_value_type TEXT;
BEGIN
    SELECT d.value_type
      INTO definition_value_type
      FROM public.resource_attributes ra
      JOIN public.attribute_definitions d ON d.id = ra.definition_id
     WHERE ra.id = NEW.resource_attribute_id
       AND ra.family_id = NEW.family_id
       AND ra.definition_id = NEW.attribute_definition_id;

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

CREATE TRIGGER resource_attribute_values_validate_type
BEFORE INSERT OR UPDATE ON public.resource_attribute_values
FOR EACH ROW EXECUTE FUNCTION public.validate_resource_attribute_value();

ALTER TABLE public.resource_classes OWNER TO garfex_admin;
ALTER TABLE public.resource_option_sets OWNER TO garfex_admin;
ALTER TABLE public.resource_families OWNER TO garfex_admin;
ALTER TABLE public.resource_types OWNER TO garfex_admin;
ALTER TABLE public.unit_definitions OWNER TO garfex_admin;
ALTER TABLE public.resource_unit_policies OWNER TO garfex_admin;
ALTER TABLE public.attribute_definitions OWNER TO garfex_admin;
ALTER TABLE public.attribute_options OWNER TO garfex_admin;
ALTER TABLE public.attribute_option_relations OWNER TO garfex_admin;
ALTER TABLE public.resource_attributes OWNER TO garfex_admin;
ALTER TABLE public.resource_type_presentation_fields OWNER TO garfex_admin;
ALTER TABLE public.recursos OWNER TO garfex_admin;
ALTER TABLE public.resource_attribute_values OWNER TO garfex_admin;
ALTER FUNCTION public.validate_resource_attribute_value() OWNER TO garfex_admin;

-- resource_classes: matches internal/domain/resource_catalog.go's
-- NewResourceCatalog() exactly (recursos-maestro PR3) — Code is the
-- referential-integrity join key only, never rendered (D10: the Go literal
-- stays the runtime source of truth for names/order/aliases/keywords).
INSERT INTO public.resource_classes (code, name, plural, slug, display_order, active) VALUES
    ('MATERIAL', 'Material', 'Materiales', 'materiales', 1, TRUE),
    ('MANO_DE_OBRA', 'Mano de obra', 'Mano de obra', 'mano-de-obra', 2, TRUE),
    ('EQUIPO_HERRAMIENTA', 'Equipo/Herramienta', 'Equipo/Herramienta', 'equipo-herramienta', 3, TRUE);

INSERT INTO public.resource_families (class_id, code, name, description)
SELECT cl.id, 'CONDUCTORES', 'Conductores', 'Electrical conductors' FROM public.resource_classes cl WHERE cl.code = 'MATERIAL';
INSERT INTO public.resource_families (class_id, code, name, description)
SELECT cl.id, 'CANALIZACIONES', 'Canalizaciones', 'Technical tubes/conduit used as APU materials' FROM public.resource_classes cl WHERE cl.code = 'MATERIAL';

INSERT INTO public.resource_types (class_id, family_id, code, name)
SELECT f.class_id, f.id, 'CABLE', 'Cable' FROM public.resource_families f WHERE f.code = 'CONDUCTORES';
INSERT INTO public.resource_types (class_id, family_id, code, name)
SELECT f.class_id, f.id, 'TUBERIA', 'Tubería' FROM public.resource_families f WHERE f.code = 'CANALIZACIONES';

INSERT INTO public.unit_definitions (code, symbol, dimension) VALUES
    ('M', 'M', 'LENGTH'), ('PZA', 'PZA', 'PIECE');
INSERT INTO public.resource_unit_policies (family_id, unit_id, allowed, suggested)
SELECT f.id, u.id, TRUE, TRUE FROM public.resource_families f, public.unit_definitions u
WHERE f.code = 'CONDUCTORES' AND u.code = 'M';
INSERT INTO public.resource_unit_policies (family_id, unit_id, allowed, suggested)
SELECT f.id, u.id, TRUE, TRUE FROM public.resource_families f, public.unit_definitions u
WHERE f.code = 'CANALIZACIONES' AND u.code = 'PZA';

-- attribute_definitions: Spanish names per recursos-maestro PR3
-- (TestKnownResourceCodesAreNeverRenderedOrTranslated pins every .Code
-- below unchanged). voltage is CONTROLLED_OPTION (folded-in 000005 content).
INSERT INTO public.attribute_definitions (code, name, value_type, dimension, default_identity_participates) VALUES
    ('conductor_material', 'Material del conductor', 'CONTROLLED_OPTION', NULL, TRUE),
    ('gauge', 'Calibre', 'CONTROLLED_OPTION', NULL, TRUE),
    ('insulation', 'Aislamiento', 'CONTROLLED_OPTION', NULL, TRUE),
    ('color', 'Color', 'CONTROLLED_OPTION', NULL, TRUE),
    ('voltage', 'Voltaje', 'CONTROLLED_OPTION', NULL, TRUE),
    ('tipo', 'Tipo', 'CONTROLLED_OPTION', NULL, TRUE),
    ('diameter_inch', 'Diámetro pulgadas', 'CONTROLLED_OPTION', NULL, TRUE),
    ('diameter_mm', 'Diámetro mm', 'CONTROLLED_OPTION', NULL, FALSE);

INSERT INTO public.resource_attributes (class_id, family_id, type_id, definition_id, mode, identity_participates)
SELECT cl.id, f.id, t.id, d.id, 'REQUIRED', TRUE
FROM public.resource_classes cl
JOIN public.resource_families f ON f.class_id = cl.id AND f.code = 'CONDUCTORES'
JOIN public.resource_types t ON t.family_id = f.id AND t.code = 'CABLE'
JOIN public.attribute_definitions d ON d.code IN ('conductor_material', 'gauge', 'insulation')
WHERE cl.code = 'MATERIAL';
INSERT INTO public.resource_attributes (class_id, family_id, type_id, definition_id, mode, identity_participates, condition_definition_id, condition_operator, condition_value, not_applicable_when_condition)
SELECT cl.id, f.id, t.id, d.id, 'CONDITIONAL', TRUE, i.id, 'EQUALS', 'DESNUDO', TRUE
FROM public.resource_classes cl
JOIN public.resource_families f ON f.class_id = cl.id AND f.code = 'CONDUCTORES'
JOIN public.resource_types t ON t.family_id = f.id AND t.code = 'CABLE'
JOIN public.attribute_definitions d ON d.code IN ('color', 'voltage')
JOIN public.attribute_definitions i ON i.code = 'insulation'
WHERE cl.code = 'MATERIAL';
INSERT INTO public.resource_attributes (class_id, family_id, type_id, definition_id, mode, identity_participates)
SELECT cl.id, f.id, t.id, d.id, 'REQUIRED', TRUE
FROM public.resource_classes cl
JOIN public.resource_families f ON f.class_id = cl.id AND f.code = 'CANALIZACIONES'
JOIN public.resource_types t ON t.family_id = f.id AND t.code = 'TUBERIA'
JOIN public.attribute_definitions d ON d.code IN ('tipo', 'diameter_inch')
WHERE cl.code = 'MATERIAL';
INSERT INTO public.resource_attributes (class_id, family_id, type_id, definition_id, mode, identity_participates)
SELECT cl.id, f.id, t.id, d.id, 'REQUIRED', FALSE
FROM public.resource_classes cl
JOIN public.resource_families f ON f.class_id = cl.id AND f.code = 'CANALIZACIONES'
JOIN public.resource_types t ON t.family_id = f.id AND t.code = 'TUBERIA'
JOIN public.attribute_definitions d ON d.code = 'diameter_mm'
WHERE cl.code = 'MATERIAL';

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
CROSS JOIN (VALUES ('300 V'), ('600 V'), ('1000 V'), ('5000 V'), ('15000 V'), ('25000 V'), ('35000 V')) AS x(code)
WHERE d.code = 'voltage';
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

INSERT INTO public.resource_type_presentation_fields (type_id, attribute_definition_id, position)
SELECT t.id, d.id, x.position
FROM (VALUES
    ('CABLE', 'insulation', 1),
    ('CABLE', 'gauge', 2),
    ('CABLE', 'color', 3),
    ('TUBERIA', 'tipo', 1),
    ('TUBERIA', 'diameter_inch', 2)
) AS x(type_code, attribute_code, position)
JOIN public.resource_types t ON t.code = x.type_code
JOIN public.attribute_definitions d ON d.code = x.attribute_code;

GRANT USAGE ON SCHEMA public TO garfex_app;
REVOKE ALL ON
    public.resource_classes, public.resource_option_sets, public.resource_families, public.resource_types,
    public.unit_definitions, public.resource_unit_policies, public.attribute_definitions, public.resource_attributes,
    public.attribute_options, public.attribute_option_relations, public.resource_type_presentation_fields,
    public.recursos, public.resource_attribute_values FROM garfex_app;
REVOKE ALL ON SEQUENCE
    public.resource_classes_id_seq, public.resource_families_id_seq, public.resource_types_id_seq,
    public.unit_definitions_id_seq, public.attribute_definitions_id_seq, public.resource_attributes_id_seq,
    public.attribute_option_relations_id_seq, public.recursos_id_seq, public.resource_attribute_values_id_seq FROM garfex_app;
GRANT SELECT ON
    public.resource_classes, public.resource_option_sets, public.resource_families, public.resource_types,
    public.unit_definitions, public.resource_unit_policies, public.attribute_definitions, public.resource_attributes,
    public.attribute_options, public.attribute_option_relations, public.resource_type_presentation_fields TO garfex_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON
    public.recursos, public.resource_attribute_values TO garfex_app;
GRANT USAGE, SELECT ON SEQUENCE
    public.recursos_id_seq, public.resource_attribute_values_id_seq TO garfex_app;
