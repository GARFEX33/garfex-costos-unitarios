-- 000004_unit_names: add the required human identity for units.
-- Only M and PZA have approved canonical Spanish names. All other values
-- remain explicitly provisional until an administrator corrects them.

ALTER TABLE public.unit_definitions
    ADD COLUMN IF NOT EXISTS name TEXT;

UPDATE public.unit_definitions
   SET name = CASE code
       WHEN 'M' THEN 'Metro'
       WHEN 'PZA' THEN 'Pieza'
       ELSE 'Provisional: Código ' || code
   END
 WHERE name IS NULL OR btrim(name) = '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conrelid = 'public.unit_definitions'::regclass
           AND conname = 'unit_definitions_name_nonblank'
    ) THEN
        ALTER TABLE public.unit_definitions
            ADD CONSTRAINT unit_definitions_name_nonblank CHECK (btrim(name) <> '');
    END IF;
END
$$;

ALTER TABLE public.unit_definitions
    ALTER COLUMN name SET NOT NULL;

ALTER TABLE public.unit_definitions OWNER TO garfex_admin;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.unit_definitions TO garfex_app;
