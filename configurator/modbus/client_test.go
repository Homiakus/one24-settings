package modbus

import (
	"errors"
	"testing"
	"time"

	"modbus-configurator/internal/testutil"
)

// mockExceptionStruct эмулирует ошибку Modbus Exception Code (например, 0x02 Illegal Data Address).
type mockExceptionStruct struct {
	code uint8
}

func (m mockExceptionStruct) Error() string {
	return "modbus exception"
}

func (m mockExceptionStruct) ExceptionCode() uint8 {
	return m.code
}

// TestIsModbusExceptionVerifiesTypeCheck проверяет распознавание ошибок Modbus.
func TestIsModbusExceptionVerifiesTypeCheck(t *testing.T) {
	// 1. Обычная стандартная ошибка Go
	stdErr := errors.New("connection reset by peer")
	testutil.AssertFalse(t, isModbusException(stdErr), "Обычная ошибка не должна быть распознана как exception")

	// 2. nil ошибка
	testutil.AssertFalse(t, isModbusException(nil), "nil ошибка не является exception")

	// 3. Кастомная Modbus Exception ошибка
	modbusErr := mockExceptionStruct{code: 2}
	testutil.AssertTrue(t, isModbusException(modbusErr), "Ошибки с интерфейсом ExceptionCode должны давать true")
}

// TestDefaultRetryConfigVerifiesDefaults проверяет значения конфигурации повторов по умолчанию.
func TestDefaultRetryConfigVerifiesDefaults(t *testing.T) {
	cfg := DefaultRetryConfig()
	testutil.AssertEqual(t, cfg.MaxRetries, 3, "Количество повторов по умолчанию = 3")
	testutil.AssertEqual(t, cfg.Delay, 300*time.Millisecond, "Задержка между повторами = 300 мс")
}

// TestDefaultConfigVerifiesModbusDefaults проверяет Modbus RTU параметры по умолчанию.
func TestDefaultConfigVerifiesModbusDefaults(t *testing.T) {
	cfg := DefaultConfig()
	testutil.AssertEqual(t, cfg.Port, "COM4", "Порт по умолчанию COM4")
	testutil.AssertEqual(t, cfg.Baudrate, 115200, "Скорость по умолчанию 115200")
	testutil.AssertEqual(t, cfg.SlaveID, 1, "Slave ID = 1")
	testutil.AssertEqual(t, cfg.DataBits, 8, "8 дата-бит")
	testutil.AssertEqual(t, cfg.StopBits, 1, "1 стоп-бит")
	testutil.AssertEqual(t, cfg.Parity, "N", "Четность N")
}

// TestNewClientFailsOnNonExistentPort проверяет обработку невозможности открыть COM-порт.
func TestNewClientFailsOnNonExistentPort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Port = "COM99999_NON_EXISTENT"
	retry := DefaultRetryConfig()

	client, err := NewClient(cfg, retry)
	testutil.AssertNotNil(t, err, "Подключение к несуществующему порту должно завершаться ошибкой")
	testutil.AssertNil(t, client, "Клиент не должен быть создан")
}
