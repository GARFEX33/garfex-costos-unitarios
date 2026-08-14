ALTER TABLE public.materiales DROP CONSTRAINT materiales_product_type_family_fk;
ALTER TABLE public.materiales DROP COLUMN product_type_id;
ALTER TABLE public.family_attributes DROP COLUMN product_type_id;
UPDATE public.material_families SET code = 'TUBERIAS', name = 'Tuberías' WHERE code = 'CANALIZACIONES';
DROP TABLE public.product_types;
