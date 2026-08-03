package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"modbus-configurator/model"
	mb "modbus-configurator/modbus"
)

// ─── Статус и подключение ─────────────────────────────────────────────────────

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	connected := s.modbus != nil && s.modbus.Connected()
	s.state.Connection.Connected = connected

	data := map[string]any{
		"connected":    connected,
		"port":         s.state.Connection.Port,
		"baudrate":     s.state.Connection.Baudrate,
		"slave_id":     s.state.Connection.SlaveID,
		"errors_count": s.state.Connection.ErrorsCount,
	}

	if connected {
		ready, err := s.modbus.ReadRegister(1)
		if err == nil {
			data["ready_status"] = ready
		}
		errCode, err := s.modbus.ReadRegister(5)
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

	if s.modbus != nil {
		s.modbus.Close()
	}

	cfg := mb.DefaultConfig()
	cfg.Port = req.Port
	cfg.Baudrate = req.Baudrate
	cfg.SlaveID = req.SlaveID

	client, err := mb.NewClient(cfg, mb.DefaultRetryConfig())
	if err != nil {
		s.state.Connection.ErrorsCount++
		s.state.Connection.LastError = err.Error()
		s.state.Connection.Connected = false
		jsonError(w, http.StatusInternalServerError, "Не удалось открыть порт: "+err.Error())
		return
	}

	s.modbus = client
	s.state.Connection.Port = req.Port
	s.state.Connection.Baudrate = req.Baudrate
	s.state.Connection.SlaveID = req.SlaveID
	s.state.Connection.Connected = true
	s.state.Connection.LastError = ""

	s.addLog("INFO", "Подключено: "+req.Port+", "+strconv.Itoa(req.Baudrate)+", slave="+strconv.Itoa(req.SlaveID))

	s.hub.Broadcast(model.WSEvent{
		Type: "connect",
		Data: map[string]any{"port": req.Port, "baudrate": req.Baudrate},
	})

	jsonOK(w, map[string]string{"status": "connected", "port": req.Port})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if s.modbus != nil {
		s.modbus.Close()
	}
	s.state.Connection.Connected = false
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
	jsonOK(w, map[string]any{
		"protocol":  "pap_stain",
		"steps":     s.state.Steps,
		"detection": s.state.Detection,
	})
}

func (s *Server) handleGetStep(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id := atoi(idStr)
	if id < 1 || id > 11 {
		jsonError(w, http.StatusBadRequest, "id должен быть 1..11")
		return
	}
	jsonOK(w, s.state.Steps[id-1])
}

func (s *Server) handlePutStep(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id := atoi(idStr)
	if id < 1 || id > 11 {
		jsonError(w, http.StatusBadRequest, "id должен быть 1..11")
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

	if req.ExposureTime < 1 || req.ExposureTime > 600 {
		jsonError(w, http.StatusBadRequest, "exposure_time должен быть 1..600 сек")
		return
	}
	if req.FillVolume < 1 || req.FillVolume > 6000 {
		jsonError(w, http.StatusBadRequest, "fill_volume должен быть 1..6000 (мл×10)")
		return
	}

	s.state.Steps[id-1].ExposureTime = req.ExposureTime
	s.state.Steps[id-1].FillVolume = req.FillVolume
	s.state.Steps[id-1].Dirty = true

	s.addLog("INFO", "Шаг "+idStr+" изменён: t="+strconv.Itoa(req.ExposureTime)+"с, v="+strconv.Itoa(req.FillVolume))

	jsonOK(w, s.state.Steps[id-1])
}

func (s *Server) handleReadAll(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	steps, det, err := s.modbus.ReadAllSettings(s.progressCallback("read"))
	if err != nil {
		s.addLog("ERROR", "Ошибка чтения настроек: "+err.Error())
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.state.Steps = steps
	s.state.Detection = det
	for i := range s.state.Steps {
		s.state.Steps[i].Dirty = false
	}
	s.state.Detection.Dirty = false

	s.addLog("INFO", "Прочитаны настройки: 11 шагов")
	jsonOK(w, map[string]any{
		"steps":     s.state.Steps,
		"detection": s.state.Detection,
	})
}

func (s *Server) handleWriteAll(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	// Пишем ВСЕ шаги (не только dirty) и текущую delt'у
	delta := s.state.Detection.ReagentEmptyDelta

	written, err := s.modbus.WriteAllSettings(s.state.Steps, delta, s.progressCallback("write"))
	if err != nil {
		s.addLog("ERROR", "Ошибка записи настроек: "+err.Error())
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range s.state.Steps {
		s.state.Steps[i].Dirty = false
	}
	s.state.Detection.Dirty = false

	s.addLog("INFO", "Сохранено параметров: "+strconv.Itoa(written))
	jsonOK(w, map[string]any{"written": written})
}

func (s *Server) handleGetDetection(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, s.state.Detection)
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
	s.state.Detection.ReagentEmptyDelta = req.Delta
	s.state.Detection.Dirty = true
	s.addLog("INFO", "Дельта изменена: "+strconv.Itoa(req.Delta))
	jsonOK(w, s.state.Detection)
}

// ─── Настройки переключающих клапанов (селекторов) ───────────────────────────

func (s *Server) handleGetValves(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{
		"selector1": s.state.SelectorPositions1,
		"selector2": s.state.SelectorPositions2,
	})
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

	if sel == 1 {
		s.state.SelectorPositions1[hole].Coord = req.Coord
		s.state.SelectorPositions1[hole].Dirty = true
		s.addLog("INFO", "Клапан 1 отв "+holeStr+" изменён: coord="+strconv.Itoa(req.Coord))
		jsonOK(w, s.state.SelectorPositions1[hole])
	} else {
		s.state.SelectorPositions2[hole].Coord = req.Coord
		s.state.SelectorPositions2[hole].Dirty = true
		s.addLog("INFO", "Клапан 2 отв "+holeStr+" изменён: coord="+strconv.Itoa(req.Coord))
		jsonOK(w, s.state.SelectorPositions2[hole])
	}
}

func (s *Server) handleReadAllValves(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	sel1, sel2, err := s.modbus.ReadAllValvePositions(s.progressCallback("read_valves"))
	if err != nil {
		s.addLog("ERROR", "Ошибка чтения клапанов: "+err.Error())
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.state.SelectorPositions1 = sel1
	s.state.SelectorPositions2 = sel2
	for i := range s.state.SelectorPositions1 {
		s.state.SelectorPositions1[i].Dirty = false
	}
	for i := range s.state.SelectorPositions2 {
		s.state.SelectorPositions2[i].Dirty = false
	}

	s.addLog("INFO", "Прочитаны положения клапанов: 2 клапана × 15 позиций")
	jsonOK(w, map[string]any{
		"selector1": s.state.SelectorPositions1,
		"selector2": s.state.SelectorPositions2,
	})
}

func (s *Server) handleWriteAllValves(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	written, err := s.modbus.WriteAllValvePositions(s.state.SelectorPositions1, s.state.SelectorPositions2, s.progressCallback("write_valves"))
	if err != nil {
		s.addLog("ERROR", "Ошибка записи положений клапанов: "+err.Error())
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range s.state.SelectorPositions1 {
		s.state.SelectorPositions1[i].Dirty = false
	}
	for i := range s.state.SelectorPositions2 {
		s.state.SelectorPositions2[i].Dirty = false
	}

	s.addLog("INFO", "Сохранено положений клапанов: "+strconv.Itoa(written))
	jsonOK(w, map[string]any{"written": written})
}

func modbusName(code uint16) string {
	name := mb.ErrorNames[code]
	if name == "" {
		return "UNKNOWN"
	}
	return name
}
