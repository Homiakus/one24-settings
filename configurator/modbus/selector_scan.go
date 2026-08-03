package modbus

import (
	"fmt"
	"time"

	"modbus-configurator/model"
)

// selectorScanTransport отделяет алгоритм обхода координат от физического
// Modbus-транспорта. Это позволяет проверять количество и порядок запросов.
type selectorScanTransport interface {
	waitReady(timeout time.Duration) error
	writeRegister(addr, value uint16) error
	readRegister(addr uint16) (uint16, error)
}

// lockedSelectorScanTransport выполняет серию запросов при уже захваченном
// Client.mu. Благодаря этому другие HTTP-запросы не могут вклиниться между
// выбором селектора, выбором отверстия и чтением координаты.
type lockedSelectorScanTransport struct {
	client *Client
}

func (t lockedSelectorScanTransport) waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	lastReady := uint16(0xFFFF)
	transportErrors := 0
	attempts := 0

	for time.Now().Before(deadline) {
		attempts++
		results, err := t.client.client.ReadHoldingRegisters(RegReadyStatus, 1)
		if err != nil {
			transportErrors++
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if len(results) < 2 {
			return fmt.Errorf("короткий ответ ready_status: %d байт", len(results))
		}

		lastReady = uint16(results[0])<<8 | uint16(results[1])
		if lastReady == 0 {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}

	if transportErrors > 0 && transportErrors >= attempts/2 {
		return fmt.Errorf(
			"нет ответа от контроллера (ошибок связи: %d/%d за %v)",
			transportErrors,
			attempts,
			timeout,
		)
	}
	if lastReady != 0xFFFF {
		return fmt.Errorf("контроллер занят (ready_status=%d, таймаут %v)", lastReady, timeout)
	}
	return fmt.Errorf("таймаут ожидания ready_status (%v)", timeout)
}

func (t lockedSelectorScanTransport) writeRegister(addr, value uint16) error {
	var lastErr error
	for attempt := 0; attempt <= t.client.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(t.client.retry.Delay)
		}
		_, err := t.client.client.WriteSingleRegister(addr, value)
		if err == nil {
			return nil
		}
		if isModbusException(err) {
			return fmt.Errorf("modbus exception on write reg %d: %w", addr, err)
		}
		lastErr = err
	}
	return fmt.Errorf(
		"write reg %d failed after %d retries: %w",
		addr,
		t.client.retry.MaxRetries+1,
		lastErr,
	)
}

func (t lockedSelectorScanTransport) readRegister(addr uint16) (uint16, error) {
	var lastErr error
	for attempt := 0; attempt <= t.client.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(t.client.retry.Delay)
		}
		results, err := t.client.client.ReadHoldingRegisters(addr, 1)
		if err != nil {
			if isModbusException(err) {
				return 0, fmt.Errorf("modbus exception on read reg %d: %w", addr, err)
			}
			lastErr = err
			continue
		}
		if len(results) < 2 {
			return 0, fmt.Errorf("короткий ответ регистра %d: %d байт", addr, len(results))
		}
		return uint16(results[0])<<8 | uint16(results[1]), nil
	}
	return 0, fmt.Errorf(
		"read reg %d failed after %d retries: %w",
		addr,
		t.client.retry.MaxRetries+1,
		lastErr,
	)
}

// readAllValvePositionsFast удерживает транспорт на время всего снимка.
// Старый алгоритм выполнял для каждой из 30 позиций:
// ready + selector target + hole + coordinate = около 120 запросов.
// Новый алгоритм выполняет ready один раз, target два раза и по два запроса
// на позицию: 63 запроса. Параллельное вмешательство исключено mutex-ом.
func (c *Client) readAllValvePositionsFast(progress ProgressFunc) ([]model.SelectorPosition, []model.SelectorPosition, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return scanSelectorPositions(lockedSelectorScanTransport{client: c}, progress)
}

func scanSelectorPositions(transport selectorScanTransport, progress ProgressFunc) ([]model.SelectorPosition, []model.SelectorPosition, error) {
	if err := transport.waitReady(10 * time.Second); err != nil {
		return nil, nil, fmt.Errorf("контроллер не готов к чтению координат: %w", err)
	}

	selectors := [2][]model.SelectorPosition{
		make([]model.SelectorPosition, 0, 15),
		make([]model.SelectorPosition, 0, 15),
	}

	const total = 30
	current := 0

	for selectorNum := 1; selectorNum <= 2; selectorNum++ {
		// Выбор селектора не меняется внутри его 15 отверстий, поэтому записываем
		// регистр 19 один раз вместо пятнадцати одинаковых записей.
		if err := transport.writeRegister(RegSelectorTarget, uint16(selectorNum)); err != nil {
			return nil, nil, fmt.Errorf("выбор селектора %d (рег 19): %w", selectorNum, err)
		}

		for hole := 0; hole <= 14; hole++ {
			if err := transport.writeRegister(RegSelectorHole, uint16(hole)); err != nil {
				return nil, nil, fmt.Errorf("селектор %d, отверстие %d: выбор отверстия: %w", selectorNum, hole, err)
			}

			coord, err := transport.readRegister(RegSelectorCoord)
			if err != nil {
				return nil, nil, fmt.Errorf("селектор %d, отверстие %d: чтение координаты: %w", selectorNum, hole, err)
			}

			name := ""
			if hole < len(DefaultSelectorHoleNames) {
				name = DefaultSelectorHoleNames[hole]
			}
			selectors[selectorNum-1] = append(selectors[selectorNum-1], model.SelectorPosition{
				Selector: selectorNum,
				Hole:     hole,
				Name:     name,
				Coord:    int(coord),
			})

			current++
			if progress != nil {
				progress(current, total, fmt.Sprintf("Чтение селектора %d: позиция %d/14", selectorNum, hole))
			}
		}
	}

	return selectors[0], selectors[1], nil
}

// writeAllValvePositionsFast записывает изменённые положения селекторов в атомарном батч-режиме под единым mutex.
// Позволяет сократить время записи 30 положений с ~60 секунд до ~1-2 секунд.
func (c *Client) writeAllValvePositionsFast(sel1, sel2 []model.SelectorPosition, progress ProgressFunc) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	transport := lockedSelectorScanTransport{client: c}

	for _, p := range sel1 {
		if err := validateSelectorPosition(p.Selector, p.Hole, p.Coord); err != nil {
			return 0, fmt.Errorf("предпроверка клапан 1 отв %d: %w", p.Hole, err)
		}
	}
	for _, p := range sel2 {
		if err := validateSelectorPosition(p.Selector, p.Hole, p.Coord); err != nil {
			return 0, fmt.Errorf("предпроверка клапан 2 отв %d: %w", p.Hole, err)
		}
	}

	total := len(sel1) + len(sel2)
	if total == 0 {
		return 0, nil
	}

	if err := transport.waitReady(10 * time.Second); err != nil {
		return 0, fmt.Errorf("контроллер не готов к записи координат: %w", err)
	}

	written := 0

	if len(sel1) > 0 {
		if err := transport.writeRegister(RegSelectorTarget, 1); err != nil {
			return 0, fmt.Errorf("выбор селектора 1 (рег 19): %w", err)
		}
		for _, p := range sel1 {
			if err := transport.writeHoleAndCoord(p.Hole, p.Coord); err != nil {
				return written, fmt.Errorf("клапан 1 отв %d: %w", p.Hole, err)
			}
			written++
			if progress != nil {
				progress(written, total, fmt.Sprintf("Запись Клапан 1, отв. %d/14", p.Hole))
			}
		}
	}

	if len(sel2) > 0 {
		if err := transport.writeRegister(RegSelectorTarget, 2); err != nil {
			return written, fmt.Errorf("выбор селектора 2 (рег 19): %w", err)
		}
		for _, p := range sel2 {
			if err := transport.writeHoleAndCoord(p.Hole, p.Coord); err != nil {
				return written, fmt.Errorf("клапан 2 отв %d: %w", p.Hole, err)
			}
			written++
			if progress != nil {
				progress(written, total, fmt.Sprintf("Запись Клапан 2, отв. %d/14", p.Hole))
			}
		}
	}

	// Отправка команды 222 в рег. 1 для сохранения всех настроек клапанов в EEPROM
	if err := transport.writeRegister(RegReadyStatus, CmdSaveEEPROM); err != nil {
		return written, fmt.Errorf("сохранение клапанов в EEPROM (рег 1 = 222): %w", err)
	}

	return written, nil
}

func (t lockedSelectorScanTransport) writeHoleAndCoord(hole, coord int) error {
	bytesPayload := []byte{
		byte(hole >> 8), byte(hole),
		byte(coord >> 8), byte(coord),
	}
	var lastErr error
	for attempt := 0; attempt <= t.client.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(t.client.retry.Delay)
		}
		_, err := t.client.client.WriteMultipleRegisters(RegSelectorHole, 2, bytesPayload)
		if err == nil {
			return nil
		}
		if isModbusException(err) {
			break
		}
		lastErr = err
	}

	if err := t.writeRegister(RegSelectorHole, uint16(hole)); err != nil {
		return fmt.Errorf("выбор отверстия %d: %w", hole, err)
	}
	if err := t.writeRegister(RegSelectorCoord, uint16(coord)); err != nil {
		return fmt.Errorf("запись координаты %d: %w", coord, err)
	}
	_ = lastErr
	return nil
}
