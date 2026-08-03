package modbus

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	gmodbus "github.com/goburrow/modbus"
)

// Client — обёртка над goburrow/modbus для RTU.
type Client struct {
	mu      sync.Mutex
	handler *gmodbus.RTUClientHandler
	client  gmodbus.Client
	cfg     Config
	retry   RetryConfig
}

// RetryConfig — настройки retry для Modbus-операций.
type RetryConfig struct {
	MaxRetries int
	Delay      time.Duration
}

// DefaultRetryConfig — 3 попытки, 300 мс задержка.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{MaxRetries: 3, Delay: 300 * time.Millisecond}
}

// Config — параметры подключения Modbus RTU.
type Config struct {
	Port     string
	Baudrate int
	DataBits int
	StopBits int
	Parity   string
	SlaveID  int
	Timeout  time.Duration
}

// DefaultConfig возвращает конфигурацию по умолчанию.
func DefaultConfig() Config {
	return Config{
		Port:     "COM4",
		Baudrate: 115200,
		DataBits: 8,
		StopBits: 1,
		Parity:   "N",
		SlaveID:  1,
		Timeout:  500 * time.Millisecond,
	}
}

// NewClient создаёт и подключает Modbus RTU клиент.
func NewClient(cfg Config, retry RetryConfig) (*Client, error) {
	handler := gmodbus.NewRTUClientHandler(cfg.Port)
	handler.Timeout = cfg.Timeout
	handler.SlaveId = byte(cfg.SlaveID)
	handler.BaudRate = cfg.Baudrate
	handler.DataBits = cfg.DataBits
	handler.Parity = strings.ToUpper(strings.TrimSpace(cfg.Parity))
	handler.StopBits = cfg.StopBits

	handler.Logger = log.New(os.Stderr, "[modbus] ", log.LstdFlags|log.Lmicroseconds)

	if err := handler.Connect(); err != nil {
		return nil, fmt.Errorf("modbus connect %s: %w", cfg.Port, err)
	}

	return &Client{
		handler: handler,
		client:  gmodbus.NewClient(handler),
		cfg:     cfg,
		retry:   retry,
	}, nil
}

// Connected возвращает true если порт открыт и контроллер отвечает.
func (c *Client) Connected() bool {
	if c.handler == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.client.ReadHoldingRegisters(1, 1)
	return err == nil
}

// ReadRegisterFast — одиночное чтение без retry (для polling-циклов).
func (c *Client) ReadRegisterFast(addr uint16) (uint16, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	results, err := c.client.ReadHoldingRegisters(addr, 1)
	if err != nil {
		return 0, err
	}
	if len(results) < 2 {
		return 0, fmt.Errorf("short response for reg %d: got %d bytes", addr, len(results))
	}
	return uint16(results[0])<<8 | uint16(results[1]), nil
}

// WriteRegister записывает одиночный holding register (FC 06) с retry.
func (c *Client) WriteRegister(addr, value uint16) error {
	var lastErr error
	for attempt := 0; attempt <= c.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retry.Delay)
		}
		c.mu.Lock()
		_, err := c.client.WriteSingleRegister(addr, value)
		c.mu.Unlock()
		if err == nil {
			return nil
		}
		if isModbusException(err) {
			return fmt.Errorf("modbus exception on write reg %d: %w", addr, err)
		}
		lastErr = err
	}
	return fmt.Errorf("write reg %d failed after %d retries: %w", addr, c.retry.MaxRetries+1, lastErr)
}

// ReadRegister читает одиночный holding register (FC 03) с retry.
func (c *Client) ReadRegister(addr uint16) (uint16, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retry.Delay)
		}
		c.mu.Lock()
		results, err := c.client.ReadHoldingRegisters(addr, 1)
		c.mu.Unlock()
		if err != nil {
			if isModbusException(err) {
				return 0, fmt.Errorf("modbus exception on read reg %d: %w", addr, err)
			}
			lastErr = err
			continue
		}
		if len(results) < 2 {
			return 0, fmt.Errorf("short response for reg %d: got %d bytes, want 2", addr, len(results))
		}
		return uint16(results[0])<<8 | uint16(results[1]), nil
	}
	return 0, fmt.Errorf("read reg %d failed after %d retries: %w", addr, c.retry.MaxRetries+1, lastErr)
}

// ReadRegisters читает несколько holding registers подряд (FC 03) с retry.
func (c *Client) ReadRegisters(addr, count uint16) ([]uint16, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retry.Delay)
		}
		c.mu.Lock()
		results, err := c.client.ReadHoldingRegisters(addr, count)
		c.mu.Unlock()
		if err != nil {
			if isModbusException(err) {
				return nil, fmt.Errorf("modbus exception on read regs %d+%d: %w", addr, count, err)
			}
			lastErr = err
			continue
		}
		expected := int(count) * 2
		if len(results) < expected {
			return nil, fmt.Errorf("short response: regs %d+%d: got %d bytes, want %d", addr, count, len(results), expected)
		}
		values := make([]uint16, count)
		for i := uint16(0); i < count; i++ {
			values[i] = uint16(results[i*2])<<8 | uint16(results[i*2+1])
		}
		return values, nil
	}
	return nil, fmt.Errorf("read regs %d+%d failed after %d retries: %w", addr, count, c.retry.MaxRetries+1, lastErr)
}

// Close закрывает соединение.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handler != nil {
		return c.handler.Close()
	}
	return nil
}

// Recover переподключает порт после транспортной ошибки.
func (c *Client) Recover() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handler == nil {
		return nil
	}
	_ = c.handler.Close()
	if err := c.handler.Connect(); err != nil {
		return fmt.Errorf("reconnect: %w", err)
	}
	return nil
}

func isModbusException(err error) bool {
	if err == nil {
		return false
	}
	type exception interface{ ExceptionCode() uint8 }
	_, ok := err.(exception)
	return ok
}
