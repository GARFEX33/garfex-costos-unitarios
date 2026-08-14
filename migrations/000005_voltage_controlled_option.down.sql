DELETE FROM public.attribute_options
WHERE attribute_definition_id = (SELECT id FROM public.attribute_definitions WHERE code = 'voltage');

UPDATE public.attribute_definitions
    SET value_type = 'QUANTITY', dimension = 'VOLTAGE'
    WHERE code = 'voltage';
