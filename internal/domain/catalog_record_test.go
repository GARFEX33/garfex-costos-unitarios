package domain

import (
	"reflect"
	"testing"
)

func TestCatalogRecord_RulesCopyPreservesRuleSemantics(t *testing.T) {
	record := CatalogRecord{
		Kind: KindAttributeBinding,
		Rules: []CatalogRuleRecord{
			{
				When:                 AttributeCondition{AttributeCode: "insulation", Equals: "DESNUDO"},
				Mode:                 ModeForbidden,
				IdentityParticipates: true,
				NotApplicable:        true,
				Active:               false,
			},
		},
	}

	copied := cloneCatalogRecord(record)
	if !reflect.DeepEqual(copied, record) {
		t.Fatalf("copied record = %#v, want %#v", copied, record)
	}
	if copied.Rules[0].When.AttributeCode != "insulation" || copied.Rules[0].When.Equals != "DESNUDO" {
		t.Fatalf("copied rule When = %#v, want the complete condition", copied.Rules[0].When)
	}
}

func TestCatalogRecord_RulesNilAndEmptyRemainDistinct(t *testing.T) {
	tests := []struct {
		name    string
		rules   []CatalogRuleRecord
		wantNil bool
	}{
		{name: "omitted aggregate", rules: nil, wantNil: true},
		{name: "explicit rule-free aggregate", rules: []CatalogRuleRecord{}, wantNil: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copied := cloneCatalogRecord(CatalogRecord{Rules: tt.rules})
			if (copied.Rules == nil) != tt.wantNil {
				t.Fatalf("copied Rules nil = %t, want %t", copied.Rules == nil, tt.wantNil)
			}
			if len(copied.Rules) != 0 {
				t.Fatalf("copied Rules = %#v, want empty", copied.Rules)
			}
		})
	}
}

func TestCatalogRecord_RuleOrderIsPreservedAndMeaningful(t *testing.T) {
	first := CatalogRuleRecord{When: AttributeCondition{AttributeCode: "insulation", Equals: "DESNUDO"}, Mode: ModeForbidden}
	second := CatalogRuleRecord{When: AttributeCondition{AttributeCode: "insulation", Equals: "THW"}, Mode: ModeOptional}
	ordered := CatalogRecord{Rules: []CatalogRuleRecord{first, second}}
	reordered := CatalogRecord{Rules: []CatalogRuleRecord{second, first}}

	if reflect.DeepEqual(ordered, reordered) {
		t.Fatal("reordered applicability rules compare equal; rule order is semantic")
	}
	copied := cloneCatalogRecord(ordered)
	if copied.Rules[0] != first || copied.Rules[1] != second {
		t.Fatalf("copied rule order = %#v, want [%#v %#v]", copied.Rules, first, second)
	}
}

func TestCatalogRecord_RuleCopyIsolatedFromCallerAndPriorRecord(t *testing.T) {
	rules := []CatalogRuleRecord{
		{When: AttributeCondition{AttributeCode: "insulation", Equals: "DESNUDO"}, Mode: ModeForbidden, NotApplicable: true},
		{When: AttributeCondition{AttributeCode: "insulation", Equals: "THW"}, Mode: ModeRequired, Active: true},
	}
	record := CatalogRecord{Rules: rules}
	copied := cloneCatalogRecord(record)

	rules[0].When.Equals = "caller changed"
	rules[1].Mode = ModeOptional
	record.Rules[0].Active = true
	if copied.Rules[0].When.Equals != "DESNUDO" || copied.Rules[1].Mode != ModeRequired || copied.Rules[0].Active {
		t.Fatalf("copied rules changed through caller storage: %#v", copied.Rules)
	}

	copied.Rules[0].When.AttributeCode = "candidate changed"
	copied.Rules[1].NotApplicable = true
	if record.Rules[0].When.AttributeCode != "insulation" || record.Rules[1].NotApplicable {
		t.Fatalf("prior record changed through copied rules: %#v", record.Rules)
	}
}

func TestCatalogRecord_LegacyZeroValueRulesRemainCompatible(t *testing.T) {
	catalog := ResourceCatalog{Classes: []ResourceClass{{Code: "MATERIAL", Active: true}}}
	record := CatalogRecord{Kind: KindClass, Values: map[string]CatalogValue{
		"code": {Text: "TOOLS"},
	}}

	got, err := ApplyCatalogMutation(catalog, NewCatalogRegistry(), CatalogMutation{Op: OpInsert, Record: record})
	if err != nil {
		t.Fatalf("legacy zero-value rule record failed: %v", err)
	}
	if len(got.Classes) != 2 || got.Classes[1].Code != "TOOLS" {
		t.Fatalf("legacy zero-value mutation = %#v, want the inserted class", got.Classes)
	}
}
