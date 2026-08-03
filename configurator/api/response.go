package api

import (
	"encoding/json"
	"net/http"

	"modbus-configurator/model"
)

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(model.APIResponse{OK: true, Data: data})
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(model.APIResponse{OK: false, Error: msg})
}
