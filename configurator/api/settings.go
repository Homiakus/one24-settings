package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	mb "modbus-configurator/modbus"
	"modbus-configurator/model"
)

// ─── Статус и подключение ─────────────────────────────────────────────────────

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.modbusMu.RLock()
	mb := s.modbus
	s.modbusMu.RUnlock()
	connected := mb != nil && mb.Connected()

	s.lockState()
	s.state.Connection.Connected = connected
	connection := s.state.Connection
	s.unlockState()

	data := map[string]any{
		"connected":    connected,
		"port":         connection.Port,
		"baudrate":     connection.Baudrate,
		"slave_id":     connection.SlaveID,
		"errors_count": connection.ErrorsCount,
	}

	if connected {
		ready, err := mb.ReadRegister(1)
		if err == nil {
			data["ready_status"] = ready
		}
		errCode, err := mb.ReadRegister(5)
		if err == nil {
			data["status_error"] = errCode
			if errCode != 0 {
				data["error_name"] = modbusName(errCode)
			}
		}
	}

	jsonOK(w, data)
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Port     string `json:"port"`
		Baudrate int    `json:"baudrate"`
		SlaveID  int    `json:"slave_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "неверный JSON")
		return
	}

	if req.Port == "" {
		req.Port = "COM4"
	}
	if req.Baudrate == 0 {
		req.Baudrate = 115200
	}
	if req.SlaveID == 0 {
		req.SlaveID = 1
	}

	s.modbusMu.Lock()
	if s.modbus != nil {
		s.modbus.Close()
	}

	cfg := mb.DefaultConfig()
	cfg.Port = req.Port
	cfg.Baudrate = req.Baudrate
	cfg.SlaveID = req.SlaveID

	client, err := mb.NewClient(cfg, mb.DefaultRetryConfig())
	if err != nil {
		s.modbusMu.Unlock()
		s.lockState()
		s.state.Connection.ErrorsCount++
		s.state.Connection.LastError = err.Error()
		s.state.Connection.Connected = false
		s.unlockState()
		jsonError(w, http.StatusInternalServerError, "Не удалось открыть порт: "+err.Error())
		return
	}

	s.modbus = client
	s.modbusMu.Unlock()
	s.lockState()
	s.state.Connection.Port = req.Port
	s.state.Connection.Baudrate = req.Baudrate
	s.state.Connection.SlaveID = req.SlaveID
	s.state.Connection.Connected = true
	s.state.Connection.LastError = ""
	s.unlockState()

	s.addLog("INFO", "Подключено: "+req.Port+", "+strconv.Itoa(req.Baudrate)+", slave="+strconv.Itoa(req.SlaveID))
	s.hub.Broadcast(model.WSEvent{
		Type: "connect",
		Data: map[string]any{"port": req.Port, "baudrate": req.Baudrate},
	})
	jsonOK(w, map[string]string{"status": "connected", "port": req.Port})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	s.modbusMu.Lock()
	if s.modbus != nil {
		s.modbus.Close()
		s.modbus = nil
	}
	s.modbusMu.Unlock()
	s.lockState()
	s.state.Connection.Connected = false
	s.unlockState()
	s.addLog("INFO", "Отключено")
	s.hub.Broadcast(model.WSEvent{Type: "disconnect", Data: map[string]string{"reason": "user"}})
	jsonOK(w, map[string]string{"status": "disconnected"})
}

func (s *Server) handlePorts(w http.ResponseWriter, r *http.Request) {
	ports := listCOMPorts()
	jsonOK(w, map[string]any{"ports": ports})
}

// ─── Настройки шагов ──────────────────────────────────────────────────────────

func (s *Server) handleGetSteps(w http.ResponseWriter, r *http.Request) {
	s.rlockState()
	steps := append([]model.StepParams(nil), s.state.Steps...)
	detection := s.state.Detection
	s.runlockState()
	jsonOK(w, map[string]any{
		"protocol":  "pap_stain",
		"steps":     steps,
		"detection": detection,
	})
}

func (s *Server) handleGetStep(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id := atoi(idStr)
	if id < 0 || id > 15 {
		jsonError(w, http.StatusBadRequest, "id должен быть 0..15")
		return
	}
	s.rlockState()
	var step model.StepParams
	found := false
	for _, st := range s.state.Steps {
		if st.ID == id {
			step = st
			found = true
			break
		}
	}
	s.runlockState()
	if !found {
		jsonError(w, http.StatusNotFound, "шаг не найден")
		return
	}
	jsonOK(w, step)
}

func (s *Server) handlePutStep(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id := atoi(idStr)
	if id < 0 || id > 15 {
		jsonError(w, http.StatusBadRequest, "id должен быть 0..15")
		return
	}

	var req struct {
		ExposureTime int `json:"exposure_time"`
		FillVolume   int `json:"fill_volume"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "неверный JSON")
		return
	}

	minVal := 1
	if id == 0 {
		minVal = 0
	}
	if req.ExposureTime < minVal || req.ExposureTime > 600 {
		jsonError(w, http.StatusBadRequest, "exposure_time должен быть "+itoa(minVal)+"..600 сек")
		return
	}
	if req.FillVolume < minVal || req.FillVolume > 6000 {
		jsonError(w, http.StatusBadRequest, "fill_volume должен быть "+itoa(minVal)+"..6000 (мл×10)")
		return
	}

	s.lockState()
	var step model.StepParams
	found := false
	for i := range s.state.Steps {
		if s.state.Steps[i].ID == id {
			s.state.Steps[i].ExposureTime = req.ExposureTime
			s.state.Steps[i].FillVolume = req.FillVolume
			s.state.Steps[i].Dirty = true
			step = s.state.Steps[i]
			found = true
			break
		}
	}
	s.unlockState()

	if !found {
		jsonError(w, http.StatusNotFound, "шаг не найден")
		return
	}

	s.addLog("INFO", "Шаг "+idStr+" изменён: t="+strconv.Itoa(req.ExposureTime)+"с, v="+strconv.Itoa(req.FillVolume))
	jsonOK(w, step)
}

func (s *Server) handleReadAll(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	steps, detection, err := s.modbus.ReadAllSettings(s.progressCallback("read"))
	if err != nil {
		s.addLog("ERROR", "Ошибка чтения настроек: "+err.Error())
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range steps {
		steps[i].Dirty = false
	}
	detection.Dirty = false

	s.lockState()
	s.state.Steps = steps
	s.state.Detection = detection
	s.unlockState()

	s.addLog("INFO", "Прочитаны настройки: 11 шагов")
	jsonOK(w, map[string]any{"steps": steps, "detection": detection})
}

func (s *Server) handleWriteAll(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	s.rlockState()
	steps := dirtySteps(s.state.Steps)
	delta := -1
	if s.state.Detection.Dirty {
		delta = s.state.Detection.ReagentEmptyDelta
	}
	s.runlockState()

	if len(steps) == 0 && delta < 0 {
		jsonOK(w, map[string]any{"written": 0})
		return
	}

	written, err := s.modbus.WriteAllSettings(steps, delta, s.progressCallback("write"))
	if err != nil {
		s.addLog("ERROR", "Ошибка записи настроек: "+err.Error())
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.lockState()
	clearStepDirty(s.state.Steps, steps)
	if delta >= 0 {
		s.state.Detection.Dirty = false
	}
	s.unlockState()

	s.addLog("INFO", "Сохранено изменённых параметров: "+strconv.Itoa(written))
	jsonOK(w, map[string]any{"written": written})
}

func (s *Server) handleGetDetection(w http.ResponseWriter, r *http.Request) {
	s.rlockState()
	detection := s.state.Detection
	s.runlockState()
	jsonOK(w, detection)
}

func (s *Server) handlePutDetection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Delta int `json:"delta"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "неверный JSON")
		return
	}
	if req.Delta < 0 || req.Delta > 255 {
		jsonError(w, http.StatusBadRequest, "delta должна быть 0..255")
		return
	}
	s.lockState()
	s.state.Detection.ReagentEmptyDelta = req.Delta
	s.state.Detection.Dirty = true
	detection := s.state.Detection
	s.unlockState()
	s.addLog("INFO", "Дельта изменена: "+strconv.Itoa(req.Delta))
	jsonOK(w, detection)
}

// ─── Настройки переключающих клапанов ────────────────────────────────────────

func (s *Server) handleGetValves(w http.ResponseWriter, r *http.Request) {
	s.rlockState()
	selector1 := append([]model.SelectorPosition(nil), s.state.SelectorPositions1...)
	selector2 := append([]model.SelectorPosition(nil), s.state.SelectorPositions2...)
	s.runlockState()
	jsonOK(w, map[string]any{"selector1": selector1, "selector2": selector2})
}

func (s *Server) handlePutValve(w http.ResponseWriter, r *http.Request) {
	selStr := r.PathValue("selector")
	holeStr := r.PathValue("hole")
	sel := atoi(selStr)
	hole := atoi(holeStr)
	if sel < 1 || sel > 2 {
		jsonError(w, http.StatusBadRequest, "selector должен быть 1 или 2")
		return
	}
	if hole < 0 || hole > 14 {
		jsonError(w, http.StatusBadRequest, "hole должен быть 0..14")
		return
	}

	var req struct {
		Coord int `json:"coord"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "неверный JSON")
		return
	}
	if req.Coord < 0 || req.Coord > 65535 {
		jsonError(w, http.StatusBadRequest, "coord должна быть в диапазоне 0..65535")
		return
	}

	s.lockState()
	var position model.SelectorPosition
	if sel == 1 {
		s.state.SelectorPositions1[hole].Coord = req.Coord
		s.state.SelectorPositions1[hole].Dirty = true
		position = s.state.SelectorPositions1[hole]
	} else {
		s.state.SelectorPositions2[hole].Coord = req.Coord
		s.state.SelectorPositions2[hole].Dirty = true
		position = s.state.SelectorPositions2[hole]
	}
	s.unlockState()

	s.addLog("INFO", "Клапан "+selStr+" отв "+holeStr+" изменён: coord="+strconv.Itoa(req.Coord))
	jsonOK(w, position)
}

func (s *Server) handleReadAllValves(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	selector1, selector2, err := s.modbus.ReadAllValvePositions(s.progressCallback("read_valves"))
	if err != nil {
		s.addLog("ERROR", "Ошибка чтения клапанов: "+err.Error())
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range selector1 {
		selector1[i].Dirty = false
	}
	for i := range selector2 {
		selector2[i].Dirty = false
	}

	s.lockState()
	s.state.SelectorPositions1 = selector1
	s.state.SelectorPositions2 = selector2
	s.unlockState()

	s.addLog("INFO", "Прочитаны положения клапанов: 2 клапана × 15 позиций")
	jsonOK(w, map[string]any{"selector1": selector1, "selector2": selector2})
}

func (s *Server) handleWriteAllValves(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	s.rlockState()
	selector1 := dirtySelectorPositions(s.state.SelectorPositions1)
	selector2 := dirtySelectorPositions(s.state.SelectorPositions2)
	s.runlockState()

	if len(selector1) == 0 && len(selector2) == 0 {
		jsonOK(w, map[string]any{"written": 0})
		return
	}

	written, err := s.modbus.WriteAllValvePositions(selector1, selector2, s.progressCallback("write_valves"))
	if err != nil {
		s.addLog("ERROR", "Ошибка записи положений клапанов: "+err.Error())
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.lockState()
	clearSelectorDirty(s.state.SelectorPositions1, selector1)
	clearSelectorDirty(s.state.SelectorPositions2, selector2)
	s.unlockState()

	s.addLog("INFO", "Сохранено изменённых положений клапанов: "+strconv.Itoa(written))
	jsonOK(w, map[string]any{"written": written})
}

func dirtySteps(steps []model.StepParams) []model.StepParams {
	result := make([]model.StepParams, 0, len(steps))
	for _, step := range steps {
		if step.Dirty {
			result = append(result, step)
		}
	}
	return result
}

func clearStepDirty(all, written []model.StepParams) {
	ids := make(map[int]struct{}, len(written))
	for _, step := range written {
		ids[step.ID] = struct{}{}
	}
	for i := range all {
		if _, ok := ids[all[i].ID]; ok {
			all[i].Dirty = false
		}
	}
}

func dirtySelectorPositions(values []model.SelectorPosition) []model.SelectorPosition {
	result := make([]model.SelectorPosition, 0, len(values))
	for _, value := range values {
		if value.Dirty {
			result = append(result, value)
		}
	}
	return result
}

func clearSelectorDirty(all, written []model.SelectorPosition) {
	keys := make(map[[2]int]struct{}, len(written))
	for _, value := range written {
		keys[[2]int{value.Selector, value.Hole}] = struct{}{}
	}
	for i := range all {
		if _, ok := keys[[2]int{all[i].Selector, all[i].Hole}]; ok {
			all[i].Dirty = false
		}
	}
}

func modbusName(code uint16) string {
	name := mb.ErrorNames[code]
	if name == "" {
		return "UNKNOWN"
	}
	return name
}
