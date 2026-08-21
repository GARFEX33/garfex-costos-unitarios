package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationTestAdvisoryLock int64 = 0x4741524645583351

type migrationTestState struct {
	mapExists, functionExists, constraintExists, constraintOK bool
	constraintDefinition, resourceIdentityHash, mapHash       string
	resourceCount, mapCount                                   int64
}

type migrationTestWindow struct {
	pool                               *pgxpool.Pool
	lock                               *pgxpool.Conn
	initial                            migrationTestState
	resourceIDs, attributeIDs, unitIDs []int64
	cleanups                           []func(context.Context) error
	restored, handedOff, lockReleased  bool
}

func applySchemaMigration(pool *pgxpool.Pool, name string) error {
	sql, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
	if err != nil {
		return err
	}
	_, err = pool.Exec(context.Background(), string(sql))
	return err
}
func migrationFatal(t *testing.T, label string, err error) {
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
}
func beginMigrationTestWindow(t *testing.T, pool *pgxpool.Pool) *migrationTestWindow {
	if max := pool.Stat().MaxConns(); max < 2 {
		t.Fatalf("migration tests require pool max connections >= 2, got %d", max)
	}
	ctx := context.Background()
	lock, err := pool.Acquire(ctx)
	migrationFatal(t, "acquire migration advisory-lock connection", err)
	w := &migrationTestWindow{pool: pool, lock: lock}
	defer func() {
		if !w.handedOff {
			w.releaseLock()
		}
	}()
	_, err = lock.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationTestAdvisoryLock)
	migrationFatal(t, "acquire migration advisory lock", err)
	w.initial, err = captureMigrationTestState(ctx, pool)
	migrationFatal(t, "capture migration state", err)
	if s := w.initial; s.mapExists != s.functionExists || s.constraintExists && !s.mapExists {
		t.Fatalf("inconsistent migration-5/7 state: map=%t function=%t constraint=%t", s.mapExists, s.functionExists, s.constraintExists)
	}
	t.Cleanup(func() { w.restore(t) })
	migrationFatal(t, "prepare legacy migration state", w.prepareLegacyState(ctx))
	w.handedOff = true
	return w
}
func captureMigrationTestState(ctx context.Context, pool *pgxpool.Pool) (migrationTestState, error) {
	var s migrationTestState
	err := pool.QueryRow(ctx, `SELECT to_regclass('public.resource_integrity_identity_map') IS NOT NULL, to_regprocedure('public.resource_identity_component(text)') IS NOT NULL, EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='public.recursos'::regclass AND conname='recursos_identity_key_v1'), COALESCE((SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='public.recursos'::regclass AND conname='recursos_identity_key_v1'), ''), COALESCE((SELECT convalidated FROM pg_constraint WHERE conrelid='public.recursos'::regclass AND conname='recursos_identity_key_v1'), FALSE), count(*), COALESCE(md5(string_agg(id::text || ':' || identity_key, '|' ORDER BY id)), md5('')) FROM public.recursos`).Scan(&s.mapExists, &s.functionExists, &s.constraintExists, &s.constraintDefinition, &s.constraintOK, &s.resourceCount, &s.resourceIdentityHash)
	if err != nil {
		return migrationTestState{}, err
	}
	if s.mapExists {
		err = pool.QueryRow(ctx, `SELECT count(*), COALESCE(md5(string_agg(resource_id::text || ':' || class_id::text || ':' || legacy_identity_key || ':' || v1_identity_key, '|' ORDER BY resource_id)), md5('')) FROM public.resource_integrity_identity_map`).Scan(&s.mapCount, &s.mapHash)
	}
	return s, err
}
func (w *migrationTestWindow) prepareLegacyState(ctx context.Context) error {
	state, err := captureMigrationTestState(ctx, w.pool)
	if err != nil {
		return err
	}
	if state.constraintExists && !state.mapExists {
		return errors.New("migration-7 constraint has no migration-5 identity map")
	}
	if state.constraintExists {
		if err := applySchemaMigration(w.pool, "000007_resource_identity_v1.down.sql"); err != nil {
			return fmt.Errorf("down migration 7: %w", err)
		}
	}
	return w.downMigration5(ctx)
}
func (w *migrationTestWindow) downMigration5(ctx context.Context) error {
	state, err := captureMigrationTestState(ctx, w.pool)
	if err != nil {
		return err
	}
	if state.mapExists {
		query := `UPDATE public.recursos r SET identity_key=m.legacy_identity_key FROM public.resource_integrity_identity_map m WHERE m.resource_id=r.id AND r.identity_key=m.v1_identity_key`
		var args []any
		if len(w.resourceIDs) > 0 {
			query += ` AND NOT (r.id=ANY($1::bigint[]))`
			args = []any{w.resourceIDs}
		}
		if _, err := w.pool.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("reverse initial migration-7 identities: %w", err)
		}
	}
	if state.mapExists || state.functionExists {
		if err := applySchemaMigration(w.pool, "000005_resource_integrity.down.sql"); err != nil {
			return fmt.Errorf("down migration 5: %w", err)
		}
	}
	return nil
}
func (w *migrationTestWindow) releaseLock() {
	if w.lockReleased {
		return
	}
	_, _ = w.lock.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationTestAdvisoryLock)
	w.lock.Release()
	w.lockReleased = true
}
func (w *migrationTestWindow) TrackResource(id int64)  { w.resourceIDs = append(w.resourceIDs, id) }
func (w *migrationTestWindow) TrackAttribute(id int64) { w.attributeIDs = append(w.attributeIDs, id) }
func (w *migrationTestWindow) TrackUnit(id int64)      { w.unitIDs = append(w.unitIDs, id) }
func appendMigrationError(errs *[]error, label string, err error) {
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: %w", label, err))
	}
}
func (w *migrationTestWindow) restore(t *testing.T) {
	if w.restored {
		return
	}
	ctx := context.Background()
	var errs []error
	appendMigrationError(&errs, "cleanup owned rows", w.cleanupOwned(ctx))
	for i := len(w.cleanups) - 1; i >= 0; i-- {
		appendMigrationError(&errs, "custom cleanup", w.cleanups[i](ctx))
	}
	appendMigrationError(&errs, "normalize migration state", w.prepareLegacyState(ctx))
	if w.initial.mapExists {
		appendMigrationError(&errs, "restore migration 5", applySchemaMigration(w.pool, "000005_resource_integrity.up.sql"))
	}
	if w.initial.constraintExists {
		appendMigrationError(&errs, "restore migration 7", applySchemaMigration(w.pool, "000007_resource_identity_v1.up.sql"))
	}
	if got, err := captureMigrationTestState(ctx, w.pool); err != nil {
		appendMigrationError(&errs, "capture restored migration state", err)
	} else {
		errs = append(errs, compareMigrationTestState(w.initial, got)...)
	}
	w.releaseLock()
	w.restored = true
	for _, err := range errs {
		t.Errorf("migration test restoration: %v", err)
	}
}
func (w *migrationTestWindow) cleanupOwned(ctx context.Context) error {
	ops := []struct {
		ids []int64
		sql string
	}{
		{w.attributeIDs, `DELETE FROM public.resource_attribute_values WHERE resource_attribute_id=ANY($1::bigint[])`},
		{w.attributeIDs, `DELETE FROM public.resource_attributes WHERE id=ANY($1::bigint[])`},
		{w.resourceIDs, `DELETE FROM public.resource_attribute_values WHERE resource_id=ANY($1::bigint[])`},
		{w.resourceIDs, `DELETE FROM public.recursos WHERE id=ANY($1::bigint[])`},
		{w.unitIDs, `DELETE FROM public.resource_unit_policies WHERE unit_id=ANY($1::bigint[])`},
		{w.unitIDs, `DELETE FROM public.unit_definitions WHERE id=ANY($1::bigint[])`},
	}
	var errs []error
	for i, op := range ops {
		if len(op.ids) == 0 {
			continue
		}
		if _, err := w.pool.Exec(ctx, op.sql, op.ids); err != nil {
			errs = append(errs, fmt.Errorf("owned cleanup step %d: %w", i+1, err))
		}
	}
	return errors.Join(errs...)
}
func compareMigrationTestState(want, got migrationTestState) []error {
	if want != got {
		return []error{fmt.Errorf("migration state = %#v, want %#v", got, want)}
	}
	return nil
}

// canonicalIdentityComponent mirrors domain's identityComponent length-prefixed
// encoding byte for byte, so a raw-SQL PostgreSQL fixture that bypasses
// domain.NewResource can still derive an honest v1 identity_key from what it
// actually persists, rather than a synthetic suffix with no persisted backing.
func canonicalIdentityComponent(value string) string {
	return fmt.Sprintf("%d:%s", len([]byte(value)), value)
}

// canonicalResourceIdentityKey derives the v1 identity_key for a raw-SQL
// fixture resource scoped by class/family/type that persists no
// resource_attribute_values rows: the identity has zero identity-participating
// attribute components because zero such components are actually persisted.
// This matches domain.NewResource's "v1|" + class + family + type + attributes
// construction exactly for the always-empty-attributes case, so migration 7's
// recursos_identity_key_v1 CHECK constraint is satisfied honestly.
func canonicalResourceIdentityKey(classCode, familyCode, typeCode string) string {
	return "v1|" + canonicalIdentityComponent(classCode) + canonicalIdentityComponent(familyCode) + canonicalIdentityComponent(typeCode)
}

// TestFixtureIdentityKeyDerivation proves canonicalResourceIdentityKey derives
// v1 identity keys purely from actual persisted scope codes (no synthetic,
// unpersisted suffix), matching domain.NewResource's own length-prefixed
// encoding exactly for the zero-attribute case fixtures below rely on.
func TestFixtureIdentityKeyDerivation(t *testing.T) {
	cases := []struct{ class, family, typ, want string }{
		{"TEST_REPO_CLASS", "TEST_REPO_FAM1", "TEST_REPO_TYPE", "v1|15:TEST_REPO_CLASS14:TEST_REPO_FAM114:TEST_REPO_TYPE"},
		{"MATERIAL", "CONDUCTORES", "CABLE", "v1|8:MATERIAL11:CONDUCTORES5:CABLE"},
	}
	for _, c := range cases {
		if got := canonicalResourceIdentityKey(c.class, c.family, c.typ); got != c.want {
			t.Errorf("canonicalResourceIdentityKey(%q,%q,%q) = %q, want %q", c.class, c.family, c.typ, got, c.want)
		}
	}
}
