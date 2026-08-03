package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"modbus-configurator/model"
)

const (
	settingsProfileFormat  = "onepap24-settings"
	settingsProfileVersion = 1
	maxProfileBodyBytes    = 1 << 20
)

type settingsProfile struct {
	Format     string                   `json:"format"`
	Version    int                      `json:"version"`
	ExportedAt time.Time                `json:"exported_at,omitempty"`
	Connection profileConnection        `json:"connection"`
	Steps      []profileStep             `json:"steps"`
	Detection  profileDetection          `json:"detection"`
	Selectors  profileSelectors          `json:"selectors"`
}

type profileConnection struct {
	Port     string `json:"port"`
	Baudrate int    `json:"baudrate"`
	SlaveID  int    `json:"slave_id"`
}

type profileStep struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	ExposureTime int    `json:"exposure_time"`
	FillVolume   int    `json:"fill_volume"`
}

type profileDetection struct {
	ReagentEmptyDelta int `json:"reagent_empty_delta"`
}

type profileSelectors struct {
	Selector1 []profileSelectorPosition `json:"selector_1"`
	Selector2 []profileSelectorPosition `json:"selector_2"`
}

type profileSelectorPosition struct {
	Hole  int    `json:"hole"`
	Name  string `json:"name"`
	Coord int    `json:"coord"`
}

type profileImportResult struct {
	Profile         settingsProfile `json:"profile"`
	ChangedSteps    int             `json:"changed_steps"`
	ChangedValves   int             `json:"changed_valves"`
	DetectionChanged bool           `json:"detection_changed"`
	ConnectionChanged bool          `json:"connection_changed"`
}

func (s *Server) handleExportProfile(w http.ResponseWriter, r *http.Request) {
	s.rlockState()
	profile := profileFromState(s.state)
	s.runlockState()

	filename := "onepap24-settings-" + time.Now().Format("20060102-150405") + ".json"
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(profile); err != nil {
		s.addLog("ERROR", "Ошибка экспорта JSON-профиля: "+err.Error())
	}
}

func (s *Server) handleImportProfile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxProfileBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var profile settingsProfile
	if err := decoder.Decode(&profile); err != nil {
		jsonError(w, http.StatusBadRequest, "Некорректный JSON-профиль: "+err.Error())
		return
	}
	if decoder.More() {
		jsonError(w, http.StatusBadRequest, "После JSON-профиля обнаружены лишние данные")
		return
	}
	if err := validateSettingsProfile(profile); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	s.lockState()
	result := applySettingsProfile(s.state, profile)
	s.unlockState()

	s.addLog("INFO", fmt.Sprintf(
		"Импортирован JSON-профиль: шагов изменено %d, координат %d, порог=%t, подключение=%t",
		result.ChangedSteps,
		result.ChangedValves,
		result.DetectionChanged,
		result.ConnectionChanged,
	))
	jsonOK(w, result)
}

func profileFromState(state *ServerState) settingsProfile {
	profile := settingsProfile{
		Format:     settingsProfileFormat,
		Version:    settingsProfileVersion,
		ExportedAt: time.Now().UTC(),
		Connection: profileConnection{
			Port:     state.Connection.Port,
			Baudrate: state.Connection.Baudrate,
			SlaveID:  state.Connection.SlaveID,
		},
		Detection: profileDetection{ReagentEmptyDelta: state.Detection.ReagentEmptyDelta},
		Steps:     make([]profileStep, 0, len(state.Steps)),
		Selectors: profileSelectors{
			Selector1: make([]profileSelectorPosition, 0, len(state.SelectorPositions1)),
			Selector2: make([]profileSelectorPosition, 0, len(state.SelectorPositions2)),
		},
	}
	for _, step := range state.Steps {
		profile.Steps = append(profile.Steps, profileStep{
			ID: step.ID, Name: step.Name, ExposureTime: step.ExposureTime, FillVolume: step.FillVolume,
		})
	}
	for _, position := range state.SelectorPositions1 {
		profile.Selectors.Selector1 = append(profile.Selectors.Selector1, profileSelectorPosition{
			Hole: position.Hole, Name: position.Name, Coord: position.Coord,
		})
	}
	for _, position := range state.SelectorPositions2 {
		profile.Selectors.Selector2 = append(profile.Selectors.Selector2, profileSelectorPosition{
			Hole: position.Hole, Name: position.Name, Coord: position.Coord,
		})
	}
	return profile
}

func validateSettingsProfile(profile settingsProfile) error {
	if profile.Format != settingsProfileFormat {
		return fmt.Errorf("неподдерживаемый формат профиля %q; ожидается %q", profile.Format, settingsProfileFormat)
	}
	if profile.Version != settingsProfileVersion {
		return fmt.Errorf("неподдерживаемая версия профиля %d; ожидается %d", profile.Version, settingsProfileVersion)
	}
	if profile.Connection.Port == "" {
		return fmt.Errorf("connection.port не должен быть пустым")
	}
	if profile.Connection.Baudrate < 1200 || profile.Connection.Baudrate > 4_000_000 {
		return fmt.Errorf("connection.baudrate должен быть в диапазоне 1200..4000000")
	}
	if profile.Connection.SlaveID < 1 || profile.Connection.SlaveID > 247 {
		return fmt.Errorf("connection.slave_id должен быть в диапазоне 1..247")
	}
	if len(profile.Steps) != 11 {
		return fmt.Errorf("steps должен содержать ровно 11 шагов, получено %d", len(profile.Steps))
	}
	seenSteps := make(map[int]bool, 11)
	for i, step := range profile.Steps {
		if step.ID < 1 || step.ID > 11 || seenSteps[step.ID] {
			return fmt.Errorf("steps[%d].id должен быть уникальным числом 1..11", i)
		}
		seenSteps[step.ID] = true
		if step.Name == "" {
			return fmt.Errorf("steps[%d].name не должен быть пустым", i)
		}
		if step.ExposureTime < 1 || step.ExposureTime > 600 {
			return fmt.Errorf("steps[%d].exposure_time должен быть в диапазоне 1..600", i)
		}
		if step.FillVolume < 1 || step.FillVolume > 6000 {
			return fmt.Errorf("steps[%d].fill_volume должен быть в диапазоне 1..6000", i)
		}
	}
	if profile.Detection.ReagentEmptyDelta < 0 || profile.Detection.ReagentEmptyDelta > 255 {
		return fmt.Errorf("detection.reagent_empty_delta должен быть в диапазоне 0..255")
	}
	if err := validateProfileSelector("selectors.selector_1", profile.Selectors.Selector1); err != nil {
		return err
	}
	if err := validateProfileSelector("selectors.selector_2", profile.Selectors.Selector2); err != nil {
		return err
	}
	return nil
}

func validateProfileSelector(path string, positions []profileSelectorPosition) error {
	if len(positions) != 15 {
		return fmt.Errorf("%s должен содержать ровно 15 позиций, получено %d", path, len(positions))
	}
	seen := make(map[int]bool, 15)
	for i, position := range positions {
		if position.Hole < 0 || position.Hole > 14 || seen[position.Hole] {
			return fmt.Errorf("%s[%d].hole должен быть уникальным числом 0..14", path, i)
		}
		seen[position.Hole] = true
		if position.Name == "" {
			return fmt.Errorf("%s[%d].name не должен быть пустым", path, i)
		}
		if position.Coord < 0 || position.Coord > 65535 {
			return fmt.Errorf("%s[%d].coord должен быть в диапазоне 0..65535", path, i)
		}
	}
	return nil
}

func applySettingsProfile(state *ServerState, profile settingsProfile) profileImportResult {
	result := profileImportResult{Profile: profile}
	result.ConnectionChanged = state.Connection.Port != profile.Connection.Port ||
		state.Connection.Baudrate != profile.Connection.Baudrate ||
		state.Connection.SlaveID != profile.Connection.SlaveID
	state.Connection.Port = profile.Connection.Port
	state.Connection.Baudrate = profile.Connection.Baudrate
	state.Connection.SlaveID = profile.Connection.SlaveID

	stepsByID := make(map[int]profileStep, len(profile.Steps))
	for _, step := range profile.Steps {
		stepsByID[step.ID] = step
	}
	for i := range state.Steps {
		incoming := stepsByID[state.Steps[i].ID]
		changed := state.Steps[i].Name != incoming.Name ||
			state.Steps[i].ExposureTime != incoming.ExposureTime ||
			state.Steps[i].FillVolume != incoming.FillVolume
		state.Steps[i].Name = incoming.Name
		state.Steps[i].ExposureTime = incoming.ExposureTime
		state.Steps[i].FillVolume = incoming.FillVolume
		state.Steps[i].Dirty = changed
		if changed {
			result.ChangedSteps++
		}
	}

	result.DetectionChanged = state.Detection.ReagentEmptyDelta != profile.Detection.ReagentEmptyDelta
	state.Detection.ReagentEmptyDelta = profile.Detection.ReagentEmptyDelta
	state.Detection.Dirty = result.DetectionChanged

	result.ChangedValves += applyProfileSelector(state.SelectorPositions1, profile.Selectors.Selector1)
	result.ChangedValves += applyProfileSelector(state.SelectorPositions2, profile.Selectors.Selector2)
	return result
}

func applyProfileSelector(target []model.SelectorPosition, incoming []profileSelectorPosition) int {
	byHole := make(map[int]profileSelectorPosition, len(incoming))
	for _, position := range incoming {
		byHole[position.Hole] = position
	}
	changedCount := 0
	for i := range target {
		position := byHole[target[i].Hole]
		changed := target[i].Name != position.Name || target[i].Coord != position.Coord
		target[i].Name = position.Name
		target[i].Coord = position.Coord
		target[i].Dirty = changed
		if changed {
			changedCount++
		}
	}
	return changedCount
}
