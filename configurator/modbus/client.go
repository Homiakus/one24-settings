package modbus

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	gmodbus "github.com/goburrow/modbus"
)

// Client — обёртка над goburrow/modbus для RTU с гарантией однопоточного исполнения (Single-Worker).
type Client struct {
	handler   *gmodbus.RTUClientHandler
	client    gmodbus.Client
	cfg       Config
	retry     RetryConfig
	taskQueue chan *modbusTask
	closeChan chan struct{}
	closeOnce sync.Once
}

type reqKind int

const (
	kindReadReg reqKind = iota
	kindWriteReg
	kindReadRegs
	kindReadFast
	kindWriteFast
	kindConnected
	kindRecover
	kindExec
)

type taskResult struct {
	val uint16
	arr []uint16
	any any
	err error
}

type modbusTask struct {
	kind    reqKind
	addr    uint16
	value   uint16
	count   uint16
	execFn  func() (any, error)
	resChan chan taskResult
}

var taskPool = sync.Pool{
	New: func() any {
		return &modbusTask{
			resChan: make(chan taskResult, 1),
		}
	},
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

	c := &Client{
		handler:   handler,
		client:    gmodbus.NewClient(handler),
		cfg:       cfg,
		retry:     retry,
		taskQueue: make(chan *modbusTask, 128),
		closeChan: make(chan struct{}),
	}

	go c.workerLoop()
	return c, nil
}

const interFrameDelay = 5 * time.Millisecond

func (c *Client) workerLoop() {
	for {
		select {
		case <-c.closeChan:
			return
		case task, ok := <-c.taskQueue:
			if !ok {
				return
			}
			var res taskResult
			switch task.kind {
			case kindReadReg:
				res.val, res.err = c.doReadRegister(task.addr)
			case kindWriteReg:
				res.err = c.doWriteRegister(task.addr, task.value)
			case kindReadRegs:
				res.arr, res.err = c.doReadRegisters(task.addr, task.count)
			case kindReadFast:
				res.val, res.err = c.doReadRegisterFast(task.addr)
			case kindWriteFast:
				res.err = c.doWriteRegisterFast(task.addr, task.value)
			case kindConnected:
				res.any = c.doConnected()
			case kindRecover:
				res.err = c.doRecover()
			case kindExec:
				if task.execFn != nil {
					res.any, res.err = task.execFn()
				}
			}
			time.Sleep(interFrameDelay)
			task.resChan <- res
		}
	}
}

func (c *Client) dispatch(kind reqKind, addr, value, count uint16, fn func() (any, error)) taskResult {
	task := taskPool.Get().(*modbusTask)
	task.kind = kind
	task.addr = addr
	task.value = value
	task.count = count
	task.execFn = fn

	select {
	case <-c.closeChan:
		task.execFn = nil
		taskPool.Put(task)
		return taskResult{err: errors.New("modbus client closed")}
	case c.taskQueue <- task:
	}

	res := <-task.resChan
	task.execFn = nil
	taskPool.Put(task)
	return res
}

// Exec выполняет произвольную функцию обмена строго в единственном воркер-потоке Modbus.
func (c *Client) Exec(fn func() (any, error)) (any, error) {
	res := c.dispatch(kindExec, 0, 0, 0, fn)
	return res.any, res.err
}

// Connected возвращает true если порт открыт и контроллер отвечает.
func (c *Client) Connected() bool {
	if c.handler == nil {
		return false
	}
	res := c.dispatch(kindConnected, 0, 0, 0, nil)
	if res.any == nil {
		return false
	}
	return res.any.(bool)
}

func (c *Client) doConnected() bool {
	if c.handler == nil {
		return false
	}
	_, err := c.client.ReadHoldingRegisters(1, 1)
	return err == nil
}

// ReadRegisterFast — одиночное чтение без retry (для polling-циклов).
func (c *Client) ReadRegisterFast(addr uint16) (uint16, error) {
	res := c.dispatch(kindReadFast, addr, 0, 1, nil)
	return res.val, res.err
}

func (c *Client) doReadRegisterFast(addr uint16) (uint16, error) {
	results, err := c.client.ReadHoldingRegisters(addr, 1)
	if err != nil {
		return 0, err
	}
	if len(results) < 2 {
		return 0, fmt.Errorf("short response for reg %d: got %d bytes", addr, len(results))
	}
	return uint16(results[0])<<8 | uint16(results[1]), nil
}

// WriteRegisterFast — одиночная запись без retry (для команд сброса/EEPROM).
func (c *Client) WriteRegisterFast(addr, value uint16) error {
	res := c.dispatch(kindWriteFast, addr, value, 1, nil)
	return res.err
}

func (c *Client) doWriteRegisterFast(addr, value uint16) error {
	_, err := c.client.WriteSingleRegister(addr, value)
	return err
}

// WriteRegister записывает одиночный holding register (FC 06) с retry.
func (c *Client) WriteRegister(addr, value uint16) error {
	res := c.dispatch(kindWriteReg, addr, value, 1, nil)
	return res.err
}

func (c *Client) doWriteRegister(addr, value uint16) error {
	var lastErr error
	for attempt := 0; attempt <= c.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retry.Delay)
		}
		_, err := c.client.WriteSingleRegister(addr, value)
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
	res := c.dispatch(kindReadReg, addr, 0, 1, nil)
	return res.val, res.err
}

func (c *Client) doReadRegister(addr uint16) (uint16, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retry.Delay)
		}
		results, err := c.client.ReadHoldingRegisters(addr, 1)
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
	res := c.dispatch(kindReadRegs, addr, 0, count, nil)
	return res.arr, res.err
}

func (c *Client) doReadRegisters(addr, count uint16) ([]uint16, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retry.Delay)
		}
		results, err := c.client.ReadHoldingRegisters(addr, count)
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

// Close закрывает соединение и останавливает воркер.
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closeChan)
		if c.handler != nil {
			err = c.handler.Close()
		}
	})
	return err
}

// Recover переподключает порт после транспортной ошибки.
func (c *Client) Recover() error {
	res := c.dispatch(kindRecover, 0, 0, 0, nil)
	return res.err
}

func (c *Client) doRecover() error {
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
	var mbErr *gmodbus.ModbusError
	return errors.As(err, &mbErr)
}
