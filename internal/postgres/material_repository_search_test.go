package postgres

import "testing"

func TestClampLimitOffset(t *testing.T) {
	tests := []struct {
		name       string
		limit      int
		offset     int
		wantLimit  int
		wantOffset int
	}{
		{name: "non-positive limit falls back to default", limit: 0, offset: 0, wantLimit: defaultSearchLimit, wantOffset: 0},
		{name: "negative limit falls back to default", limit: -5, offset: 0, wantLimit: defaultSearchLimit, wantOffset: 0},
		{name: "positive limit unchanged", limit: 10, offset: 0, wantLimit: 10, wantOffset: 0},
		{name: "negative offset clamps to zero", limit: 10, offset: -3, wantLimit: 10, wantOffset: 0},
		{name: "non-negative offset unchanged", limit: 10, offset: 20, wantLimit: 10, wantOffset: 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLimit, gotOffset := clampLimitOffset(tt.limit, tt.offset)
			if gotLimit != tt.wantLimit || gotOffset != tt.wantOffset {
				t.Fatalf("clampLimitOffset(%d, %d) = (%d, %d), want (%d, %d)", tt.limit, tt.offset, gotLimit, gotOffset, tt.wantLimit, tt.wantOffset)
			}
		})
	}
}
