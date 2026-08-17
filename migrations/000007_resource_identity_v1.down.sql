BEGIN;

ALTER TABLE public.recursos
    DROP CONSTRAINT IF EXISTS recursos_identity_key_v1;

COMMIT;
