package taxcategory

import "testing"

func TestCatalogIsCompleteAndUnique(t *testing.T) {
	sectionKeys := map[string]bool{}
	for _, s := range Sections {
		if sectionKeys[s.Key] {
			t.Fatalf("duplicate section %s", s.Key)
		}
		sectionKeys[s.Key] = true
	}

	seen := map[string]bool{}
	for _, c := range Categories {
		if seen[c.Key] {
			t.Fatalf("duplicate category %s", c.Key)
		}
		seen[c.Key] = true
		if !sectionKeys[c.Section] {
			t.Fatalf("category %s points at unknown section %s", c.Key, c.Section)
		}
	}

	if !IsKnownCategory("work_equipment") || !IsKnownCategory("health") {
		t.Fatal("expected core categories to be known")
	}
	if IsKnownCategory("none") || IsKnownCategory("Work") {
		t.Fatal("none and legacy values must not be catalog categories")
	}

	c, ok := CategoryByKey("health")
	if !ok || c.Label != "Krankheitskosten" || c.Section != "extraordinary" {
		t.Fatalf("unexpected health category: %#v", c)
	}
	if len(CategoriesForSection("work_expenses")) != 8 {
		t.Fatalf("unexpected work category count")
	}
	if len(CategoriesForSection(CapitalIncomeSection)) != 0 {
		t.Fatal("capital income is generated, not taggable")
	}
}
