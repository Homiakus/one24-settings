package modbus

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"modbus-configurator/model"
)

// ─── Высокоуровневые операции ─────────────────────────────────────────────────

// WaitReady ждёт, пока контроллер освободится (ready_status == 0).
// Использует быстрое чтение без retry чтобы не терять бюджет таймаута на повторах.
func (c *Client) WaitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	lastReady := uint16(0xFFFF)
	transportErrors := 0
	consecutiveTransportErrors := 0
	attempts := 0

	for time.Now().Before(deadline) {
		attempts++
		ready, err := c.ReadRegisterFast(RegReadyStatus)
		if err != nil {
			transportErrors++
			consecutiveTransportErrors++
			if consecutiveTransportErrors >= 5 {
				return fmt.Errorf("потеря связи с контроллером (%d подряд ошибок чтения)", consecutiveTransportErrors)
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}
		consecutiveTransportErrors = 0
		lastReady = ready
		if ready == 0 {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Диагностика: почему таймаут
	if transportErrors > 0 && transportErrors >= attempts/2 {
		return fmt.Errorf("нет ответа от контроллера (ошибок связи: %d/%d попыток за %v)",
			transportErrors, attempts, timeout)
	}
	if lastReady != 0xFFFF {
		return fmt.Errorf("контроллер занят (ready_status=%d, опрошено %d раз за %v)",
			lastReady, attempts, timeout)
	}
	return fmt.Errorf("таймаут ожидания ready_status (> %v, попыток: %d)", timeout, attempts)
}

// SafeWriteRegister — запись с проверкой готовности и эхо-подтверждением.
func (c *Client) SafeWriteRegister(addr, value uint16) error {
	if err := c.WaitReady(60 * time.Second); err != nil {
		return fmt.Errorf("контроллер не готов: %w", err)
	}

	if err := c.WriteRegister(addr, value); err != nil {
		return fmt.Errorf("запись регистра %d: %w", addr, err)
	}

	actual, err := c.ReadRegister(addr)
	if err != nil {
		return fmt.Errorf("эхо-чтение регистра %d: %w", addr, err)
	}
	if actual != value {
		return fmt.Errorf("эхо не совпало: записано %d, прочитано %d", value, actual)
	}
	return nil
}

// validateStepParams проверяет диапазоны значений (шаг 0 — экспозиция образца, 1..11 — окраска, 12..15 — промывка).
func validateStepParams(stepNum, exposure, volume int) error {
	if stepNum < 0 || stepNum > 15 {
		return fmt.Errorf("недопустимый номер шага: %d (0..15)", stepNum)
	}
	minVal := 1
	if stepNum == 0 {
		minVal = 0
	}
	if exposure < minVal || exposure > 600 {
		return fmt.Errorf("время экспозиции %d вне диапазона %d..600 сек", exposure, minVal)
	}
	if volume < minVal || volume > 6000 {
		return fmt.Errorf("объём налива %d вне диапазона %d..6000 (мл×10)", volume, minVal)
	}
	return nil
}

// ReadStepParams читает параметры одного шага (0..15).
func (c *Client) ReadStepParams(stepNum int) (model.StepParams, error) {
	if stepNum < 0 || stepNum > 15 {
		return model.StepParams{}, fmt.Errorf("недопустимый номер шага: %d", stepNum)
	}

	if err := c.WaitReady(10 * time.Second); err != nil {
		return model.StepParams{}, err
	}

	if err := c.WriteRegister(RegCurrentStep, uint16(stepNum)); err != nil {
		return model.StepParams{}, fmt.Errorf("запись current_step: %w", err)
	}

	echo, err := c.ReadRegister(RegCurrentStep)
	if err != nil {
		return model.StepParams{}, fmt.Errorf("эхо current_step: %w", err)
	}
	if echo != uint16(stepNum) {
		return model.StepParams{}, fmt.Errorf("эхо current_step: ожид. %d, получ. %d", stepNum, echo)
	}

	exposure, err := c.ReadRegister(RegExposureTime)
	if err != nil {
		return model.StepParams{}, fmt.Errorf("чтение exposure_time: %w", err)
	}

	volume, err := c.ReadRegister(RegLiquidFillVolume)
	if err != nil {
		return model.StepParams{}, fmt.Errorf("чтение fill_volume: %w", err)
	}

	_, err = c.ReadRegister(RegReagentEmpty)
	if err != nil {
		return model.StepParams{}, fmt.Errorf("чтение reagent_empty: %w", err)
	}

	name := ""
	if stepNum >= 0 && stepNum < len(StainStepNames) {
		name = StainStepNames[stepNum]
	}

	return model.StepParams{
		ID:           stepNum,
		Name:         name,
		ExposureTime: int(exposure),
		FillVolume:   int(volume),
	}, nil
}

// WriteStepParams записывает время и объём для заданного шага.
// Алгоритм: сначала переключить шаг (рег. 37), потом время (рег. 38) и объём (рег. 39).
// Регистр 41 (delta) — независимо от шага.
func (c *Client) WriteStepParams(p model.StepParams) error {
	if err := validateStepParams(p.ID, p.ExposureTime, p.FillVolume); err != nil {
		return err
	}

	if err := c.WaitReady(5 * time.Second); err != nil {
		return fmt.Errorf("контроллер занят перед записью шага %d: %w", p.ID, err)
	}

	// 1. Переключить на нужный шаг (рег. 37)
	if err := c.WriteRegister(RegCurrentStep, uint16(p.ID)); err != nil {
		return fmt.Errorf("выбор шага %d (current_step): %w", p.ID, err)
	}
	if actual, err := c.ReadRegister(RegCurrentStep); err != nil {
		return fmt.Errorf("эхо current_step шага %d: %w", p.ID, err)
	} else if actual != uint16(p.ID) {
		return fmt.Errorf("эхо current_step шага %d: записано %d, прочитано %d", p.ID, p.ID, actual)
	}

	// 2. Записать время экспозиции (рег. 38)
	if err := c.WriteRegister(RegExposureTime, uint16(p.ExposureTime)); err != nil {
		return fmt.Errorf("запись exposure_time шага %d: %w", p.ID, err)
	}
	if actual, err := c.ReadRegister(RegExposureTime); err != nil {
		return fmt.Errorf("эхо exposure_time шага %d: %w", p.ID, err)
	} else if actual != uint16(p.ExposureTime) {
		return fmt.Errorf("эхо exposure_time шага %d: записано %d, прочитано %d", p.ID, p.ExposureTime, actual)
	}

	// 3. Записать объём налива (рег. 39)
	if err := c.WriteRegister(RegLiquidFillVolume, uint16(p.FillVolume)); err != nil {
		return fmt.Errorf("запись fill_volume шага %d: %w", p.ID, err)
	}
	if actual, err := c.ReadRegister(RegLiquidFillVolume); err != nil {
		return fmt.Errorf("эхо fill_volume шага %d: %w", p.ID, err)
	} else if actual != uint16(p.FillVolume) {
		return fmt.Errorf("эхо fill_volume шага %d: записано %d, прочитано %d", p.ID, p.FillVolume, actual)
	}

	return nil
}

// ReadAllSettings читает шаг 0 + все 11 шагов + 4 шага промывки (0..15) + detection с контроллера.
// Если progress != nil, вызывает после каждого шага: progress(current, total, label).
func (c *Client) ReadAllSettings(progress ProgressFunc) ([]model.StepParams, model.DetectionParams, error) {
	// Pre-check: контроллер на связи?
	if _, err := c.ReadRegister(RegReadyStatus); err != nil {
		return nil, model.DetectionParams{}, fmt.Errorf("контроллер не отвечает (reg 1): %w", err)
	}

	steps := make([]model.StepParams, 0, 16)

	for i := 0; i <= 15; i++ {
		if progress != nil {
			progress(i+1, 16, "Чтение шага "+strconv.Itoa(i))
		}
		step, err := c.ReadStepParams(i)
		if err != nil {
			return nil, model.DetectionParams{}, fmt.Errorf("шаг %d: %w", i, err)
		}
		steps = append(steps, step)
	}

	delta, err := c.ReadRegister(RegReagentEmptyDelta)
	if err != nil {
		return nil, model.DetectionParams{}, fmt.Errorf("чтение delta: %w", err)
	}

	empty, err := c.ReadRegister(RegReagentEmpty)
	if err != nil {
		return nil, model.DetectionParams{}, fmt.Errorf("чтение reagent_empty: %w", err)
	}

	detection := model.DetectionParams{
		ReagentEmpty:      empty == 1,
		ReagentEmptyDelta: int(delta),
	}

	return steps, detection, nil
}

// WriteAllSettings записывает ВСЕ 11 шагов + delta на контроллер в оптимизированном атомарном батч-режиме.
func (c *Client) WriteAllSettings(steps []model.StepParams, delta int, progress ProgressFunc) (int, error) {
	for _, step := range steps {
		if err := validateStepParams(step.ID, step.ExposureTime, step.FillVolume); err != nil {
			return 0, fmt.Errorf("предпроверка шага %d: %w", step.ID, err)
		}
	}

	res, err := c.Exec(func() (any, error) {
		transport := lockedSelectorScanTransport{client: c}
		if err := transport.waitReady(10 * time.Second); err != nil {
			return 0, fmt.Errorf("контроллер не готов к записи настроек: %w", err)
		}

		written := 0
		total := len(steps)
		if delta >= 0 && delta <= 255 {
			total++
		}

		for _, step := range steps {
			if progress != nil {
				progress(written+1, total, fmt.Sprintf("Запись шага %d/11", step.ID))
			}

			payload := [6]byte{
				byte(step.ID >> 8), byte(step.ID),
				byte(step.ExposureTime >> 8), byte(step.ExposureTime),
				byte(step.FillVolume >> 8), byte(step.FillVolume),
			}
			_, err := c.client.WriteMultipleRegisters(RegCurrentStep, 3, payload[:])
			if err != nil {
				if err := transport.writeRegister(RegCurrentStep, uint16(step.ID)); err != nil {
					return written, fmt.Errorf("шаг %d (current_step): %w", step.ID, err)
				}
				if err := transport.writeRegister(RegExposureTime, uint16(step.ExposureTime)); err != nil {
					return written, fmt.Errorf("шаг %d (exposure_time): %w", step.ID, err)
				}
				if err := transport.writeRegister(RegLiquidFillVolume, uint16(step.FillVolume)); err != nil {
					return written, fmt.Errorf("шаг %d (fill_volume): %w", step.ID, err)
				}
			}
			written++
		}

		if delta >= 0 && delta <= 255 {
			if progress != nil {
				progress(written+1, total, "Запись delta")
			}
			if err := transport.writeRegister(RegReagentEmptyDelta, uint16(delta)); err != nil {
				return written, fmt.Errorf("запись delta (рег. 41): %w", err)
			}
			written++
		}

		if _, err := transport.client.client.WriteSingleRegister(RegReadyStatus, CmdSaveEEPROM); err != nil {
			return written, fmt.Errorf("сохранение в EEPROM (рег 1 = 222): %w", err)
		}

		return written, nil
	})

	if err != nil {
		return 0, err
	}
	return res.(int), nil
}

// WriteZone записывает выбор зоны в регистр 46 (1=первая, 2=вторая, 3=обе).
func (c *Client) WriteZone(zone uint16) error {
	if zone < 1 || zone > 3 {
		return fmt.Errorf("недопустимая зона %d (допустимы 1, 2, 3)", zone)
	}
	return c.WriteRegister(RegZoneSelect, zone)
}

// SendCommand отправляет команду в рег. 9, ждёт завершения, проверяет ошибки.
// Принимает context для поддержки cancellation/timeout.
func (c *Client) SendCommand(ctx context.Context, cmd uint16, timeout time.Duration) error {
	if err := c.WaitReady(10 * time.Second); err != nil {
		return fmt.Errorf("контроллер занят перед командой %d: %w", cmd, err)
	}

	if err := c.WriteRegister(RegCmdTarget, cmd); err != nil {
		return fmt.Errorf("отправка команды %d: %w", cmd, err)
	}

	// Пауза 300мс, чтобы микроконтроллер успел взвести ready_status != 0 (занят)
	time.Sleep(300 * time.Millisecond)

	// Ожидание подтверждения старта: плата переходит в статус "занята"
	startWaitDeadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(startWaitDeadline) {
		ready, err := c.ReadRegisterFast(RegReadyStatus)
		if err == nil && ready != 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Ждать ready_status == 0 (окончание выполнения операции)
	deadline := time.Now().Add(timeout)
	completed := false
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return fmt.Errorf("команда %d прервана: %w", cmd, ctx.Err())
		default:
		}
		ready, err := c.ReadRegister(RegReadyStatus)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if ready == 0 {
			completed = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !completed {
		return fmt.Errorf("команда %d: таймаут ожидания (> %v)", cmd, timeout)
	}

	// Проверить ошибки
	errCode, err := c.ReadRegister(RegStatusError)
	if err != nil {
		return fmt.Errorf("чтение status_error после команды %d: %w", cmd, err)
	}
	if errCode != 0 {
		name := ErrorNames[errCode]
		if name == "" {
			name = fmt.Sprintf("0x%02X", errCode)
		}
		return fmt.Errorf("ошибка контроллера: %s (код %d)", name, errCode)
	}

	return nil
}

// SetSelector переключает селектор на заданное отверстие.
// Протокол: рег. 19 (RegSelectorTarget) = номер селектора (1/2),
//
//	рег. 20 (RegSelectorHole) = номер отверстия (1..14).
//
// selectorNum: 1 или 2; hole: 1..14.
func (c *Client) SetSelector(selectorNum, hole int) error {
	if selectorNum < 1 || selectorNum > 2 {
		return fmt.Errorf("недопустимый номер селектора: %d (1 или 2)", selectorNum)
	}
	if hole < 1 || hole > 14 {
		return fmt.Errorf("недопустимый номер отверстия: %d (1..14)", hole)
	}

	if err := c.WaitReady(30 * time.Second); err != nil {
		return fmt.Errorf("контроллер не готов: %w", err)
	}

	// 1. Записать номер селектора в рег. 19 (RegSelectorTarget)
	if err := c.WriteRegister(RegSelectorTarget, uint16(selectorNum)); err != nil {
		return fmt.Errorf("выбор селектора %d (рег 19): %w", selectorNum, err)
	}

	// 2. Записать номер отверстия в рег. 20 (RegSelectorHole)
	if err := c.WriteRegister(RegSelectorHole, uint16(hole)); err != nil {
		return fmt.Errorf("запись отверстия %d селектора %d (рег 20): %w", hole, selectorNum, err)
	}

	// 3. Эхо-проверка: убедиться, что номер отверстия записан
	actual, err := c.ReadRegister(RegSelectorHole)
	if err != nil {
		return fmt.Errorf("эхо-чтение отверстия селектора %d: %w", selectorNum, err)
	}
	if actual != uint16(hole) {
		return fmt.Errorf("эхо селектора %d: записано %d, прочитано %d", selectorNum, hole, actual)
	}

	// 4. Ожидание завершения перемещения
	if err := c.WaitReady(15 * time.Second); err != nil {
		return fmt.Errorf("селектор %d не завершил перемещение: %w", selectorNum, err)
	}

	// 5. Проверка ошибок контроллера
	errCode, err := c.ReadRegister(RegStatusError)
	if err != nil {
		return fmt.Errorf("чтение status_error после селектора %d: %w", selectorNum, err)
	}
	if errCode == ErrMultiplexer {
		return fmt.Errorf("селектор %d не достиг позиции (ERR_MULTIPLEXER)", selectorNum)
	}
	if errCode != 0 {
		name := ErrorNames[errCode]
		if name == "" {
			name = fmt.Sprintf("0x%02X", errCode)
		}
		return fmt.Errorf("ошибка после селектора %d: %s (код %d)", selectorNum, name, errCode)
	}

	return nil
}

// ReadSensorSnapshot читает сервисную карту (входы, температуры, вес).
func (c *Client) ReadSensorSnapshot() (model.SensorSnapshot, error) {
	var snap model.SensorSnapshot

	values, err := c.ReadRegisters(22, 13)
	if err != nil {
		return snap, fmt.Errorf("чтение входов 22..34: %w", err)
	}
	for i := 0; i < 13 && i < len(values); i++ {
		snap.Inputs[i] = values[i] == 1
	}

	mcuTemp, err := c.ReadRegister(6)
	if err == nil {
		snap.MCUTemp = float64(mcuTemp) / 10.0
	}

	ntcTemp, err := c.ReadRegister(7)
	if err == nil {
		snap.NTCTemp = float64(ntcTemp) / 10.0
	}

	weight, err := c.ReadRegister(8)
	if err == nil {
		snap.HX711Weight = int(weight)
	}

	return snap, nil
}

// ─── Настройки координат селекторов (рег. 19, 20, 21) ───────────────────────

// validateSelectorPosition проверяет границы параметров селектора (selectorNum 1..2, hole 0..14, coord 0..65535).
func validateSelectorPosition(selectorNum, hole, coord int) error {
	if selectorNum < 1 || selectorNum > 2 {
		return fmt.Errorf("недопустимый номер селектора: %d (1 или 2)", selectorNum)
	}
	if hole < 0 || hole > 14 {
		return fmt.Errorf("недопустимый номер отверстия: %d (0..14)", hole)
	}
	if coord < 0 || coord > 65535 {
		return fmt.Errorf("координата %d вне диапазона 0..65535", coord)
	}
	return nil
}

// ReadSelectorPositionParams читает координату одного отверстия селектора.
func (c *Client) ReadSelectorPositionParams(selectorNum, hole int) (model.SelectorPosition, error) {
	if selectorNum < 1 || selectorNum > 2 {
		return model.SelectorPosition{}, fmt.Errorf("недопустимый номер селектора: %d (1 или 2)", selectorNum)
	}
	if hole < 0 || hole > 14 {
		return model.SelectorPosition{}, fmt.Errorf("недопустимый номер отверстия: %d (0..14)", hole)
	}

	if err := c.WaitReady(10 * time.Second); err != nil {
		return model.SelectorPosition{}, err
	}

	// 1. Выбор селектора (рег. 19)
	if err := c.WriteRegister(RegSelectorTarget, uint16(selectorNum)); err != nil {
		return model.SelectorPosition{}, fmt.Errorf("выбор селектора (рег 19): %w", err)
	}

	// 2. Выбор отверстия (рег. 20)
	if err := c.WriteRegister(RegSelectorHole, uint16(hole)); err != nil {
		return model.SelectorPosition{}, fmt.Errorf("выбор отверстия (рег 20): %w", err)
	}

	// 3. Чтение координаты (рег. 21)
	coord, err := c.ReadRegister(RegSelectorCoord)
	if err != nil {
		return model.SelectorPosition{}, fmt.Errorf("чтение координата (рег 21): %w", err)
	}

	name := ""
	if hole < len(DefaultSelectorHoleNames) {
		name = DefaultSelectorHoleNames[hole]
	}

	return model.SelectorPosition{
		Selector: selectorNum,
		Hole:     hole,
		Name:     name,
		Coord:    int(coord),
	}, nil
}

// WriteSelectorPositionParams записывает координату одного отверстия селектора.
func (c *Client) WriteSelectorPositionParams(p model.SelectorPosition) error {
	if err := validateSelectorPosition(p.Selector, p.Hole, p.Coord); err != nil {
		return err
	}

	if err := c.WaitReady(5 * time.Second); err != nil {
		return fmt.Errorf("контроллер занят перед записью селектора %d отв %d: %w", p.Selector, p.Hole, err)
	}

	// 1. Выбор селектора (рег. 19)
	if err := c.WriteRegister(RegSelectorTarget, uint16(p.Selector)); err != nil {
		return fmt.Errorf("выбор селектора %d (рег 19): %w", p.Selector, err)
	}

	// 2. Выбор отверстия (рег. 20)
	if err := c.WriteRegister(RegSelectorHole, uint16(p.Hole)); err != nil {
		return fmt.Errorf("выбор отверстия %d (рег 20): %w", p.Hole, err)
	}

	// 3. Запись координаты (рег. 21)
	if err := c.WriteRegister(RegSelectorCoord, uint16(p.Coord)); err != nil {
		return fmt.Errorf("запись координаты %d (рег 21): %w", p.Coord, err)
	}

	// 4. Отправка команды 222 в рег. 1 для сохранения в EEPROM (без retry)
	if err := c.WriteRegisterFast(RegReadyStatus, CmdSaveEEPROM); err != nil {
		return fmt.Errorf("сохранение в EEPROM (рег 1 = 222): %w", err)
	}
	time.Sleep(200 * time.Millisecond)

	return nil
}

// ReadAllValvePositions читает все 15 положений для обоих селекторов
// оптимизированным атомарным проходом.
func (c *Client) ReadAllValvePositions(progress ProgressFunc) ([]model.SelectorPosition, []model.SelectorPosition, error) {
	return c.readAllValvePositionsFast(progress)
}

// WriteAllValvePositions записывает циклом все положения для селектора 1 и 2 в оптимизированном атомарном батч-режиме.
func (c *Client) WriteAllValvePositions(sel1, sel2 []model.SelectorPosition, progress ProgressFunc) (int, error) {
	return c.writeAllValvePositionsFast(sel1, sel2, progress)
}
