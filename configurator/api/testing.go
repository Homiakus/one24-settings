package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"modbus-configurator/model"
)

func (s *Server) handleSelector(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	var req struct {
		Num  int `json:"num"`  // 1 или 2
		Hole int `json:"hole"` // 1..14
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "неверный JSON: ожидается {\"num\":1|2,\"hole\":1..14}")
		return
	}

	if req.Num < 1 || req.Num > 2 {
		jsonError(w, http.StatusBadRequest, "num должен быть 1 или 2")
		return
	}
	if req.Hole < 1 || req.Hole > 14 {
		jsonError(w, http.StatusBadRequest, "hole должен быть 1..14")
		return
	}

	if err := s.modbus.SetSelector(req.Num, req.Hole); err != nil {
		s.addLog("ERROR", "Селектор "+strconv.Itoa(req.Num)+"→"+strconv.Itoa(req.Hole)+": "+err.Error())
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.addLog("INFO", "Селектор "+strconv.Itoa(req.Num)+" → отверстие "+strconv.Itoa(req.Hole))

	jsonOK(w, map[string]any{
		"selector": req.Num,
		"hole":     req.Hole,
		"status":   "ok",
	})
}

func (s *Server) handleSelectorCalibrate(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	var req struct {
		Num int `json:"num"` // 1 или 2
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "неверный JSON: ожидается {\"num\":1|2}")
		return
	}

	if req.Num < 1 || req.Num > 2 {
		jsonError(w, http.StatusBadRequest, "num должен быть 1 или 2")
		return
	}

	cmd := uint16(100) // calibrate_sel1
	if req.Num == 2 {
		cmd = uint16(110) // calibrate_sel2
	}

	if err := s.modbus.SendCommand(r.Context(), cmd, 300*time.Second); err != nil {
		s.addLog("ERROR", "Калибровка селектора "+strconv.Itoa(req.Num)+": "+err.Error())
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.addLog("INFO", "Калибровка селектора "+strconv.Itoa(req.Num)+" завершена")

	jsonOK(w, map[string]any{
		"selector": req.Num,
		"status":   "calibrated",
	})
}

// ─── Вспомогательные ────────────────────────────────────────────────────────────

func (s *Server) handleSensors(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	snap, err := s.modbus.ReadSensorSnapshot()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.hub.Broadcast(model.WSEvent{
		Type: "sensors",
		Data: snap,
	})

	jsonOK(w, snap)
}

func listCOMPorts() []string {
	ports := make([]string, 16)
	for i := 0; i < 16; i++ {
		ports[i] = "COM" + itoa(i+1)
	}
	return ports
}
