package api

import (
	"testing"

	"modbus-configurator/model"
)

func validTestProfile() settingsProfile {
	profile := settingsProfile{
		Format:  settingsProfileFormat,
		Version: settingsProfileVersion,
		Connection: profileConnection{Port: "COM7", Baudrate: 115200, SlaveID: 2},
		Detection:  profileDetection{ReagentEmptyDelta: 42},
		Steps:      make([]profileStep, 11),
		Selectors: profileSelectors{
			Selector1: make([]profileSelectorPosition, 15),
			Selector2: make([]profileSelectorPosition, 15),
		},
	}
	for i := 0; i < 11; i++ {
		profile.Steps[i] = profileStep{ID: i + 1, Name: "Step", ExposureTime: 10 + i, FillVolume: 50 + i}
	}
	for i := 0; i < 15; i++ {
		profile.Selectors.Selector1[i] = profileSelectorPosition{Hole: i, Name: "S1", Coord: i * 100}
		profile.Selectors.Selector2[i] = profileSelectorPosition{Hole: i, Name: "S2", Coord: i * 200}
	}
	return profile
}

func TestValidateSettingsProfile(t *testing.T) {
	if err := validateSettingsProfile(validTestProfile()); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}
}

func TestValidateSettingsProfileRejectsDuplicateStep(t *testing.T) {
	profile := validTestProfile()
	profile.Steps[10].ID = profile.Steps[0].ID
	if err := validateSettingsProfile(profile); err == nil {
		t.Fatal("expected duplicate step error")
	}
}

func TestValidateSettingsProfileRejectsIncompleteSelector(t *testing.T) {
	profile := validTestProfile()
	profile.Selectors.Selector1 = profile.Selectors.Selector1[:14]
	if err := validateSettingsProfile(profile); err == nil {
		t.Fatal("expected selector length error")
	}
}

func TestApplySettingsProfileMarksOnlyChangesDirty(t *testing.T) {
	state := &ServerState{
		Steps:              makeDefaultSteps(),
		SelectorPositions1: makeDefaultSelectorPositions(1),
		SelectorPositions2: makeDefaultSelectorPositions(2),
		Detection:          model.DetectionParams{ReagentEmptyDelta: 10},
		Connection:         model.ConnectionState{Port: "COM4", Baudrate: 115200, SlaveID: 1},
	}
	profile := profileFromState(state)
	profile.ExportedAt = profile.ExportedAt.UTC()
	profile.Steps[1].ExposureTime++
	profile.Detection.ReagentEmptyDelta++
	profile.Selectors.Selector2[3].Coord++
	profile.Connection.Port = "COM8"

	result := applySettingsProfile(state, profile)
	if result.ChangedSteps != 1 {
		t.Fatalf("changed steps = %d, want 1", result.ChangedSteps)
	}
	if result.ChangedValves != 1 {
		t.Fatalf("changed valves = %d, want 1", result.ChangedValves)
	}
	if !result.DetectionChanged || !result.ConnectionChanged {
		t.Fatalf("expected detection and connection changes: %+v", result)
	}
	if !state.Steps[1].Dirty || state.Steps[0].Dirty {
		t.Fatal("step dirty flags are incorrect")
	}
	if !state.SelectorPositions2[3].Dirty || state.SelectorPositions1[3].Dirty {
		t.Fatal("selector dirty flags are incorrect")
	}
}

func TestProfileFromStateOmitsRuntimeFields(t *testing.T) {
	state := &ServerState{
		Steps:              makeDefaultSteps(),
		SelectorPositions1: makeDefaultSelectorPositions(1),
		SelectorPositions2: makeDefaultSelectorPositions(2),
		Detection:          model.DetectionParams{ReagentEmpty: true, ReagentEmptyDelta: 20, Dirty: true},
		Connection:         model.ConnectionState{Port: "COM4", Baudrate: 115200, SlaveID: 1, Connected: true, ErrorsCount: 9},
	}
	profile := profileFromState(state)
	if profile.Detection.ReagentEmptyDelta != 20 {
		t.Fatalf("delta = %d, want 20", profile.Detection.ReagentEmptyDelta)
	}
	if profile.Connection.Port != "COM4" || len(profile.Steps) != 11 || len(profile.Selectors.Selector1) != 15 {
		t.Fatalf("unexpected export: %+v", profile)
	}
}
