package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"modbus-configurator/internal/testutil"
)

// TestDefaultConfigVerifiesContract проверяет, что конфигурация по умолчанию содержит валидные порты и значения таймаутов.
func TestDefaultConfigVerifiesContract(t *testing.T) {
	// Цель теста: проверить контракт получения значений по умолчанию без внешнего TOML файла.
	cfg := DefaultConfig()

	// Проверяем сетевой порт и хост сервера
	testutil.AssertEqual(t, cfg.Server.Host, "127.0.0.1", "Проверка хоста по умолчанию")
	testutil.AssertEqual(t, cfg.Server.Port, 8080, "Проверка порта HTTP по умолчанию")

	// Проверяем параметры Modbus
	testutil.AssertEqual(t, cfg.Modbus.Port, "COM4", "Проверка COM-порта Modbus по умолчанию")
	testutil.AssertEqual(t, cfg.Modbus.Baudrate, 115200, "Проверка скорости 115200 бод")
	testutil.AssertEqual(t, cfg.Modbus.SlaveID, 1, "Проверка Slave ID = 1")
	testutil.AssertEqual(t, cfg.Modbus.Duration(), 500*time.Millisecond, "Проверка преобразования таймаута в time.Duration")
}

// TestLoadConfigWithValidFile проверяет загрузку валидного TOML файла из временного каталога.
func TestLoadConfigWithValidFile(t *testing.T) {
	// Создаём временный TOML-файл для изолированного выполнения теста
	tmpDir := t.TempDir()
	tomlPath := filepath.Join(tmpDir, "test_config.toml")

	tomlContent := `
[server]
host = "0.0.0.0"
port = 9090
read_timeout_sec = 10
shutdown_timeout_sec = 5

[modbus]
port = "COM5"
baudrate = 19200
databits = 8
stopbits = 1
parity = "E"
slave_id = 2
timeout_ms = 1000
retries = 5
retry_delay_ms = 100

[security]
rate_limit_per_sec = 50
api_key = "secret123"
max_ws_clients = 10
`
	// Записываем данные с явным освобождением ресурсов через T.Cleanup
	err := os.WriteFile(tomlPath, []byte(tomlContent), 0644)
	testutil.AssertNil(t, err, "Запись временного файлового конфига")

	// Читаем и валидируем загруженный конфиг
	cfg, err := LoadConfig(tomlPath)
	testutil.AssertNil(t, err, "Парсинг валидного TOML файла")
	testutil.AssertEqual(t, cfg.Server.Host, "0.0.0.0", "Загрузка переопределённого хоста")
	testutil.AssertEqual(t, cfg.Server.Port, 9090, "Загрузка переопределённого порта")
	testutil.AssertEqual(t, cfg.Modbus.Port, "COM5", "Загрузка переопределённого COM-порта")
	testutil.AssertEqual(t, cfg.Modbus.Parity, "E", "Загрузка четности E")
	testutil.AssertEqual(t, cfg.Security.APIKey, "secret123", "Загрузка API ключа")
}

// TestLoadConfigFileNotFound проверяет обработку отсутствующего файла конфигурации.
func TestLoadConfigFileNotFound(t *testing.T) {
	// Цель: убедиться, что при отсутствии файла возвращается явная ошибка, не вызывая panic.
	nonExistent := filepath.Join(t.TempDir(), "non_existent.toml")
	_, err := LoadConfig(nonExistent)
	testutil.AssertNotNil(t, err, "Ожидается ошибка при отсутствии файла")
}

// BenchmarkLoadConfig замеряет скорость парсинга TOML конфигурации.
func BenchmarkLoadConfig(b *testing.B) {
	tmpDir := b.TempDir()
	tomlPath := filepath.Join(tmpDir, "bench_config.toml")
	tomlContent := `
[server]
host = "127.0.0.1"
port = 8080
[modbus]
port = "COM4"
baudrate = 115200
[security]
rate_limit_per_sec = 100
`
	_ = os.WriteFile(tomlPath, []byte(tomlContent), 0644)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg, err := LoadConfig(tomlPath)
		if err != nil {
			b.Fatalf("Ошибка парсинга в бенчмарке: %v", err)
		}
		testutil.KeepInterface(cfg)
	}
}
