package api

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"modbus-configurator/modbus"
	"modbus-configurator/model"
	"modbus-configurator/ws"
)

// Server — HTTP-сервер конфигуратора.
type Server struct {
	mux      *http.ServeMux
	modbus   *modbus.Client
	modbusMu sync.RWMutex
	hub      *ws.Hub
	state    *ServerState
	cfg      *ServerConfig
}

// ServerConfig — настройки из main.
type ServerConfig struct {
	RateLimitRPS      int
	APIKey            string
	MaxWSClients      int
	Zone              uint16 // зона по умолчанию (1, 2, или 3)
	OrchestratorState func(context.Context) (any, error)
}

// ServerState — разделяемое состояние с защитой мьютексом.
type ServerState struct {
	mu                 sync.RWMutex
	Steps              []model.StepParams
	Detection          model.DetectionParams
	SelectorPositions1 []model.SelectorPosition
	SelectorPositions2 []model.SelectorPosition
	Connection         model.ConnectionState
	Program            model.ProgramState
	Log                []model.LogEntry
	Zone               uint16 `json:"zone"` // выбранная зона: 1, 2, или 3 (обе)
	startTime          time.Time
	reagentResume      chan bool // сигнал: true=продолжить, false=отменить
}

// New создаёт сервер, регистрирует все маршруты.
func New(mb *modbus.Client, h *ws.Hub, uiFS embed.FS, cfg *ServerConfig) (*Server, error) {
	s := &Server{
		mux:    http.NewServeMux(),
		modbus: mb,
		hub:    h,
		cfg:    cfg,
		state: &ServerState{
			Steps:              makeDefaultSteps(),
			SelectorPositions1: makeDefaultSelectorPositions(1),
			SelectorPositions2: makeDefaultSelectorPositions(2),
			Connection:         model.ConnectionState{Port: "COM4", Baudrate: 115200, SlaveID: 1},
			Log:                make([]model.LogEntry, 0),
			Zone:               zoneDefault(cfg),
			startTime:          time.Now(),
		},
	}
	s.routes(uiFS)
	return s, nil
}

func (s *Server) routes(uiFS embed.FS) {
	// ─── Health check ───────────────────────────────────────────────────────
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		s.modbusMu.RLock()
		mb := s.modbus
		s.modbusMu.RUnlock()
		connected := mb != nil && mb.Connected()
		response := map[string]any{"ok": true, "modbus": connected}
		if s.cfg != nil && s.cfg.OrchestratorState != nil {
			state, err := s.cfg.OrchestratorState(r.Context())
			if err != nil {
				response["orchestrator_error"] = err.Error()
			} else {
				response["orchestrator"] = state
			}
		}
		_ = json.NewEncoder(w).Encode(response)
	})

	// ─── API v1 ────────────────────────────────────────────────────────────
	s.mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	s.mux.HandleFunc("POST /api/v1/connect", s.handleConnect)
	s.mux.HandleFunc("POST /api/v1/disconnect", s.handleDisconnect)
	s.mux.HandleFunc("GET /api/v1/ports", s.handlePorts)

	s.mux.HandleFunc("GET /api/v1/settings/steps", s.handleGetSteps)
	s.mux.HandleFunc("GET /api/v1/settings/steps/{id}", s.handleGetStep)
	s.mux.HandleFunc("PUT /api/v1/settings/steps/{id}", s.handlePutStep)
	s.mux.HandleFunc("POST /api/v1/settings/read-all", s.handleReadAll)
	s.mux.HandleFunc("POST /api/v1/settings/write-all", s.handleWriteAll)
	s.mux.HandleFunc("GET /api/v1/settings/detection", s.handleGetDetection)
	s.mux.HandleFunc("PUT /api/v1/settings/detection", s.handlePutDetection)
	s.mux.HandleFunc("GET /api/v1/settings/zone", s.handleGetZone)
	s.mux.HandleFunc("PUT /api/v1/settings/zone", s.handlePutZone)

	s.mux.HandleFunc("GET /api/v1/settings/valves", s.handleGetValves)
	s.mux.HandleFunc("PUT /api/v1/settings/valves/{selector}/{hole}", s.handlePutValve)
	s.mux.HandleFunc("POST /api/v1/settings/valves/read-all", s.handleReadAllValves)
	s.mux.HandleFunc("POST /api/v1/settings/valves/write-all", s.handleWriteAllValves)

	s.mux.HandleFunc("POST /api/v1/programs/system-check", s.handleSystemCheck)
	s.mux.HandleFunc("POST /api/v1/programs/load", s.handleLoad)
	s.mux.HandleFunc("POST /api/v1/programs/sedimentation", s.handleSedimentation)
	s.mux.HandleFunc("POST /api/v1/programs/stain/start", s.handleStainStart)
	s.mux.HandleFunc("POST /api/v1/programs/stain/pause", s.handleStainPause)
	s.mux.HandleFunc("POST /api/v1/programs/stain/resume", s.handleStainResume)
	s.mux.HandleFunc("POST /api/v1/programs/stain/stop", s.handleStainStop)
	s.mux.HandleFunc("POST /api/v1/programs/wash/start", s.handleWashStart)
	s.mux.HandleFunc("POST /api/v1/programs/full/start", s.handleFullStart)
	s.mux.HandleFunc("POST /api/v1/programs/emergency-stop", s.handleEmergencyStop)
	s.mux.HandleFunc("POST /api/v1/programs/reset-plc", s.handleResetPLC)
	s.mux.HandleFunc("POST /api/v1/programs/stain/reagent-replaced", s.handleReagentReplaced)
	s.mux.HandleFunc("POST /api/v1/programs/stain/reagent-cancel", s.handleReagentCancel)
	s.mux.HandleFunc("POST /api/v1/programs/sequence/execute", s.handleCustomSequence)

	s.mux.HandleFunc("GET /api/v1/testing/sensors", s.handleSensors)
	s.mux.HandleFunc("POST /api/v1/testing/selector", s.handleSelector)
	s.mux.HandleFunc("POST /api/v1/testing/selector/calibrate", s.handleSelectorCalibrate)

	s.mux.HandleFunc("GET /api/v1/diagnostics/log", s.handleLog)
	s.mux.HandleFunc("DELETE /api/v1/diagnostics/log", s.handleClearLog)
	s.mux.HandleFunc("GET /api/v1/diagnostics/error", s.handleLastError)
	s.mux.HandleFunc("POST /api/v1/diagnostics/error/clear", s.handleClearError)

	// ─── WebSocket ─────────────────────────────────────────────────────────
	s.mux.HandleFunc("GET /ws/events", s.hub.ServeHTTP)

	// ─── Статика ───────────────────────────────────────────────────────────
	subFS, err := fs.Sub(uiFS, "ui")
	if err != nil {
		log.Fatalf("[api] embedded UI: %v", err)
	}
	fileServer := http.FileServer(http.FS(subFS))
	s.mux.HandleFunc("/", fileServer.ServeHTTP)
}

// Handler возвращает http.Handler с middleware-цепочкой.
func (s *Server) Handler() http.Handler {
	var h http.Handler = s.mux

	// Цепочка: recovery → request ID → CSRF → rate limit → auth → CORS
	h = recoveryMiddleware(h)
	h = requestIDMiddleware(h)
	h = csrfMiddleware(h)

	if s.cfg != nil {
		h = rateLimitMiddleware(s.cfg.RateLimitRPS)(h)
		h = authMiddleware(s.cfg.APIKey)(h)
		allowedOrigins := []string{"localhost", "127.0.0.1"}
		h = corsMiddleware(allowedOrigins)(h)
	}

	return h
}

// ─── State accessors ──────────────────────────────────────────────────────────

func (s *Server) lockState()    { s.state.mu.Lock() }
func (s *Server) unlockState()  { s.state.mu.Unlock() }
func (s *Server) rlockState()   { s.state.mu.RLock() }
func (s *Server) runlockState() { s.state.mu.RUnlock() }

// progressCallback возвращает modbus.ProgressFunc, транслирующий прогресс в WebSocket.
func (s *Server) progressCallback(op string) modbus.ProgressFunc {
	return func(current, total int, label string) {
		s.hub.Broadcast(model.WSEvent{
			Type: "progress",
			Data: map[string]any{
				"op": op, "current_step": current, "total_steps": total,
				"step_name": label, "running": true,
			},
		})
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (s *Server) checkModbus(w http.ResponseWriter) bool {
	s.modbusMu.RLock()
	mb := s.modbus
	s.modbusMu.RUnlock()
	if mb == nil || !mb.Connected() {
		jsonError(w, http.StatusServiceUnavailable, "Modbus не подключён. Используйте POST /api/v1/connect")
		return false
	}
	return true
}

func (s *Server) addLog(level, msg string) {
	s.state.mu.Lock()
	s.state.Log = append(s.state.Log, model.LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   msg,
	})
	if len(s.state.Log) > 500 {
		s.state.Log = s.state.Log[len(s.state.Log)-500:]
	}
	s.state.mu.Unlock()

	s.hub.Broadcast(model.WSEvent{
		Type: "log",
		Data: map[string]string{"level": level, "message": msg},
	})
	log.Printf("[%s] %s", level, msg)
}

func makeDefaultSteps() []model.StepParams {
	steps := make([]model.StepParams, 16)
	for i := 0; i < 16; i++ {
		steps[i] = model.StepParams{
			ID:           i,
			Name:         modbus.StainStepNames[i],
			ExposureTime: modbus.DefaultExposureTimes[i],
			FillVolume:   modbus.DefaultFillVolumes[i],
		}
	}
	return steps
}

func makeDefaultSelectorPositions(sel int) []model.SelectorPosition {
	positions := make([]model.SelectorPosition, 15)
	coordsSource := modbus.DefaultSelectorCoords
	if sel == 1 {
		coordsSource = modbus.DefaultSelector1Coords
	}
	for i := 0; i <= 14; i++ {
		name := ""
		if i < len(modbus.DefaultSelectorHoleNames) {
			name = modbus.DefaultSelectorHoleNames[i]
		}
		coord := 0
		if i < len(coordsSource) {
			coord = coordsSource[i]
		}
		positions[i] = model.SelectorPosition{
			Selector: sel,
			Hole:     i,
			Name:     name,
			Coord:    coord,
		}
	}
	return positions
}

func atoi(s string) int {
	var v int
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		v = v*10 + int(c-'0')
	}
	return v
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func zoneDefault(cfg *ServerConfig) uint16 {
	if cfg != nil && cfg.Zone >= 1 && cfg.Zone <= 3 {
		return cfg.Zone
	}
	return 3
}

func isProgramEndpoint(path string) bool {
	return len(path) > 20 && path[:20] == "/api/v1/programs/"
}
