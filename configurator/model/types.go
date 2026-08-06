package model

import "time"

// ─── Connection ───────────────────────────────────────────────────────────────

// ConnectionState описывает состояние подключения Modbus.
type ConnectionState struct {
	Port        string `json:"port"`
	Baudrate    int    `json:"baudrate"`
	SlaveID     int    `json:"slave_id"`
	Connected   bool   `json:"connected"`
	LastError   string `json:"last_error,omitempty"`
	ErrorsCount int    `json:"errors_count"`
	Uptime      string `json:"uptime"`
}

// ─── Steps ────────────────────────────────────────────────────────────────────

// StepParams — параметры одного шага окраски.
type StepParams struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	ExposureTime int    `json:"exposure_time"` // секунды, рег. 38
	FillVolume   int    `json:"fill_volume"`   // мл×10, рег. 39
	Dirty        bool   `json:"dirty"`
}

// DetectionParams — детекция реагента.
type DetectionParams struct {
	ReagentEmpty      bool `json:"reagent_empty"`       // рег. 40
	ReagentEmptyDelta int  `json:"reagent_empty_delta"` // рег. 41
	Dirty             bool `json:"dirty"`
}

// SelectorPosition — параметры одной позиции (отверстия) переключающего клапана.
type SelectorPosition struct {
	Selector int    `json:"selector"` // 1 или 2
	Hole     int    `json:"hole"`     // 0..14
	Name     string `json:"name"`     // "Home", "Воздух", "Гематоксилин Харриса", и т.д.
	Coord    int    `json:"coord"`    // координата отверстия (рег. 21)
	Dirty    bool   `json:"dirty"`
}

// ─── Sensor Snapshot ──────────────────────────────────────────────────────────

// SensorSnapshot — снимок всех датчиков.
type SensorSnapshot struct {
	Inputs      [13]bool `json:"inputs"`       // входы 22..34
	MCUTemp     float64  `json:"mcu_temp"`     // °C
	NTCTemp     float64  `json:"ntc_temp"`     // °C
	HX711Weight int      `json:"hx711_weight"` // raw
}

// ─── Program State ────────────────────────────────────────────────────────────

// ProgramState — состояние выполняемой программы.
type ProgramState struct {
	Program     string `json:"program"`
	Running     bool   `json:"running"`
	Paused      bool   `json:"paused"`
	CurrentStep int    `json:"current_step"`
	TotalSteps  int    `json:"total_steps"`
	StepName    string `json:"step_name,omitempty"`
	Phase       string `json:"phase,omitempty"` // filling, exposure, draining, done, reagent_low
	ElapsedSec  int    `json:"elapsed_sec"`
	TotalSec    int    `json:"total_sec"`
	Message     string `json:"message,omitempty"`
	ReagentLow  bool   `json:"reagent_low"` // флаг: реагент закончился, ждём пользователя
}

// ─── Sequence Builder ─────────────────────────────────────────────────────────

// SequenceStep — один шаг в пользовательской последовательности команд.
type SequenceStep struct {
	Name       string `json:"name"`
	Cmd        uint16 `json:"cmd"`
	Zone       uint16 `json:"zone,omitempty"`        // 0=не менять, 1, 2, 3
	DelaySec   int    `json:"delay_sec,omitempty"`   // задержка после выполнения команды
	TimeoutSec int    `json:"timeout_sec,omitempty"` // таймаут (сек)
}

// CustomSequenceRequest — запрос на запуск последовательности команд.
type CustomSequenceRequest struct {
	Name  string         `json:"name"`
	Steps []SequenceStep `json:"steps"`
}

// ─── Log ──────────────────────────────────────────────────────────────────────

// LogEntry — запись в журнале операций.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // INFO, WARN, ERROR
	Message   string    `json:"message"`
}

// ─── Config ───────────────────────────────────────────────────────────────────

// ServerConfig — конфигурация сервера (modbus-server.toml).
type ServerConfig struct {
	Server   ServerSettings   `toml:"server"`
	Modbus   ModbusSettings   `toml:"modbus"`
	Features FeaturesSettings `toml:"features"`
	Files    FilesSettings    `toml:"files"`
}

// ServerSettings — параметры HTTP-сервера.
type ServerSettings struct {
	Port        int    `toml:"port"`
	Host        string `toml:"host"`
	CORSEnabled bool   `toml:"cors_enabled"`
	LogLevel    string `toml:"log_level"`
}

// ModbusSettings — параметры Modbus RTU.
type ModbusSettings struct {
	Port         string `toml:"port"`
	Baudrate     int    `toml:"baudrate"`
	DataBits     int    `toml:"data_bits"`
	StopBits     int    `toml:"stop_bits"`
	Parity       string `toml:"parity"`
	SlaveID      int    `toml:"slave_id"`
	TimeoutMs    int    `toml:"timeout_ms"`
	Retries      int    `toml:"retries"`
	RetryDelayMs int    `toml:"retry_delay_ms"`
}

// FeaturesSettings — настройки функций.
type FeaturesSettings struct {
	AutoReconnect          bool `toml:"auto_reconnect"`
	SensorPollIntervalMs   int  `toml:"sensor_poll_interval_ms"`
	ProgressEmitIntervalMs int  `toml:"progress_emit_interval_ms"`
}

// FilesSettings — пути к файлам.
type FilesSettings struct {
	UserSettings string `toml:"user_settings"`
	LogFile      string `toml:"log_file"`
}

// ─── API Response ─────────────────────────────────────────────────────────────

// APIResponse — универсальный JSON-ответ API.
type APIResponse struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// ─── WebSocket Event ──────────────────────────────────────────────────────────

// WSEvent — событие, отправляемое через WebSocket.
type WSEvent struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}
