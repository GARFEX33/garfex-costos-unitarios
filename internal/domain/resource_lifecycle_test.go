package domain

import "testing"

func TestResourceLifecycleMethodsAreExplicitAndIdempotent(t *testing.T) {
	resource := Resource{Active: true}
	changed := resource.Deactivate()
	if !changed || resource.Active {
		t.Fatalf("Deactivate() changed=%v active=%v, want changed and inactive", changed, resource.Active)
	}
	changed = resource.Deactivate()
	if changed || resource.Active {
		t.Fatalf("repeated Deactivate() changed=%v active=%v, want no-op inactive", changed, resource.Active)
	}
	changed = resource.Reactivate()
	if !changed || !resource.Active {
		t.Fatalf("Reactivate() changed=%v active=%v, want changed and active", changed, resource.Active)
	}
	changed = resource.Reactivate()
	if changed || !resource.Active {
		t.Fatalf("repeated Reactivate() changed=%v active=%v, want no-op active", changed, resource.Active)
	}
}
