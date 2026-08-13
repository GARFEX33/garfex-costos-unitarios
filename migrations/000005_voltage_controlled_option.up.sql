UPDATE public.attribute_definitions
    SET value_type = 'CONTROLLED_OPTION', dimension = NULL
    WHERE code = 'voltage';

INSERT INTO public.attribute_options (attribute_definition_id, code, label)
SELECT d.id, x.code, x.code FROM public.attribute_definitions d
CROSS JOIN (VALUES ('300 V'), ('600 V'), ('1000 V'), ('5000 V'), ('15000 V'), ('25000 V'), ('35000 V')) AS x(code)
WHERE d.code = 'voltage';

-- D10 precedent (see migrations/000003, /000004): no real business data
-- exists yet (Phase 0); existing materiales rows are dev/test fixtures that
-- must be regenerated against the new model, not backfilled — a row with a
-- stored voltage quantity_value would now be inconsistent with voltage's
-- new CONTROLLED_OPTION definition.
DELETE FROM public.materiales;
