package runtimeadmission

import "testing"

func TestCategoriesAreFiniteAndDistinct(t *testing.T) {
	categories := []Category{
		CategoryAdapterContract,
		CategorySourceVersion,
		CategoryCommand,
		CategoryWrapperOutput,
		CategoryArgvGrammar,
		CategorySelectorConflict,
	}
	seen := make(map[Category]struct{}, len(categories))
	for _, category := range categories {
		if !category.Valid() || category == "" {
			t.Fatalf("invalid category %q", category)
		}
		if _, duplicate := seen[category]; duplicate {
			t.Fatalf("duplicate category %q", category)
		}
		seen[category] = struct{}{}
	}
	if Category("unknown").Valid() {
		t.Fatal("unknown category is valid")
	}
}
