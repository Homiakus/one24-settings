package api

import (
	"net/http"

	"modbus-configurator/model"
)

func (s *Server) handleLog(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{
		"entries": s.state.Log,
		"count":   len(s.state.Log),
	})
}

func (s *Server) handleClearLog(w http.ResponseWriter, r *http.Request) {
	s.state.Log = make([]model.LogEntry, 0)
	s.addLog("INFO", "Журнал очищен")
	jsonOK(w, map[string]string{"status": "cleared"})
}

func (s *Server) handleLastError(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	errCode, err := s.modbus.ReadRegister(5)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось прочитать статус: "+err.Error())
		return
	}

	jsonOK(w, map[string]any{
		"code":      errCode,
		"name":      modbusName(errCode),
		"has_error": errCode != 0,
	})
}

func (s *Server) handleClearError(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	if err := s.modbus.WriteRegister(5, 0); err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось сбросить ошибку: "+err.Error())
		return
	}

	s.addLog("INFO", "Ошибка сброшена")
	jsonOK(w, map[string]string{"status": "cleared"})
}
