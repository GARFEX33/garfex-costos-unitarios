package tui

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/internal/app/catalogo"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/app/recursos"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/domain"
	"github.com/GARFEX33/garfex-costos-unitarios/internal/postgres"
	"github.com/charmbracelet/x/ansi"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestCatalogAdminE2EAdminAuthoredStructureDrivesRealResourceCreateFlow is
// the AC #21 proof (design §7, task 10.1) — the single most load-bearing
// scenario of this whole change: a complete Clase→...→Presentación catalog
// structure is authored ENTIRELY through internal/app/catalogo.Service
// (never a Go literal, never raw SQL) against a real PostgreSQL instance, two
// ordered FRESH postgres.LoadResourceCatalog calls prove restart persistence
// (co-satisfying catalog-structure-persistence/Restart Persistence's
// two-load proof), and the UNMODIFIED internal/tui/resource_editor.go create
// flow — driven with the same synthetic keypress pattern as
// synthetic_class_test.go's TestSyntheticFourthClassCreateFlowRendersItsOwnCatalogData
// — is used to create a real domain.Resource scoped to the admin-authored
// structure, whose catalog.Describe() output reflects the admin-configured
// Presentación field.
//
// Per apply-progress's own PR9 "What PR10 needs to know" note (and design
// §7's own pseudocode, unchanged by PR9): this proof drives
// internal/app/catalogo.Service directly, not the guided wizard
// (catalog_wizard.go) — the wizard adds no new domain/service behavior of
// its own (every Create/Update call it triggers is byte-identical to what
// the generic per-kind create flows already produced pre-PR9), so it sits
// off this proof's critical path; wizard-level sequencing already has its
// own dedicated coverage in catalog_wizard_test.go.
//
// Skips unless GARFEX_TEST_DSN is set. Cleanup removes ONLY this test's own
// rows, by their exact repository-assigned id, in reverse dependency order
// (via t.Cleanup's LIFO execution) — never a blanket DELETE
// (docs/architecture/testing-data-boundary.md).
func TestCatalogAdminE2EAdminAuthoredStructureDrivesRealResourceCreateFlow(t *testing.T) {
	dsn := os.Getenv("GARFEX_TEST_DSN")
	if dsn == "" {
		t.Skip("GARFEX_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)

	must := func(label string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
	}

	// boot is the initial snapshot catalogo.NewService validates every
	// mutation against (design D9) — mirrors the exact value main.go loads
	// at boot via postgres.LoadResourceCatalog. It is deliberately NOT the
	// same load used later to drive resource_editor.go (see "fresh" below):
	// reusing it there would silently skip the restart-persistence proof.
	boot, err := postgres.LoadResourceCatalog(ctx, pool)
	must("LoadResourceCatalog (boot snapshot for catalogo.Service)", err)

	svc := catalogo.NewService(postgres.NewCatalogAdminRepository(pool), domain.NewCatalogRegistry(), boot)

	const (
		classCode      = "TEST_E2E_SERVICIO"
		familyCode     = "TEST_E2E_INSTALACION"
		typeCode       = "TEST_E2E_PUESTA_MARCHA"
		definitionCode = "test_e2e_nivel_servicio"
		optionSetCode  = "TEST_E2E_NIVELES"
		option1Code    = "TEST_E2E_OPCION_UNO"
		option2Code    = "TEST_E2E_OPCION_DOS"
		unitCode       = "TEST_E2E_PZA"
		unitSymbol     = "TEST_E2E_PZA"
	)

	// 1. Clase.
	classRec, err := svc.Create(ctx, domain.KindClass, domain.CatalogRecord{
		Active: true,
		Values: map[string]domain.CatalogValue{
			"code": {Text: classCode}, "name": {Text: "Test E2E Servicio"}, "plural": {Text: "Test E2E Servicios"},
			"slug": {Text: "test-e2e-servicio"}, "order": {Int: 99}, "aliases": {List: []string{}}, "keywords": {List: []string{}},
		},
	})
	must("create class", err)
	t.Cleanup(func() { must("cleanup class", svc.Delete(ctx, domain.KindClass, classRec.ID)) })

	// 2. Familia.
	familyRec, err := svc.Create(ctx, domain.KindFamily, domain.CatalogRecord{
		Active: true,
		Values: map[string]domain.CatalogValue{
			"class": {Ref: domain.CatalogRef{Kind: domain.KindClass, Code: classCode}},
			"code":  {Text: familyCode}, "name": {Text: "Test E2E Instalación"},
		},
	})
	must("create family", err)
	t.Cleanup(func() { must("cleanup family", svc.Delete(ctx, domain.KindFamily, familyRec.ID)) })

	// 3. Tipo.
	typeRec, err := svc.Create(ctx, domain.KindType, domain.CatalogRecord{
		Active: true,
		Values: map[string]domain.CatalogValue{
			"class":  {Ref: domain.CatalogRef{Kind: domain.KindClass, Code: classCode}},
			"family": {Ref: domain.CatalogRef{Kind: domain.KindFamily, Code: familyCode}},
			"code":   {Text: typeCode}, "name": {Text: "Test E2E Puesta en marcha"},
		},
	})
	must("create type", err)
	t.Cleanup(func() { must("cleanup type", svc.Delete(ctx, domain.KindType, typeRec.ID)) })

	// 4. Característica.
	definitionRec, err := svc.Create(ctx, domain.KindAttributeDefinition, domain.CatalogRecord{
		Active: true,
		Values: map[string]domain.CatalogValue{
			"code": {Text: definitionCode}, "name": {Text: "Nivel de Servicio Test E2E"},
			"valueType": {Text: "CONTROLLED_OPTION"}, "dimension": {Text: ""}, "defaultIdentityParticipates": {Bool: true},
		},
	})
	must("create characteristic", err)
	t.Cleanup(func() {
		must("cleanup characteristic", svc.Delete(ctx, domain.KindAttributeDefinition, definitionRec.ID))
	})

	// 5. Conjunto de Opciones.
	optionSetRec, err := svc.Create(ctx, domain.KindOptionSet, domain.CatalogRecord{
		Active: true,
		Values: map[string]domain.CatalogValue{"code": {Text: optionSetCode}, "name": {Text: "Test E2E Niveles"}},
	})
	must("create option set", err)
	t.Cleanup(func() { must("cleanup option set", svc.Delete(ctx, domain.KindOptionSet, optionSetRec.ID)) })

	// 6-7. Opciones (2+).
	option1Rec, err := svc.Create(ctx, domain.KindOption, domain.CatalogRecord{
		Active: true,
		Values: map[string]domain.CatalogValue{
			"optionSet":      {Ref: domain.CatalogRef{Kind: domain.KindOptionSet, Code: optionSetCode}},
			"characteristic": {Ref: domain.CatalogRef{Kind: domain.KindAttributeDefinition, Code: definitionCode}},
			"code":           {Text: option1Code}, "label": {Text: "Test E2E Opción Uno"},
		},
	})
	must("create option 1", err)
	t.Cleanup(func() { must("cleanup option 1", svc.Delete(ctx, domain.KindOption, option1Rec.ID)) })

	option2Rec, err := svc.Create(ctx, domain.KindOption, domain.CatalogRecord{
		Active: true,
		Values: map[string]domain.CatalogValue{
			"optionSet":      {Ref: domain.CatalogRef{Kind: domain.KindOptionSet, Code: optionSetCode}},
			"characteristic": {Ref: domain.CatalogRef{Kind: domain.KindAttributeDefinition, Code: definitionCode}},
			"code":           {Text: option2Code}, "label": {Text: "Test E2E Opción Dos"},
		},
	})
	must("create option 2", err)
	t.Cleanup(func() { must("cleanup option 2", svc.Delete(ctx, domain.KindOption, option2Rec.ID)) })

	// 8. Unidad.
	unitRec, err := svc.Create(ctx, domain.KindUnit, domain.CatalogRecord{
		Active: true,
		Values: map[string]domain.CatalogValue{
			"code": {Text: unitCode}, "name": {Text: "Test Unidad"}, "symbol": {Text: unitSymbol}, "dimension": {Text: "PIECE"},
		},
	})
	must("create unit", err)
	t.Cleanup(func() { must("cleanup unit", svc.Delete(ctx, domain.KindUnit, unitRec.ID)) })

	// 9. Política de Unidad — references the Unidad created above, not the
	// migration's seeded PZA row.
	unitPolicyRec, err := svc.Create(ctx, domain.KindUnitPolicy, domain.CatalogRecord{
		Active: true,
		Values: map[string]domain.CatalogValue{
			"class":     {Ref: domain.CatalogRef{Kind: domain.KindClass, Code: classCode}},
			"family":    {Ref: domain.CatalogRef{Kind: domain.KindFamily, Code: familyCode}},
			"unit":      {Ref: domain.CatalogRef{Kind: domain.KindUnit, Code: unitCode}},
			"allowed":   {Bool: true},
			"suggested": {Bool: true},
		},
	})
	must("create unit policy", err)
	t.Cleanup(func() { must("cleanup unit policy", svc.Delete(ctx, domain.KindUnitPolicy, unitPolicyRec.ID)) })

	// 10. Aplicabilidad — binds the característica to the Tipo (not just the
	// Familia), required, participating in Identidad.
	bindingRec, err := svc.Create(ctx, domain.KindAttributeBinding, domain.CatalogRecord{
		Active: true,
		Values: map[string]domain.CatalogValue{
			"class":                {Ref: domain.CatalogRef{Kind: domain.KindClass, Code: classCode}},
			"family":               {Ref: domain.CatalogRef{Kind: domain.KindFamily, Code: familyCode}},
			"type":                 {Ref: domain.CatalogRef{Kind: domain.KindType, Code: typeCode}},
			"characteristic":       {Ref: domain.CatalogRef{Kind: domain.KindAttributeDefinition, Code: definitionCode}},
			"optionSet":            {Ref: domain.CatalogRef{Kind: domain.KindOptionSet, Code: optionSetCode}},
			"mode":                 {Text: "REQUIRED"},
			"identityParticipates": {Bool: true},
		},
	})
	must("create attribute binding", err)
	t.Cleanup(func() { must("cleanup attribute binding", svc.Delete(ctx, domain.KindAttributeBinding, bindingRec.ID)) })

	// 11. Presentación.
	presentationRec, err := svc.Create(ctx, domain.KindPresentationField, domain.CatalogRecord{
		Active: true,
		Values: map[string]domain.CatalogValue{
			"class":          {Ref: domain.CatalogRef{Kind: domain.KindClass, Code: classCode}},
			"family":         {Ref: domain.CatalogRef{Kind: domain.KindFamily, Code: familyCode}},
			"type":           {Ref: domain.CatalogRef{Kind: domain.KindType, Code: typeCode}},
			"characteristic": {Ref: domain.CatalogRef{Kind: domain.KindAttributeDefinition, Code: definitionCode}},
			"position":       {Int: 1},
		},
	})
	must("create presentation field", err)
	t.Cleanup(func() {
		must("cleanup presentation field", svc.Delete(ctx, domain.KindPresentationField, presentationRec.ID))
	})

	// Restart-persistence proof (co-satisfies catalog-structure-persistence/
	// Restart Persistence's two-load requirement): exactly two genuinely FRESH
	// loads, in order, never a reuse of `boot`'s in-memory state. Each loader
	// call opens its own transaction; the second call is the simulated restart
	// process and is the catalog consumed by the resource flow below.
	postWriteCatalog, err := postgres.LoadResourceCatalog(ctx, pool)
	must("LoadResourceCatalog (post-write load 1)", err)
	restartedCatalog, err := postgres.LoadResourceCatalog(ctx, pool)
	must("LoadResourceCatalog (post-write load 2)", err)
	if !reflect.DeepEqual(postWriteCatalog, restartedCatalog) {
		t.Fatalf("post-write catalog loads differ: first=%#v second=%#v", postWriteCatalog, restartedCatalog)
	}
	assertAdminAuthoredStructure(t, postWriteCatalog, classCode, familyCode, typeCode, definitionCode, optionSetCode, option1Code, option2Code, unitCode, unitSymbol)
	assertAdminAuthoredStructure(t, restartedCatalog, classCode, familyCode, typeCode, definitionCode, optionSetCode, option1Code, option2Code, unitCode, unitSymbol)
	catalog := restartedCatalog

	// Drive the UNMODIFIED resource_editor.go create flow against the
	// admin-authored structure, exactly like synthetic_class_test.go's own
	// TestSyntheticFourthClassCreateFlowRendersItsOwnCatalogData — except the
	// creator here is the REAL recursos.Service over the REAL repository,
	// not a fake, so the flow genuinely persists.
	resourcesRepo := postgres.NewResourceRepository(pool)
	resourceSvc := recursos.NewService(resourcesRepo, catalog)
	agentFor := func(code string) InteractionAgent {
		return NewResourcesWorkspaceAdapter(resourceSvc, resourceSvc, resourceSvc, resourceSvc, resourceSvc, resourceSvc, catalog, code)
	}
	descriptors := buildWorkspaceDescriptors(catalog.ActiveClasses(), agentFor)
	m := NewWithWorkspaces(Handlers{}, NewFakeAgent(), descriptors)

	const slug = "test-e2e-servicio"
	if ok := m.enterWorkspace(slug); !ok {
		t.Fatalf("enterWorkspace(%q) = false, want true (buildWorkspaceDescriptors must register the admin-authored class)", slug)
	}

	// Open the workspace-scoped "/" palette and confirm its "Crear ..."
	// leaf — startCreateEditor's real entry point, exactly as a user
	// reaches it. classFilter is pre-seeded, so the flow starts directly at
	// the Familia question.
	m, _ = update(t, m, key('/'))
	m, _ = update(t, m, enter())

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Test E2E Instalación") {
		t.Fatalf("view after starting the create flow = %q, want the admin-authored family %q to render", plain, "Test E2E Instalación")
	}
	m, _ = update(t, m, enter()) // select the family

	plain = ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Test E2E Puesta en marcha") {
		t.Fatalf("view after selecting the family = %q, want the admin-authored type %q to render", plain, "Test E2E Puesta en marcha")
	}
	m, _ = update(t, m, enter()) // select the type

	plain = ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Nivel de Servicio Test E2E") {
		t.Fatalf("view after selecting the type = %q, want the admin-authored characteristic prompt %q to render", plain, "Nivel de Servicio Test E2E")
	}
	if !strings.Contains(plain, "Test E2E Opción Uno") || !strings.Contains(plain, "Test E2E Opción Dos") {
		t.Fatalf("view after selecting the type = %q, want both admin-authored option labels to be selectable", plain)
	}
	m, _ = update(t, m, enter()) // select "Test E2E Opción Uno"

	plain = ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "unidad natural") {
		t.Fatalf("view after answering the characteristic = %q, want the natural-unit question", plain)
	}
	m, _ = update(t, m, enter()) // select the unit (PZA, the admin-configured suggested unit)

	found, err := resourceSvc.Search(ctx, domain.SearchCriteria{ClassCode: classCode})
	must("Search() created resource", err)
	if len(found) != 1 {
		t.Fatalf("Search(ClassCode=%q) = %d resources, want exactly 1 (the create flow must have reached recursos.Service.Create)", classCode, len(found))
	}
	resource := found[0]
	t.Cleanup(func() {
		// recursos.Service/domain.ResourceRepository never hard-deletes (only
		// soft-delete via SetActive — a deliberate project decision), so this
		// test's own created resource row is removed by its own exact id,
		// never a blanket DELETE, so the catalog rows above (which Código
		// immutability would otherwise keep "referenced" forever) can be
		// cleaned up too. resource_attribute_values cascades via its own FK
		// (migrations/000002_resource_master.up.sql).
		if _, err := pool.Exec(ctx, "DELETE FROM public.recursos WHERE id = $1", resource.ID); err != nil {
			t.Errorf("cleanup resource id %d: %v", resource.ID, err)
		}
	})

	if resource.ClassCode != classCode || resource.FamilyCode != familyCode || resource.TypeCode != typeCode {
		t.Fatalf("created resource Class/Family/Type = %q/%q/%q, want %q/%q/%q",
			resource.ClassCode, resource.FamilyCode, resource.TypeCode, classCode, familyCode, typeCode)
	}

	roundTrip, err := resourcesRepo.Get(ctx, classCode, resource.IdentityKey)
	must("Get() round trip", err)

	description := catalog.Describe(roundTrip)
	if !strings.Contains(description, "Test E2E Puesta en marcha") {
		t.Fatalf("Describe() = %q, want the admin-authored type name", description)
	}
	if !strings.Contains(description, option1Code) {
		t.Fatalf("Describe() = %q, want the admin-configured Presentación field to render the chosen option's code %q", description, option1Code)
	}
}

func assertAdminAuthoredStructure(t *testing.T, catalog domain.ResourceCatalog, classCode, familyCode, typeCode, definitionCode, optionSetCode, option1Code, option2Code, unitCode, unitSymbol string) {
	t.Helper()

	var class domain.ResourceClass
	for _, candidate := range catalog.Classes {
		if candidate.Code == classCode {
			class = candidate
			break
		}
	}
	if class.Code != classCode || class.Name != "Test E2E Servicio" || !class.Active {
		t.Fatalf("loaded class = %#v, want TEST_ class authored by catalogo.Service", class)
	}

	var family domain.ResourceFamily
	for _, candidate := range catalog.Families {
		if candidate.ClassCode == classCode && candidate.Code == familyCode {
			family = candidate
			break
		}
	}
	if family.Code != familyCode || family.Name != "Test E2E Instalación" || !family.Active {
		t.Fatalf("loaded family = %#v, want TEST_ family authored by catalogo.Service", family)
	}

	var resourceType domain.ResourceType
	for _, candidate := range catalog.Types {
		if candidate.ClassCode == classCode && candidate.FamilyCode == familyCode && candidate.Code == typeCode {
			resourceType = candidate
			break
		}
	}
	if resourceType.Code != typeCode || resourceType.Name != "Test E2E Puesta en marcha" || !resourceType.Active {
		t.Fatalf("loaded type = %#v, want TEST_ type authored by catalogo.Service", resourceType)
	}

	var definition domain.AttributeDefinition
	for _, candidate := range catalog.Definitions {
		if candidate.Code == definitionCode {
			definition = candidate
			break
		}
	}
	if definition.Code != definitionCode || definition.Name != "Nivel de Servicio Test E2E" {
		t.Fatalf("loaded characteristic = %#v, want TEST_ characteristic authored by catalogo.Service", definition)
	}

	var optionSetFound bool
	for _, candidate := range catalog.Options {
		if candidate.OptionSet == optionSetCode && candidate.AttributeCode == definitionCode && candidate.Code == option1Code {
			optionSetFound = true
		}
	}
	if !optionSetFound {
		t.Fatalf("loaded options missing %q/%q in option set %q", option1Code, option2Code, optionSetCode)
	}
	var option2Found bool
	for _, candidate := range catalog.Options {
		if candidate.OptionSet == optionSetCode && candidate.AttributeCode == definitionCode && candidate.Code == option2Code {
			option2Found = true
		}
	}
	if !option2Found {
		t.Fatalf("loaded options missing %q in option set %q", option2Code, optionSetCode)
	}

	var unit domain.UnitDefinition
	for _, candidate := range catalog.Units {
		if candidate.Code == unitCode {
			unit = candidate
			break
		}
	}
	if unit.Code != unitCode || unit.Symbol != unitSymbol || !unit.Active {
		t.Fatalf("loaded unit = %#v, want TEST_ unit authored by catalogo.Service", unit)
	}

	bindingFound := false
	for _, candidate := range catalog.Attributes {
		if candidate.ClassCode == classCode && candidate.FamilyCode == familyCode && candidate.TypeCode == typeCode && candidate.Definition.Code == definitionCode && candidate.Mode == domain.ModeRequired && candidate.IdentityParticipates {
			bindingFound = true
			break
		}
	}
	if !bindingFound {
		t.Fatalf("loaded applicability missing required identity binding for %q", definitionCode)
	}

	presentationFound := false
	for _, candidate := range catalog.PresentationFields {
		if candidate.ClassCode == classCode && candidate.FamilyCode == familyCode && candidate.TypeCode == typeCode && candidate.AttributeCode == definitionCode && candidate.Position == 1 {
			presentationFound = true
			break
		}
	}
	if !presentationFound {
		t.Fatalf("loaded presentation missing field for %q", definitionCode)
	}

	policyFound := false
	for _, candidate := range catalog.UnitPolicies {
		if candidate.ClassCode == classCode && candidate.FamilyCode == familyCode && candidate.UnitCode == unitCode && candidate.Allowed && candidate.Suggested {
			policyFound = true
			break
		}
	}
	if !policyFound {
		t.Fatalf("loaded unit policy missing allowed/suggested unit %q", unitCode)
	}
}
