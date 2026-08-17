-- Schema rollback only. Runtime code has no physical deletion path.
DROP TABLE IF EXISTS public.supplier_contacts;
DROP TABLE IF EXISTS public.supplier_branches;
DROP TABLE IF EXISTS public.suppliers;
