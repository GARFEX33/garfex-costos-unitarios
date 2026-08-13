CREATE TABLE public.product_types (
    id BIGSERIAL PRIMARY KEY,
    family_id BIGINT NOT NULL REFERENCES public.material_families(id),
    code TEXT NOT NULL CHECK (btrim(code) <> ''),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE (family_id, code)
);

ALTER TABLE public.product_types
    ADD CONSTRAINT product_types_id_family_key UNIQUE (id, family_id);

ALTER TABLE public.product_types OWNER TO garfex_admin;

-- D9 (approved): TUBERIAS is renamed to CANALIZACIONES, the general Family for
-- canalizaciones-type objects; the concrete object (tubería) becomes a
-- ProductType underneath it, so a future charola/canaleta ProductType can
-- share the same Family without misrepresenting them as "tuberías".
UPDATE public.material_families
    SET code = 'CANALIZACIONES', name = 'Canalizaciones'
    WHERE code = 'TUBERIAS';

INSERT INTO public.product_types (family_id, code, name)
SELECT id, 'CABLE', 'Cable' FROM public.material_families WHERE code = 'CONDUCTORES';
INSERT INTO public.product_types (family_id, code, name)
SELECT id, 'TUBERIA', 'Tubería' FROM public.material_families WHERE code = 'CANALIZACIONES';

ALTER TABLE public.family_attributes
    ADD COLUMN product_type_id BIGINT REFERENCES public.product_types(id);

-- Each family has exactly one ProductType today, so this join is unambiguous.
-- A future migration introducing a second ProductType per family must revisit
-- this (and the family_attributes UNIQUE(family_id, definition_id) constraint,
-- left untouched here — out of scope, not needed until that day comes).
UPDATE public.family_attributes fa
    SET product_type_id = pt.id
    FROM public.product_types pt
    WHERE fa.family_id = pt.family_id;

-- D10 (approved): no real business data exists yet (Phase 0). Existing rows
-- are dev/test fixtures, not a catalog to preserve — they must be regenerated
-- against the new model rather than backfilled with a guessed ProductType.
-- ON DELETE CASCADE on material_attribute_values(material_id, family_id) means
-- this alone also clears their attribute values.
DELETE FROM public.materiales;

ALTER TABLE public.materiales
    ADD COLUMN product_type_id BIGINT NOT NULL REFERENCES public.product_types(id);

ALTER TABLE public.materiales
    ADD CONSTRAINT materiales_product_type_family_fk
    FOREIGN KEY (product_type_id, family_id) REFERENCES public.product_types(id, family_id);

GRANT SELECT ON public.product_types TO garfex_app;
