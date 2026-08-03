package api

import (
	"testing"

	"modbus-configurator/model"
)

func TestDirtyStepsReturnsOnlyChangedValues(t *testing.T) {
	steps := []model.StepParams{
		{ID: 1, Dirty: false},
		{ID: 2, Dirty: true},
		{ID: 3, Dirty: false},
		{ID: 4, Dirty: true},
	}

	got := dirtySteps(steps)
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 4 {
		t.Fatalf("dirtySteps() = %#v, want IDs 2 and 4", got)
	}
}

func TestClearStepDirtyDoesNotClearUnwrittenValues(t *testing.T) {
	steps := []model.StepParams{
		{ID: 1, Dirty: true},
		{ID: 2, Dirty: true},
		{ID: 3, Dirty: false},
	}
	clearStepDirty(steps, []model.StepParams{{ID: 2}})

	if !steps[0].Dirty {
		t.Fatal("step 1 was not written and must remain dirty")
	}
	if steps[1].Dirty {
		t.Fatal("step 2 was written and must be cleared")
	}
}

func TestDirtySelectorPositionsAndClear(t *testing.T) {
	positions := []model.SelectorPosition{
		{Selector: 1, Hole: 0, Dirty: true},
		{Selector: 1, Hole: 1, Dirty: false},
		{Selector: 1, Hole: 2, Dirty: true},
	}

	dirty := dirtySelectorPositions(positions)
	if len(dirty) != 2 || dirty[0].Hole != 0 || dirty[1].Hole != 2 {
		t.Fatalf("dirtySelectorPositions() = %#v, want holes 0 and 2", dirty)
	}

	clearSelectorDirty(positions, []model.SelectorPosition{{Selector: 1, Hole: 2}})
	if !positions[0].Dirty {
		t.Fatal("selector 1 hole 0 was not written and must remain dirty")
	}
	if positions[2].Dirty {
		t.Fatal("selector 1 hole 2 was written and must be cleared")
	}
}
