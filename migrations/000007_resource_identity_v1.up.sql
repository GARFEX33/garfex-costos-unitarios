BEGIN;

UPDATE public.recursos r
   SET identity_key = m.v1_identity_key,
       updated_at = NOW()
  FROM public.resource_integrity_identity_map m
 WHERE m.resource_id = r.id
   AND r.identity_key = m.legacy_identity_key;

ALTER TABLE public.recursos
    ADD CONSTRAINT recursos_identity_key_v1
    CHECK (identity_key LIKE 'v1|%');

COMMIT;
