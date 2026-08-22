# Design: `supplier-master-core-read`

## Package layout

New `suppliercore/` at repo root (mirrors `resourcecore/`'s location and file split, sized down — no polymorphic `Value`/descriptor machinery is needed since Supplier/Branch/Contact are flat structs):

| File | Responsibility |
| --- | --- |
| `doc.go` | Package contract statement — translation boundary over `internal/modules/suppliers/app.Service` via a bridge; read-only; the presence of this read API is never permission to infer any write operation exists. |
| `types.go` | `Supplier`, `Branch`, `Contact` DTOs; `BranchKey`, `ContactKey`. |
| `queries.go` | `LifecycleScope` + consts; `SupplierQuery`/`BranchQuery`/`ContactQuery`; `SupplierPage`/`BranchPage`/`ContactPage`. |
| `errors.go` | `ErrorCode`, `Error`, `NewError`, `Code`, `IsCode` — 5 categories. |
| `copy.go` | `CloneSupplier`, `CloneBranch`, `CloneContact`, and the three slice-clone helpers. |
| `reader.go` | `ReadCapabilities` interface, `Reader` struct, `NewReadOnly`, the 6 public methods, shape validators. |

New `internal/bridge/suppliercore/adapter.go`: the sole translation layer, implementing `public.ReadCapabilities`.

## Field completeness (every internal field, mapped or justified)

**Supplier** (`domain.Supplier` → public `Supplier`): `ID, TradeName, LegalName, TaxIdentifier, Website, Notes, Active, CreatedAt, UpdatedAt` — all 9 fields map 1:1, no omissions. Flat value type, no slices or pointers.

**Branch** (`domain.Branch` → public `Branch`): `ID, SupplierID, Name, Reference, City, State, Country, Address, GeneralPhone, GeneralEmail, Notes, Active, CreatedAt, UpdatedAt` — all 14 fields map 1:1, no omissions. Flat value type.

**Contact** (`domain.Contact` → public `Contact`): `ID, SupplierID, BranchID *int64, Name, Role, Phone, Mobile, Email, Notes, Active, CreatedAt, UpdatedAt` — all 12 fields map 1:1, no omissions. The one non-flat field, `BranchID *int64`, is the only field needing a real clone (see Defensive copying below).

No internal field is dropped: unlike `resourcecore`'s `CatalogRecord` (a polymorphic `map[string]Value`), these three domain structs are already public-shaped, so `types.go` is close to a structural copy of the domain structs minus the `*Details` builder types (not needed for reads).

## `types.go`

```go
package suppliercore

import "time"

type Supplier struct {
	ID            int64
	TradeName     string
	LegalName     string
	TaxIdentifier string
	Website       string
	Notes         string
	Active        bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Branch struct {
	ID           int64
	SupplierID   int64
	Name         string
	Reference    string
	City         string
	State        string
	Country      string
	Address      string
	GeneralPhone string
	GeneralEmail string
	Notes        string
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Contact struct {
	ID         int64
	SupplierID int64
	BranchID   *int64
	Name       string
	Role       string
	Phone      string
	Mobile     string
	Email      string
	Notes      string
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// BranchKey identifies one Branch for read; Branch has no independent
// identity outside its owning Supplier.
type BranchKey struct {
	SupplierID int64
	BranchID   int64
}

// ContactKey identifies one Contact for read; Contact has no independent
// identity outside its owning Supplier.
type ContactKey struct {
	SupplierID int64
	ContactID  int64
}
```

## `queries.go`

```go
package suppliercore

type LifecycleScope string

const (
	ScopeActive   LifecycleScope = "ACTIVE"
	ScopeInactive LifecycleScope = "INACTIVE"
	ScopeAll      LifecycleScope = "ALL"
)

type SupplierQuery struct {
	Text   string
	Scope  LifecycleScope
	Limit  int
	Offset int
}

// BranchQuery always carries SupplierID — Branch has no independent search.
type BranchQuery struct {
	SupplierID int64
	Text       string
	Scope      LifecycleScope
	Limit      int
	Offset     int
}

// ContactQuery always carries SupplierID; BranchID narrows to one branch's
// contacts when set (nil means "any branch, or no branch").
type ContactQuery struct {
	SupplierID int64
	BranchID   *int64
	Text       string
	Scope      LifecycleScope
	Limit      int
	Offset     int
}

type SupplierPage struct {
	Query       SupplierQuery
	Suppliers   []Supplier
	HasPrevious bool
	HasNext     bool
}

type BranchPage struct {
	Query       BranchQuery
	Branches    []Branch
	HasPrevious bool
	HasNext     bool
}

type ContactPage struct {
	Query       ContactQuery
	Contacts    []Contact
	HasPrevious bool
	HasNext     bool
}
```

## `errors.go`

Byte-for-byte structural mirror of `resourcecore/errors.go`'s mechanism, with exactly 5 categories instead of 15 (proportional to what this module's internal taxonomy — `ErrValidation`/`ErrNotFound`/`ErrConflict` — actually needs, plus the two boundary-only categories every `*core` package needs regardless of domain size):

```go
package suppliercore

import "errors"

type ErrorCode string

const (
	NotFound        ErrorCode = "NOT_FOUND"
	Validation      ErrorCode = "VALIDATION"
	Conflict        ErrorCode = "CONFLICT"
	InvalidArgument ErrorCode = "INVALID_ARGUMENT"
	Internal        ErrorCode = "INTERNAL"
)

type Error struct {
	code    ErrorCode
	message string
}

func (e Error) Error() string   { return e.message }
func (e Error) Code() ErrorCode { return e.code }

func NewError(code ErrorCode, message string) Error { return Error{code: code, message: message} }

func Code(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var e Error
	if errors.As(err, &e) {
		return e.code
	}
	return Internal
}

func IsCode(err error, code ErrorCode) bool { return Code(err) == code }
```

`InvalidArgument` is reserved for boundary shape rejection inside `Reader` itself (never reaches the internal service); `Validation` is reserved for business-rule rejection the internal service returns (e.g. `ErrBranchOwnership`). This mirrors the exact split resourcecore already draws between its own `InvalidArgument` and `Validation` codes.

## `copy.go`

`Supplier` and `Branch` are fully flat (no pointer or slice fields), so their "clone" is a plain value copy — Go struct assignment already deep-copies flat value types. Only `Contact.BranchID *int64` needs a real pointer clone:

```go
package suppliercore

func copyBranchID(id *int64) *int64 {
	if id == nil {
		return nil
	}
	v := *id
	return &v
}

func CloneSupplier(s Supplier) Supplier { return s }

func CloneBranch(b Branch) Branch { return b }

func CloneContact(c Contact) Contact {
	out := c
	out.BranchID = copyBranchID(c.BranchID)
	return out
}

func cloneSupplierSlice(s []Supplier) []Supplier {
	if s == nil {
		return nil
	}
	out := make([]Supplier, len(s))
	copy(out, s)
	return out
}

func cloneBranchSlice(b []Branch) []Branch {
	if b == nil {
		return nil
	}
	out := make([]Branch, len(b))
	copy(out, b)
	return out
}

func cloneContactSlice(c []Contact) []Contact {
	if c == nil {
		return nil
	}
	out := make([]Contact, len(c))
	for i := range c {
		out[i] = CloneContact(c[i])
	}
	return out
}
```

`CloneSupplier`/`CloneBranch` are trivial (documented as such, not dead code) precisely because there's nothing to deep-copy — this divergence from resourcecore's much busier `copy.go` is a direct, honest consequence of the DTOs being flat rather than map/slice-heavy.

## `reader.go`

Same shape-validate → delegate → clone method body pattern as `resourcecore/reader.go`, applied to 6 methods:

```go
package suppliercore

import "context"

type ReadCapabilities interface {
	GetSupplier(context.Context, int64) (Supplier, error)
	SearchSuppliers(context.Context, SupplierQuery) (SupplierPage, error)
	ListBranches(context.Context, BranchQuery) (BranchPage, error)
	GetBranch(context.Context, BranchKey) (Branch, error)
	ListContacts(context.Context, ContactQuery) (ContactPage, error)
	GetContact(context.Context, ContactKey) (Contact, error)
}

type Reader struct {
	cap ReadCapabilities
}

func NewReadOnly(cap ReadCapabilities) (*Reader, error) {
	if cap == nil {
		return nil, NewError(InvalidArgument, "read capabilities are required")
	}
	return &Reader{cap: cap}, nil
}

func (r *Reader) GetSupplier(ctx context.Context, id int64) (Supplier, error) {
	if id <= 0 {
		return Supplier{}, NewError(InvalidArgument, "supplier id must be positive")
	}
	s, err := r.cap.GetSupplier(ctx, id)
	if err != nil {
		return Supplier{}, err
	}
	return CloneSupplier(s), nil
}

func (r *Reader) SearchSuppliers(ctx context.Context, q SupplierQuery) (SupplierPage, error) {
	if err := validateScope(q.Scope); err != nil {
		return SupplierPage{}, err
	}
	page, err := r.cap.SearchSuppliers(ctx, q)
	if err != nil {
		return SupplierPage{}, err
	}
	page.Suppliers = cloneSupplierSlice(page.Suppliers)
	return page, nil
}

func (r *Reader) ListBranches(ctx context.Context, q BranchQuery) (BranchPage, error) {
	if q.SupplierID <= 0 {
		return BranchPage{}, NewError(InvalidArgument, "supplier id must be positive")
	}
	if err := validateScope(q.Scope); err != nil {
		return BranchPage{}, err
	}
	page, err := r.cap.ListBranches(ctx, q)
	if err != nil {
		return BranchPage{}, err
	}
	page.Branches = cloneBranchSlice(page.Branches)
	return page, nil
}

func (r *Reader) GetBranch(ctx context.Context, key BranchKey) (Branch, error) {
	if key.SupplierID <= 0 {
		return Branch{}, NewError(InvalidArgument, "supplier id must be positive")
	}
	if key.BranchID <= 0 {
		return Branch{}, NewError(InvalidArgument, "branch id must be positive")
	}
	b, err := r.cap.GetBranch(ctx, key)
	if err != nil {
		return Branch{}, err
	}
	return CloneBranch(b), nil
}

func (r *Reader) ListContacts(ctx context.Context, q ContactQuery) (ContactPage, error) {
	if q.SupplierID <= 0 {
		return ContactPage{}, NewError(InvalidArgument, "supplier id must be positive")
	}
	if q.BranchID != nil && *q.BranchID <= 0 {
		return ContactPage{}, NewError(InvalidArgument, "branch id must be positive when provided")
	}
	if err := validateScope(q.Scope); err != nil {
		return ContactPage{}, err
	}
	page, err := r.cap.ListContacts(ctx, q)
	if err != nil {
		return ContactPage{}, err
	}
	page.Contacts = cloneContactSlice(page.Contacts)
	return page, nil
}

func (r *Reader) GetContact(ctx context.Context, key ContactKey) (Contact, error) {
	if key.SupplierID <= 0 {
		return Contact{}, NewError(InvalidArgument, "supplier id must be positive")
	}
	if key.ContactID <= 0 {
		return Contact{}, NewError(InvalidArgument, "contact id must be positive")
	}
	c, err := r.cap.GetContact(ctx, key)
	if err != nil {
		return Contact{}, err
	}
	return CloneContact(c), nil
}

func validateScope(s LifecycleScope) error {
	if s != "" && s != ScopeActive && s != ScopeInactive && s != ScopeAll {
		return NewError(InvalidArgument, "invalid lifecycle scope")
	}
	return nil
}
```

Note `ListContacts`' shape validation checks only that a provided `BranchID` is positive — it does **not** and cannot check that the branch belongs to the supplier (that requires a DB lookup). That check happens inside the internal service (`ensureBranchOwnership`, verified live in `app/contact.go:41`) and surfaces through the bridge as `VALIDATION`. This is the exact "shape vs. business validation" split resourcecore's own `Reader`/`Writer` methods already draw.

## Bridge: `internal/bridge/suppliercore/adapter.go`

### Deliberate divergence from resourcecore's two-port split

`resourcecore`'s adapter takes two constructor params (`catalog catalogPort, resources resourcePort`) because two **distinct internal structs** (`catalogo.Service`, `recursos.Service`) back the two aggregates. Suppliers has **one** internal struct (`suppliers/app.Service`) implementing all three aggregates' reads. Splitting into three seam params here would force every caller to pass the same `*app.Service` three times — an artificial split with no real backing distinction. Instead, one narrow seam interface covering exactly the 6 read methods this bridge needs (out of `Service`'s ~20-method full surface) preserves the same principle — the bridge depends on a narrow capability, not a concrete type — without inventing a distinction that doesn't exist internally.

```go
package suppliercore

import (
	"context"
	"errors"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/modules/suppliers/domain"
	public "github.com/GARFEX33/garfex-costos-unitarios/suppliercore"
)

// serviceReader is the narrow seam this bridge depends on — 6 of
// suppliers/app.Service's ~20 methods, matching exactly what public.ReadCapabilities needs.
type serviceReader interface {
	GetSupplier(context.Context, int64) (domain.Supplier, error)
	SearchSuppliers(context.Context, domain.SupplierSearch) ([]domain.Supplier, error)
	GetBranch(context.Context, int64, int64) (domain.Branch, error)
	ListBranches(context.Context, int64, domain.ListCriteria) ([]domain.Branch, error)
	GetContact(context.Context, int64, int64) (domain.Contact, error)
	ListContacts(context.Context, int64, domain.ContactListCriteria) ([]domain.Contact, error)
}

type Adapter struct {
	service serviceReader
}

var _ public.ReadCapabilities = (*Adapter)(nil)

// NewAdapter returns a ReadCapabilities implementation backed by service.
// service may not be nil.
func NewAdapter(service serviceReader) *Adapter {
	return &Adapter{service: service}
}
```

### Pagination — reconciling the verified `Limit` vs `Limit+1` asymmetry

Verified facts driving this design:
- `postgres/supplier.go`'s `searchSuppliersSQL` uses plain `limit(criteria.Limit)` — returns **at most** `Limit` rows, no over-fetch signal.
- `postgres/branch.go` and `postgres/contact.go` both use `limitPlusOne(criteria.Limit)` internally — return **up to `Limit+1`** rows, untrimmed, all the way up through `app.Service` to any caller.
- `postgres.limit(0)` (and negative) already falls back to `defaultLimit = 100` — this is existing internal behavior the public contract must not silently break.

**The naive fix is wrong.** If the bridge always requests `Limit: q.Limit + 1` for suppliers (mirroring resourcecore's `ListCatalog` verbatim) without first resolving `q.Limit <= 0` to a concrete positive number, a caller who left `Limit` at its zero value (asking for "the default page") would have the bridge send `Limit: 1` to `SearchSuppliers`, and postgres's `limit(1)` returns exactly 1 row — silently breaking the default-100 page into a 1-row page. This is a real bug the design must close, not an edge case to leave implicit.

**Resolution:** the bridge declares its own `defaultLimit = 100` constant (matching postgres's private one, since the bridge cannot import it) and resolves the effective limit before deciding how to request:

```go
const defaultLimit = 100

func effectiveLimit(requested int) int {
	if requested <= 0 {
		return defaultLimit
	}
	return requested
}

// buildPage trims results to at most limit entries and reports HasNext.
// overFetched indicates whether requestedFromInternal already asked for one
// extra row (true for suppliers, which the bridge must request explicitly;
// false for branches/contacts, whose postgres layer already over-fetches
// internally regardless of what the bridge asks for).
func trimToPage(results int, limit int) (end int, hasNext bool) {
	if results > limit {
		return limit, true
	}
	return results, false
}
```

Per-entity request/trim behavior:

| Entity | Limit sent to `service.X(...)` | Rows the service can return | Trim |
| --- | --- | --- | --- |
| Supplier | `effectiveLimit(q.Limit) + 1` (bridge must explicitly over-fetch — postgres does not) | up to `effectiveLimit+1` | to `effectiveLimit`, `HasNext = len > effectiveLimit` |
| Branch | `effectiveLimit(q.Limit)` (postgres already over-fetches by 1 internally via `limitPlusOne`) | up to `effectiveLimit+1` | to `effectiveLimit`, `HasNext = len > effectiveLimit` |
| Contact | `effectiveLimit(q.Limit)` (same as Branch) | up to `effectiveLimit+1` | to `effectiveLimit`, `HasNext = len > effectiveLimit` |

`HasPrevious` is `q.Offset > 0` for all three, identical to resourcecore's `buildCatalogPage`.

```go
func (a *Adapter) SearchSuppliers(ctx context.Context, q public.SupplierQuery) (public.SupplierPage, error) {
	limit := effectiveLimit(q.Limit)
	suppliers, err := a.service.SearchSuppliers(ctx, domain.SupplierSearch{
		Text:   q.Text,
		Active: activeFromScope(q.Scope),
		Limit:  limit + 1,
		Offset: q.Offset,
	})
	if err != nil {
		return public.SupplierPage{}, mapError(err)
	}
	end, hasNext := trimToPage(len(suppliers), limit)
	return public.SupplierPage{
		Query:       public.SupplierQuery{Text: q.Text, Scope: q.Scope, Limit: limit, Offset: q.Offset},
		Suppliers:   mapSupplierSlice(suppliers[:end]),
		HasPrevious: q.Offset > 0,
		HasNext:     hasNext,
	}, nil
}

func (a *Adapter) ListBranches(ctx context.Context, q public.BranchQuery) (public.BranchPage, error) {
	limit := effectiveLimit(q.Limit)
	branches, err := a.service.ListBranches(ctx, q.SupplierID, domain.ListCriteria{
		Text:   q.Text,
		Active: activeFromScope(q.Scope),
		Limit:  limit, // postgres already requests limit+1 internally
		Offset: q.Offset,
	})
	if err != nil {
		return public.BranchPage{}, mapError(err)
	}
	end, hasNext := trimToPage(len(branches), limit)
	return public.BranchPage{
		Query:       public.BranchQuery{SupplierID: q.SupplierID, Text: q.Text, Scope: q.Scope, Limit: limit, Offset: q.Offset},
		Branches:    mapBranchSlice(branches[:end]),
		HasPrevious: q.Offset > 0,
		HasNext:     hasNext,
	}, nil
}

// ListContacts follows the identical shape to ListBranches, adding BranchID
// passthrough into domain.ContactListCriteria.
```

`GetSupplier`, `GetBranch`, `GetContact` are pure translate-and-delegate, one call each, no pagination:

```go
func (a *Adapter) GetSupplier(ctx context.Context, id int64) (public.Supplier, error) {
	s, err := a.service.GetSupplier(ctx, id)
	if err != nil {
		return public.Supplier{}, mapError(err)
	}
	return mapSupplier(s), nil
}

func (a *Adapter) GetBranch(ctx context.Context, key public.BranchKey) (public.Branch, error) {
	b, err := a.service.GetBranch(ctx, key.SupplierID, key.BranchID)
	if err != nil {
		return public.Branch{}, mapError(err)
	}
	return mapBranch(b), nil
}

func (a *Adapter) GetContact(ctx context.Context, key public.ContactKey) (public.Contact, error) {
	c, err := a.service.GetContact(ctx, key.SupplierID, key.ContactID)
	if err != nil {
		return public.Contact{}, mapError(err)
	}
	return mapContact(c), nil
}
```

### Error mapping

`errors.Is`-based classification at the bridge, exactly like `internal/bridge/resourcecore/adapter.go`'s `mapError`, but hand-written against the suppliers module's informal sentinel set (there is no `core.Map` equivalent for suppliers to reuse):

| Internal sentinel (`domain....`) | Public `ErrorCode` |
| --- | --- |
| `ErrSupplierNotFound`, `ErrBranchNotFound`, `ErrContactNotFound` (all wrap `ErrNotFound`) | `NOT_FOUND` |
| `ErrBranchOwnership`, any bare `ErrValidation`, `ValidationError` | `VALIDATION` |
| `ErrTaxIdentifierConflict`, any bare `ErrConflict` | `CONFLICT` |
| anything else (including a raw, unwrapped pgx error reaching the bridge through the verified unsanitized `wrapRead` read path) | `INTERNAL`, message built fresh — never the original error's message, so no driver detail can leak |

```go
func mapError(err error) public.Error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return public.NewError(public.NotFound, "supplier master record not found")
	case errors.Is(err, domain.ErrBranchOwnership):
		return public.NewError(public.Validation, "branch does not belong to supplier")
	case errors.Is(err, domain.ErrValidation):
		return public.NewError(public.Validation, "supplier master validation failed")
	case errors.Is(err, domain.ErrConflict):
		return public.NewError(public.Conflict, "supplier master conflict")
	default:
		return public.NewError(public.Internal, "supplier master read failed")
	}
}
```

The `default` branch is what closes the verified `wrapRead`-does-not-sanitize gap: regardless of what the internal error's `.Error()` string contains (potentially raw pgx/SQLSTATE detail), the public `Error`'s message is always the fixed literal `"supplier master read failed"`, never a formatted wrap of the original error.

## Compiled-surface guard (Reader-side equivalent of `TestWriter_NoUngraduatedMethodExported`)

Mirrors `resourcecore/writer_test.go:216-`'s reflection pattern exactly, applied to the read side:

```go
func TestReader_NoUngraduatedMethodExported(t *testing.T) {
	allowed := map[string]bool{
		"GetSupplier": true, "SearchSuppliers": true,
		"ListBranches": true, "GetBranch": true,
		"ListContacts": true, "GetContact": true,
	}

	capType := reflect.TypeOf((*ReadCapabilities)(nil)).Elem()
	if capType.NumMethod() != len(allowed) {
		t.Fatalf("ReadCapabilities has %d methods, want %d", capType.NumMethod(), len(allowed))
	}
	for i := 0; i < capType.NumMethod(); i++ {
		if !allowed[capType.Method(i).Name] {
			t.Errorf("ReadCapabilities declares ungraduated method %s", capType.Method(i).Name)
		}
	}

	readerType := reflect.TypeOf(&Reader{})
	for i := 0; i < readerType.NumMethod(); i++ {
		if !allowed[readerType.Method(i).Name] {
			t.Errorf("Reader exports ungraduated method %s", readerType.Method(i).Name)
		}
	}
}
```

This is what makes "no Writer exists yet" a compile-visible, provable fact for this change rather than an informal claim — the same role the write-side guard plays for `HardDeleteResource`'s absence in `resourcecore`.

## Delivery split — confirming the proposal's 3-unit forecast

The proposal forecast 700–1,000 lines across 3 units. With the concrete design above:

1. **Public contract** (`suppliercore/`: doc.go, types.go, queries.go, errors.go, copy.go, reader.go + unit tests + the reflection guard) — roughly 250–300 authored lines of package code (smaller than resourcecore's own Reader because there's no polymorphic Value/descriptor machinery), plus a comparable amount of test code. **Confirmed as its own unit**, likely nearer the lower end of the forecast (~350–450 lines total).
2. **Supplier bridge** (`internal/bridge/suppliercore/adapter.go`'s `serviceReader` seam, `GetSupplier`, `SearchSuppliers`, `mapError`, `mapSupplier`, the `effectiveLimit`/`trimToPage` pagination helpers, field-completeness and error-category tests for Supplier only) — **confirmed as its own unit**; this unit carries the pagination-asymmetry fix, so it is not purely mechanical translation and deserves isolated review.
3. **Branch/Contact bridge and readiness** (`GetBranch`, `ListBranches`, `GetContact`, `ListContacts`, `mapBranch`/`mapContact`, the branch-ownership `VALIDATION` scenario, external-consumer test, `doc.go` finalization, readiness record) — **confirmed as its own unit**; reuses unit 2's pagination helpers without modification, keeping it closer to mechanical.

No revision to the proposal's 3-unit split is needed. Each unit stays independently a plausible ≤400-changed-line slice under strict TDD, consistent with the project's Review Workload Guard.
