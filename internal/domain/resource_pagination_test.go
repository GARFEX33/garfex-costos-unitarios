package domain

import "testing"

func TestSearchCriteriaNormalizeBoundsPageInputs(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   SearchCriteria
		want int
		err  bool
	}{
		{"default", SearchCriteria{}, DefaultSearchPageSize, false},
		{"maximum", SearchCriteria{Limit: MaxSearchPageSize}, MaxSearchPageSize, false},
		{"negative limit", SearchCriteria{Limit: -1}, 0, true},
		{"oversized limit", SearchCriteria{Limit: MaxSearchPageSize + 1}, 0, true},
		{"negative offset", SearchCriteria{Offset: -1}, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.in.Normalize()
			if (err != nil) != tt.err || (!tt.err && got.Limit != tt.want) {
				t.Fatalf("Normalize() = %+v, %v; want limit %d, error %v", got, err, tt.want, tt.err)
			}
		})
	}
}
func TestResourcePageCarriesCriteriaAndBoundaries(t *testing.T) {
	p := ResourcePage{Criteria: SearchCriteria{Text: "cable", ClassCode: "MATERIAL", Limit: 10, Offset: 10}, Resources: []Resource{{ID: 2}}, HasPrevious: true, HasNext: true}
	if p.Criteria.Text != "cable" || p.Criteria.ClassCode != "MATERIAL" || p.Criteria.Offset != 10 || !p.HasPrevious || !p.HasNext || p.Resources[0].ID != 2 {
		t.Fatalf("page = %+v, want preserved criteria, resource, and boundaries", p)
	}
}
