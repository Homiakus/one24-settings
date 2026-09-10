package main

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

// Config — корневая конфигурация приложения.
type Config struct {
	Server   ServerConfig   `toml:"server"`
	Modbus   ModbusConfig   `toml:"modbus"`
	Features FeatureConfig  `toml:"features"`
	Security SecurityConfig `toml:"security"`
	Files    FileConfig     `toml:"files"`
}

// ServerConfig — параметры HTTP-сервера.
type ServerConfig struct {
	Port               int    `toml:"port"`
	Host               string `toml:"host"`
	ReadTimeoutSec     int    `toml:"read_timeout_sec"`
	WriteTimeoutSec    int    `toml:"write_timeout_sec"`
	ShutdownTimeoutSec int    `toml:"shutdown_timeout_sec"`
}

// ModbusConfig — параметры Modbus RTU.
type ModbusConfig struct {
	Port         string `toml:"port"`
	Baudrate     int    `toml:"baudrate"`
	DataBits     int    `toml:"data_bits"`
	StopBits     int    `toml:"stop_bits"`
	Parity       string `toml:"parity"`
	SlaveID      int    `toml:"slave_id"`
	TimeoutMs    int    `toml:"timeout_ms"`
	Retries      int    `toml:"retries"`
	RetryDelayMs int    `toml:"retry_delay_ms"`
	Zone         int    `toml:"zone"` // зона обработки: 1, 2, или 3 (обе)
}

// FeatureConfig — настройки функций.
type FeatureConfig struct {
	AutoReconnect          bool `toml:"auto_reconnect"`
	SensorPollIntervalMs   int  `toml:"sensor_poll_interval_ms"`
	ProgressEmitIntervalMs int  `toml:"progress_emit_interval_ms"`
}

// SecurityConfig — настройки безопасности.
type SecurityConfig struct {
	APIKey          string `toml:"api_key"`
	RateLimitPerSec int    `toml:"rate_limit_per_sec"`
	MaxWSClients    int    `toml:"max_ws_clients"`
}

// FileConfig — пути к файлам.
type FileConfig struct {
	UserSettings    string `toml:"user_settings"`
	LogFile         string `toml:"log_file"`
	OrchestratorDir string `toml:"orchestrator_dir"`
}

// DefaultConfig возвращает конфигурацию с безопасными значениями по умолчанию.
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port:               0,
			Host:               "127.0.0.1",
			ReadTimeoutSec:     30,
			WriteTimeoutSec:    0,
			ShutdownTimeoutSec: 10,
		},
		Modbus: ModbusConfig{
			Port:         "COM4",
			Baudrate:     115200,
			DataBits:     8,
			StopBits:     1,
			Parity:       "N",
			SlaveID:      1,
			TimeoutMs:    500,
			Retries:      3,
			RetryDelayMs: 300,
			Zone:         3,
		},
		Features: FeatureConfig{
			AutoReconnect:          true,
			SensorPollIntervalMs:   500,
			ProgressEmitIntervalMs: 200,
		},
		Security: SecurityConfig{
			APIKey:          "",
			RateLimitPerSec: 20,
			MaxWSClients:    16,
		},
		Files: FileConfig{
			UserSettings:    "user_settings.toml",
			LogFile:         "modbus-configurator.log",
			OrchestratorDir: "runtime/axiom",
		},
	}
}

// LoadConfig читает TOML-файл и накладывает поверх значений по умолчанию.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, fmt.Errorf("config file not found: %s (using defaults)", path)
		}
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}

	return cfg, nil
}

// Duration returns the Modbus timeout as time.Duration.
func (m ModbusConfig) Duration() time.Duration {
	return time.Duration(m.TimeoutMs) * time.Millisecond
}

// RetryDelay returns the retry delay as time.Duration.
func (m ModbusConfig) RetryDelay() time.Duration {
	return time.Duration(m.RetryDelayMs) * time.Millisecond
}
