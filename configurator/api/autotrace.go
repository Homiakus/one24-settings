package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	core "github.com/Homiakus/autotraceLab/go_engine/core"
)

const maxAutoTraceSceneBytes = 2 << 20

// handleAutoTraceRoute delegates scene routing to the versioned AutoTrace Lab Go core.
// The browser remains responsible for presentation; validation and path planning are
// performed in the backend so Wails, browser and test clients share one contract.
func (s *Server) handleAutoTraceRoute(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAutoTraceSceneBytes)
	var request core.RouteRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		jsonError(w, http.StatusBadRequest, "некорректная AutoTrace-сцена: "+err.Error())
		return
	}
	request.Options = request.Options.Normalize()

	ctx := r.Context()
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 5*time.Second {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	result, err := core.RouteWithContext(ctx, request)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			status = http.StatusRequestTimeout
		}
		jsonError(w, status, "AutoTrace маршрутизация не выполнена: "+err.Error())
		return
	}

	jsonOK(w, result)
}
