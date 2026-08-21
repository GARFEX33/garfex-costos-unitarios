package resourcecore_test

import (
	"context"
	"testing"

	"github.com/GARFEX33/garfex-costos-unitarios/resourcecore"
)

type externalFakeCapabilities struct {
	activeClasses []resourcecore.CatalogRecord
	descriptors   []resourcecore.CatalogDescriptor
}

func (f *externalFakeCapabilities) ActiveClasses(ctx context.Context) ([]resourcecore.CatalogRecord, error) {
	return f.activeClasses, nil
}

func (f *externalFakeCapabilities) CatalogDescriptors(ctx context.Context) ([]resourcecore.CatalogDescriptor, error) {
	return f.descriptors, nil
}

func (f *externalFakeCapabilities) ListCatalog(ctx context.Context, q resourcecore.CatalogQuery) (resourcecore.CatalogPage, error) {
	return resourcecore.CatalogPage{Query: q, Records: []resourcecore.CatalogRecord{{Kind: q.Kind, ID: 1}}}, nil
}

func (f *externalFakeCapabilities) GetCatalog(ctx context.Context, key resourcecore.CatalogKey) (resourcecore.CatalogRecord, error) {
	return resourcecore.CatalogRecord{Kind: key.Kind, ID: key.ID}, nil
}

func (f *externalFakeCapabilities) SearchResources(ctx context.Context, q resourcecore.ResourceQuery) (resourcecore.ResourcePage, error) {
	return resourcecore.ResourcePage{Query: q, Resources: []resourcecore.Resource{{ID: 1}}}, nil
}

func (f *externalFakeCapabilities) GetResource(ctx context.Context, key resourcecore.ResourceKey) (resourcecore.Resource, error) {
	return resourcecore.Resource{ID: 1, Scope: resourcecore.ResourceScope{ClassCode: key.ClassCode}}, nil
}

func (f *externalFakeCapabilities) DescribeResource(ctx context.Context, key resourcecore.ResourceKey) (string, error) {
	return "described", nil
}

func TestExternal_ConstructsReaderWithOnlyPublicTypes(t *testing.T) {
	cap := &externalFakeCapabilities{
		activeClasses: []resourcecore.CatalogRecord{{Kind: resourcecore.KindClass, ID: 1}},
		descriptors:   []resourcecore.CatalogDescriptor{{Kind: resourcecore.KindClass}},
	}
	reader, err := resourcecore.NewReadOnly(cap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	classes, err := reader.ActiveClasses(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(classes) != 1 || classes[0].Kind != resourcecore.KindClass {
		t.Fatalf("unexpected classes: %+v", classes)
	}

	descs, err := reader.CatalogDescriptors(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(descs) != 1 {
		t.Fatalf("unexpected descriptors: %+v", descs)
	}

	page, err := reader.ListCatalog(context.Background(), resourcecore.CatalogQuery{Kind: resourcecore.KindClass})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Records) != 1 || page.Records[0].Kind != resourcecore.KindClass {
		t.Fatalf("unexpected page: %+v", page)
	}

	rec, err := reader.GetCatalog(context.Background(), resourcecore.CatalogKey{Kind: resourcecore.KindClass, ID: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.ID != 7 {
		t.Fatalf("unexpected record: %+v", rec)
	}

	rpage, err := reader.SearchResources(context.Background(), resourcecore.ResourceQuery{Scope: resourcecore.ScopeActive})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rpage.Resources) != 1 {
		t.Fatalf("unexpected resources: %+v", rpage)
	}

	res, err := reader.GetResource(context.Background(), resourcecore.ResourceKey{ClassCode: "MAT", IdentityV1: "v1|x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Scope.ClassCode != "MAT" {
		t.Fatalf("unexpected resource: %+v", res)
	}

	desc, err := reader.DescribeResource(context.Background(), resourcecore.ResourceKey{ClassCode: "MAT", IdentityV1: "v1|x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if desc != "described" {
		t.Fatalf("unexpected description: %q", desc)
	}
}

func TestExternal_NoInternalImports(t *testing.T) {
	// This test lives in resourcecore_test so it can only reference exported
	// symbols from the public module package. Compilation success is the proof.
	_ = resourcecore.KindClass
	_ = resourcecore.ScopeActive
	_ = resourcecore.InvalidArgument
}
