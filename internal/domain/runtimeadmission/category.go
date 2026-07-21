// Package runtimeadmission defines the finite vendor-neutral reasons why a
// source adapter cannot prove that a wrapper plan is executable.
package runtimeadmission

// Category is a safe structural diagnostic. It never carries source text,
// parser causes, provider values, or authorization meaning.
type Category string

const (
	CategoryAdapterContract  Category = "adapter_contract"
	CategorySourceVersion    Category = "source_version"
	CategoryCommand          Category = "command"
	CategoryWrapperOutput    Category = "wrapper_output"
	CategoryArgvGrammar      Category = "argv_grammar"
	CategorySelectorConflict Category = "selector_conflict"
)

// Valid reports whether the category belongs to the closed public mapping.
func (c Category) Valid() bool {
	switch c {
	case CategoryAdapterContract, CategorySourceVersion, CategoryCommand,
		CategoryWrapperOutput, CategoryArgvGrammar, CategorySelectorConflict:
		return true
	default:
		return false
	}
}
