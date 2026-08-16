-- Non-destructive rollback: retain the populated name column and data so the
-- previous binary can continue using the additive schema safely.
ALTER TABLE public.unit_definitions
    DROP CONSTRAINT IF EXISTS unit_definitions_name_nonblank;
ALTER TABLE public.unit_definitions
    ALTER COLUMN name DROP NOT NULL;
