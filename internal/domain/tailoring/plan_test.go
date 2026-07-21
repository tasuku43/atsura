package tailoring

import (
	"errors"
	"testing"
)

func TestOutputPlanValidationRejectsUnsupportedOrAmbiguousTransforms(t *testing.T) {
	tests := map[string]func(*OutputPlan){
		"input":            func(plan *OutputPlan) { plan.Input = "text" },
		"select nil":       func(plan *OutputPlan) { plan.Select = nil },
		"select duplicate": func(plan *OutputPlan) { plan.Select = []string{"number", "number"} },
		"rename nil":       func(plan *OutputPlan) { plan.Rename = nil },
		"rename missing":   func(plan *OutputPlan) { plan.Rename = []Rename{{From: "missing", To: "id"}} },
		"rename collision": func(plan *OutputPlan) { plan.Rename = []Rename{{From: "number", To: "title"}} },
		"render":           func(plan *OutputPlan) { plan.Render = "table" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			plan := validOutputPlan()
			mutate(&plan)
			if err := plan.Validate(); !errors.Is(err, ErrInvalidOutputPlan) {
				t.Fatalf("Validate() error = %v, want ErrInvalidOutputPlan", err)
			}
		})
	}
}

func validOutputPlan() OutputPlan {
	return OutputPlan{Input: InputJSON, Select: []string{"number", "title", "state"}, Rename: []Rename{{From: "number", To: "id"}}, Render: RenderCompactJSON}
}
