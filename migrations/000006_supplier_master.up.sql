-- 000006_supplier_master: independent commercial-party master data.
-- Runtime identities can create, read, update, deactivate, and reactivate;
-- physical deletion is intentionally not granted.

CREATE TABLE public.suppliers (
    id BIGSERIAL PRIMARY KEY,
    trade_name TEXT NOT NULL DEFAULT '',
    legal_name TEXT NOT NULL DEFAULT '',
    tax_identifier TEXT,
    website TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT suppliers_meaningful CHECK (
        btrim(trade_name) <> '' OR btrim(legal_name) <> '' OR btrim(COALESCE(tax_identifier, '')) <> ''
    ),
    CONSTRAINT suppliers_tax_identifier_nonblank CHECK (tax_identifier IS NULL OR btrim(tax_identifier) <> '')
);

CREATE UNIQUE INDEX suppliers_tax_identifier_key
    ON public.suppliers (upper(btrim(tax_identifier)))
    WHERE tax_identifier IS NOT NULL;

CREATE TABLE public.supplier_branches (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    branch_reference TEXT NOT NULL DEFAULT '',
    city TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT '',
    address TEXT NOT NULL DEFAULT '',
    general_phone TEXT NOT NULL DEFAULT '',
    general_email TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT supplier_branches_supplier_id_fkey
        FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id) ON DELETE RESTRICT,
    UNIQUE (supplier_id, id)
);

CREATE TABLE public.supplier_contacts (
    id BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL,
    branch_id BIGINT,
    name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    mobile TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT supplier_contacts_name_nonblank CHECK (btrim(name) <> ''),
    CONSTRAINT supplier_contacts_supplier_id_fkey
        FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id) ON DELETE RESTRICT,
    CONSTRAINT supplier_contacts_supplier_branch_fkey
        FOREIGN KEY (supplier_id, branch_id)
        REFERENCES public.supplier_branches (supplier_id, id) ON DELETE RESTRICT
);

CREATE INDEX supplier_branches_supplier_idx
    ON public.supplier_branches (supplier_id, active, id);
CREATE INDEX supplier_contacts_supplier_idx
    ON public.supplier_contacts (supplier_id, active, id);
CREATE INDEX supplier_contacts_branch_idx
    ON public.supplier_contacts (supplier_id, branch_id, active, id)
    WHERE branch_id IS NOT NULL;

CREATE TRIGGER suppliers_set_updated_at
    BEFORE UPDATE ON public.suppliers
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER supplier_branches_set_updated_at
    BEFORE UPDATE ON public.supplier_branches
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();
CREATE TRIGGER supplier_contacts_set_updated_at
    BEFORE UPDATE ON public.supplier_contacts
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

ALTER TABLE public.suppliers OWNER TO garfex_admin;
ALTER TABLE public.supplier_branches OWNER TO garfex_admin;
ALTER TABLE public.supplier_contacts OWNER TO garfex_admin;

GRANT SELECT, INSERT, UPDATE ON
    public.suppliers, public.supplier_branches, public.supplier_contacts TO garfex_app;
REVOKE DELETE ON
    public.suppliers, public.supplier_branches, public.supplier_contacts FROM garfex_app;
GRANT USAGE, SELECT ON SEQUENCE
    public.suppliers_id_seq, public.supplier_branches_id_seq, public.supplier_contacts_id_seq TO garfex_app;
