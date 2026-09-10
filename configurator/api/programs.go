package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"modbus-configurator/modbus"
	"modbus-configurator/model"
	"modbus-configurator/orchestrator"
)

var programMu sync.Mutex

type programCtx struct {
	ctx    context.Context
	cancel context.CancelFunc
	gen    uint64 // поколение: защищает от ABA-гонки при перезапуске
}

type executionIDContextKey struct{}

func executionID(ctx context.Context) string {
	if id, ok := ctx.Value(executionIDContextKey{}).(string); ok {
		return id
	}
	return ""
}

var (
	activeProgram *programCtx
	programGenSeq uint64
)

func startProgram() (context.Context, context.CancelFunc, uint64, bool) {
	programMu.Lock()
	defer programMu.Unlock()
	if activeProgram != nil {
		return nil, nil, 0, false
	}
	ctx, cancel := context.WithCancel(context.Background())
	programGenSeq++
	ctx = context.WithValue(ctx, executionIDContextKey{}, "program-"+itoa(int(programGenSeq)))
	activeProgram = &programCtx{ctx: ctx, cancel: cancel, gen: programGenSeq}
	return ctx, cancel, programGenSeq, true
}

func stopProgram() {
	programMu.Lock()
	defer programMu.Unlock()
	if activeProgram != nil {
		activeProgram.cancel()
		activeProgram = nil
	}
}

// finishProgram снимает блокировку, только если программа всё ещё активна
// и поколение совпадает. Защищает от ABA-гонки: если старая горутина
// завершается после того, как уже запущена новая программа, она не отменит новую.
func finishProgram(gen uint64) {
	programMu.Lock()
	defer programMu.Unlock()
	if activeProgram != nil && activeProgram.gen == gen {
		activeProgram = nil
	}
}

func isProgramRunning() bool {
	programMu.Lock()
	defer programMu.Unlock()
	return activeProgram != nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// checkPLCReady проверяет готовность PLC (ready_status == 0) и пишет ошибку в w при неудаче.
func (s *Server) checkPLCReady(w http.ResponseWriter) bool {
	s.modbusMu.RLock()
	mb := s.modbus
	s.modbusMu.RUnlock()
	ready, err := mb.ReadRegister(1)
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "PLC не отвечает: "+err.Error())
		return false
	}
	if ready != 0 {
		jsonError(w, http.StatusConflict, "PLC занят (ready_status="+itoa(int(ready))+"). Дождитесь завершения операции или выполните сброс.")
		return false
	}
	return true
}

// ─── System Check ─────────────────────────────────────────────────────────────

func (s *Server) handleSystemCheck(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	if err := s.writeZoneToPLC(); err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось выбрать зону: "+err.Error())
		return
	}

	if !s.checkPLCReady(w) {
		return
	}

	ctx, cancel, gen, ok := startProgram()
	if !ok {
		jsonError(w, http.StatusConflict, "Программа уже выполняется")
		return
	}
	go func() {
		defer cancel()
		defer finishProgram(gen)
		defer recoverPanic("system-check")
		s.runSystemCheck(ctx)
	}()
	jsonOK(w, map[string]string{"status": "started", "program": "system_check"})
}

func (s *Server) runSystemCheck(ctx context.Context) {
	s.setProgram("system_check", 1, 4, "Калибровка клапана", "running")
	defer s.clearProgram()

	steps := []struct {
		cmd  uint16
		name string
	}{
		{modbus.CmdCalibrateValve, "Калибровка клапана"},
		{modbus.CmdCalibrateSel1, "Калибровка селектора 1"},
		{modbus.CmdZeroRotor, "Хомирование ротора"},
		{modbus.CmdDrainIntermediate, "Слив"},
	}

	for i, step := range steps {
		select {
		case <-ctx.Done():
			if step.cmd != modbus.CmdDrainIntermediate {
				s.drainOnAbort()
			}
			return
		default:
		}
		s.progressStep(i+1, step.name)
		s.addLog("INFO", "Проверка: "+step.name)

		if err := s.modbus.SendCommand(ctx, step.cmd, 300*time.Second); err != nil {
			s.addLog("ERROR", "Ошибка проверки: "+step.name+": "+err.Error())
			s.hub.Broadcast(model.WSEvent{
				Type: "error",
				Data: map[string]string{"code": "EXEC", "message": err.Error(), "context": step.name},
			})
			if step.cmd != modbus.CmdDrainIntermediate {
				s.drainOnAbort()
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	s.addLog("INFO", "Проверка системы: OK")
}

// ─── Load / Sedimentation ─────────────────────────────────────────────────────

func (s *Server) handleLoad(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	if err := s.writeZoneToPLC(); err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось выбрать зону: "+err.Error())
		return
	}

	if !s.checkPLCReady(w) {
		return
	}

	ctx, cancel, gen, ok := startProgram()
	if !ok {
		jsonError(w, http.StatusConflict, "Программа уже выполняется")
		return
	}
	go func() {
		defer cancel()
		defer finishProgram(gen)
		defer recoverPanic("load")
		s.runSimpleCmd(ctx, modbus.CmdLoadMaterial, "load", "Загрузка образцов")
	}()
	jsonOK(w, map[string]string{"status": "started", "program": "load"})
}

func (s *Server) handleSedimentation(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	if err := s.writeZoneToPLC(); err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось выбрать зону: "+err.Error())
		return
	}

	if !s.checkPLCReady(w) {
		return
	}

	ctx, cancel, gen, ok := startProgram()
	if !ok {
		jsonError(w, http.StatusConflict, "Программа уже выполняется")
		return
	}
	go func() {
		defer cancel()
		defer finishProgram(gen)
		defer recoverPanic("sedimentation")
		s.runSimpleCmd(ctx, modbus.CmdSedimentation, "sedimentation", "Осаждение")
	}()
	jsonOK(w, map[string]string{"status": "started", "program": "sedimentation"})
}

// ─── Stain Cycle ──────────────────────────────────────────────────────────────

func (s *Server) handleStainStart(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	if err := s.writeZoneToPLC(); err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось выбрать зону: "+err.Error())
		return
	}

	if !s.checkPLCReady(w) {
		return
	}

	ctx, cancel, gen, ok := startProgram()
	if !ok {
		jsonError(w, http.StatusConflict, "Программа уже выполняется")
		return
	}
	go func() {
		defer cancel()
		defer finishProgram(gen)
		defer recoverPanic("stain-cycle")
		s.runStainCycle(ctx)
	}()
	s.setProgram("stain", 1, 11, "Спирт 96% (фиксация)", "running")
	jsonOK(w, map[string]any{
		"status": "started", "program": "stain",
		"current_step": 1, "total_steps": 11,
	})
}

func (s *Server) runStainCycle(ctx context.Context) {
	defer s.clearProgram()
	defer func() {
		if ctx.Err() != nil {
			s.drainOnAbort()
		}
	}()

	s.state.mu.RLock()
	stepsCopy := make([]model.StepParams, len(s.state.Steps))
	copy(stepsCopy, s.state.Steps)
	s.state.mu.RUnlock()

	for i := 0; i < 11; i++ {
		// Ждать снятия паузы
		for s.isPaused() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(200 * time.Millisecond):
			}
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		step := stepsCopy[i]
		s.progressStep(i+1, step.Name)
		s.state.mu.Lock()
		s.state.Program.TotalSec = step.ExposureTime + 120
		s.state.mu.Unlock()
		s.emitProgress()

		s.addLog("INFO", "Шаг "+itoa(i+1)+"/11: "+step.Name)

		if err := s.modbus.WriteStepParams(step); err != nil {
			s.addLog("ERROR", "Параметры шага "+itoa(i+1)+": "+err.Error())
			return
		}

		if err := s.modbus.SendCommand(ctx, modbus.CmdStainCycle, time.Duration(step.ExposureTime+120)*time.Second); err != nil {
			if strings.Contains(err.Error(), "ERR_REAGENT_LOW") || strings.Contains(err.Error(), "0x09") {
				s.addLog("WARN", "Реагент закончился на шаге "+itoa(i+1)+": "+step.Name)
				s.state.mu.Lock()
				s.state.Program.Phase = "reagent_low"
				s.state.Program.ReagentLow = true
				s.state.Program.Message = "Закончился реагент: " + step.Name
				ch := make(chan bool, 1)
				s.state.reagentResume = ch
				s.state.mu.Unlock()
				s.emitProgress()
				s.hub.Broadcast(model.WSEvent{
					Type: "reagent_low",
					Data: map[string]any{"step": i + 1, "step_name": step.Name, "message": "Закончился реагент на шаге " + itoa(i+1) + ": " + step.Name},
				})
				var replace bool
				select {
				case <-ctx.Done():
					s.state.mu.Lock()
					s.state.Program.ReagentLow = false
					s.state.reagentResume = nil
					s.state.mu.Unlock()
					return
				case replace = <-ch:
				}
				s.state.mu.Lock()
				s.state.Program.ReagentLow = false
				s.state.reagentResume = nil
				s.state.mu.Unlock()
				if replace {
					s.addLog("INFO", "Реагент заменён, повтор шага "+itoa(i+1))
					i--
					continue
				}
				s.addLog("WARN", "Цикл отменён из-за реагента")
				s.drainOnAbort()
				return
			}
			s.addLog("ERROR", "Шаг "+itoa(i+1)+": "+err.Error())
			s.hub.Broadcast(model.WSEvent{
				Type: "error",
				Data: map[string]any{"code": "EXEC", "message": err.Error(), "step": i + 1, "step_name": step.Name},
			})
			return
		}
	}
	s.addLog("INFO", "Цикл окраски завершён (11/11)")
}

func (s *Server) handleStainPause(w http.ResponseWriter, r *http.Request) {
	s.state.mu.Lock()
	s.state.Program.Paused = true
	s.state.mu.Unlock()
	s.addLog("INFO", "Цикл окраски: пауза")
	jsonOK(w, map[string]string{"status": "paused"})
}

func (s *Server) handleStainResume(w http.ResponseWriter, r *http.Request) {
	s.state.mu.Lock()
	s.state.Program.Paused = false
	s.state.mu.Unlock()
	s.addLog("INFO", "Цикл окраски: продолжение")
	jsonOK(w, map[string]string{"status": "resumed"})
}

func (s *Server) handleStainStop(w http.ResponseWriter, r *http.Request) {
	s.state.mu.Lock()
	s.state.Program.Running = false
	s.state.Program.Paused = false
	s.state.mu.Unlock()
	stopProgram()
	s.addLog("INFO", "Цикл окраски: остановлен")
	s.drainOnAbort()
	jsonOK(w, map[string]string{"status": "stopped"})
}

// ─── Wash ─────────────────────────────────────────────────────────────────────

func (s *Server) handleWashStart(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	if err := s.writeZoneToPLC(); err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось выбрать зону: "+err.Error())
		return
	}

	if !s.checkPLCReady(w) {
		return
	}

	ctx, cancel, gen, ok := startProgram()
	if !ok {
		jsonError(w, http.StatusConflict, "Программа уже выполняется")
		return
	}
	go func() {
		defer cancel()
		defer finishProgram(gen)
		defer recoverPanic("wash")
		s.runWashCycle(ctx)
	}()
	s.setProgram("wash", 1, 4, "Хлорка (1)", "running")
	jsonOK(w, map[string]any{
		"status": "started", "program": "wash",
		"current_step": 1, "total_steps": 4,
	})
}

func (s *Server) runWashCycle(ctx context.Context) {
	defer s.clearProgram()

	s.rlockState()
	// Копируем экспозиции в локальную карту ДО снятия блокировки,
	// чтобы избежать гонки с handleReadAll/handleWriteAll.
	exposureByStep := make(map[int]int, len(s.state.Steps))
	for _, st := range s.state.Steps {
		if st.ExposureTime > 0 {
			exposureByStep[st.ID] = st.ExposureTime
		}
	}
	washSteps := []struct {
		stepID int
		name   string
		defSec int
	}{
		{12, "Хлорка (1)", 20},
		{13, "Хлорка (2)", 20},
		{14, "Спирт", 10},
		{15, "Вода", 10},
	}
	s.runlockState()

	for i, step := range washSteps {
		select {
		case <-ctx.Done():
			return
		default:
		}
		sec := step.defSec
		if e, ok := exposureByStep[step.stepID]; ok {
			sec = e
		}
		s.progressStep(i+1, step.name)
		s.state.mu.Lock()
		s.state.Program.TotalSec = sec + 60
		s.state.mu.Unlock()
		s.emitProgress()

		s.addLog("INFO", "Промывка "+itoa(i+1)+"/4: "+step.name)

		if err := s.modbus.SafeWriteRegister(modbus.RegCurrentStep, uint16(step.stepID)); err != nil {
			s.addLog("ERROR", "Ошибка промывки: "+err.Error())
			return
		}

		if err := s.modbus.SendCommand(ctx, modbus.CmdWashSystem, time.Duration(sec+60)*time.Second); err != nil {
			s.addLog("ERROR", "Ошибка промывки: "+err.Error())
			return
		}
	}
	s.addLog("INFO", "Промывка завершена (4/4)")
}

// ─── Full Cycle ───────────────────────────────────────────────────────────────

func (s *Server) handleFullStart(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}

	if err := s.writeZoneToPLC(); err != nil {
		jsonError(w, http.StatusInternalServerError, "Не удалось выбрать зону: "+err.Error())
		return
	}

	if !s.checkPLCReady(w) {
		return
	}

	ctx, cancel, gen, ok := startProgram()
	if !ok {
		jsonError(w, http.StatusConflict, "Программа уже выполняется")
		return
	}
	go func() {
		defer cancel()
		defer finishProgram(gen)
		defer recoverPanic("full-cycle")
		s.runFullCycle(ctx)
	}()
	jsonOK(w, map[string]string{"status": "started", "program": "full"})
}

func (s *Server) runFullCycle(ctx context.Context) {
	s.runSystemCheck(ctx)
	if ctx.Err() != nil {
		return
	}

	s.runSimpleCmdSync(ctx, modbus.CmdLoadMaterial, "Загрузка образцов")
	if ctx.Err() != nil {
		return
	}

	s.runSimpleCmdSync(ctx, modbus.CmdSedimentation, "Осаждение")
	if ctx.Err() != nil {
		return
	}

	s.runStainCycle(ctx)
	if ctx.Err() != nil {
		return
	}

	s.runWashCycle(ctx)
	s.addLog("INFO", "Полный цикл завершён")
}

// ─── Reagent Low handlers ─────────────────────────────────────────────────────

func (s *Server) handleReagentReplaced(w http.ResponseWriter, r *http.Request) {
	s.state.mu.RLock()
	ch := s.state.reagentResume
	s.state.mu.RUnlock()
	if ch == nil {
		jsonError(w, http.StatusConflict, "Нет активного ожидания реагента")
		return
	}
	select {
	case ch <- true:
	default:
		jsonError(w, http.StatusConflict, "Канал уже обработан")
		return
	}
	jsonOK(w, map[string]string{"status": "replaced"})
}

func (s *Server) handleReagentCancel(w http.ResponseWriter, r *http.Request) {
	s.state.mu.RLock()
	ch := s.state.reagentResume
	s.state.mu.RUnlock()
	if ch == nil {
		jsonError(w, http.StatusConflict, "Нет активного ожидания реагента")
		return
	}
	select {
	case ch <- false:
	default:
		jsonError(w, http.StatusConflict, "Канал уже обработан")
		return
	}
	jsonOK(w, map[string]string{"status": "cancelled"})
}

// ─── Reset PLC ────────────────────────────────────────────────────────────────

func (s *Server) handleResetPLC(w http.ResponseWriter, r *http.Request) {
	s.modbusMu.RLock()
	mb := s.modbus
	s.modbusMu.RUnlock()
	if mb == nil {
		jsonError(w, http.StatusServiceUnavailable, "Modbus не подключён")
		return
	}

	// Убедиться что плата свободна перед отправкой команды сброса.
	// Для экстренного сброса разрешаем небольшой таймаут.
	if ready, err := mb.ReadRegister(modbus.RegReadyStatus); err == nil && ready != 0 {
		s.addLog("WARN", "Сброс PLC: контроллер занят (ready_status="+itoa(int(ready))+"), отправляем сброс принудительно")
	}

	s.addLog("WARN", "Сброс PLC (запись 111 в регистр 1)")

	if err := mb.WriteRegisterFast(1, 111); err != nil {
		// При сбросе контроллер может не успеть ответить — это нормально
		s.addLog("WARN", "Сброс PLC: контроллер не ответил (ожидаемо при перезагрузке)")
	}

	// Шаг 1: подождать, пока плата уйдёт в перезагрузку
	// (reg 1 != 0 или ошибка чтения) — до 3 сек
	s.addLog("INFO", "Ждём начала перезагрузки платы...")
	for i := 0; i < 15; i++ {
		time.Sleep(200 * time.Millisecond)
		val, err := mb.ReadRegisterFast(modbus.RegReadyStatus)
		if err != nil || val != 0 {
			// Плата начала перезагрузку
			break
		}
	}

	// Очистить буфер COM-порта от возможных помех линии при перезагрузке MCU
	_ = mb.Recover()

	// Шаг 2: ждать, пока плата вернётся и reg 1 == 0 — до 30 сек
	s.addLog("INFO", "Ждём завершения перезагрузки (ready_status == 0)...")
	var ready uint16
	var err error
	recovered := false
	for i := 0; i < 150; i++ {
		time.Sleep(200 * time.Millisecond)
		ready, err = mb.ReadRegisterFast(modbus.RegReadyStatus)
		if err == nil && ready == 0 {
			recovered = true
			break
		}
	}

	if !recovered {
		s.addLog("WARN", "PLC не вернулся после сброса за 30 сек")
		jsonError(w, http.StatusGatewayTimeout, "PLC не вернулся после сброса (таймаут 30 сек)")
		return
	}

	s.addLog("INFO", "PLC успешно перезагружен: ready_status=0")
	jsonOK(w, map[string]any{
		"status":       "reset",
		"ready_status": ready,
	})
}

// ─── Emergency Stop ───────────────────────────────────────────────────────────

func (s *Server) handleEmergencyStop(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) {
		return
	}
	stopProgram()

	s.state.mu.Lock()
	s.state.Program.Running = false
	s.state.Program.Paused = false
	s.state.mu.Unlock()

	s.modbusMu.RLock()
	mb := s.modbus
	s.modbusMu.RUnlock()
	if err := mb.WriteRegisterFast(1, 111); err != nil {
		// При аварийном стопе контроллер может уйти в перезагрузку — ошибка ожидаема
		s.addLog("WARN", "Аварийный стоп: контроллер не ответил (ожидаемо)")
	}

	s.addLog("WARN", "АВАРИЙНЫЙ СТОП")
	s.hub.Broadcast(model.WSEvent{
		Type: "error",
		Data: map[string]string{"code": "EMERGENCY_STOP", "message": "Аварийный останов"},
	})
	jsonOK(w, map[string]string{"status": "emergency_stop"})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (s *Server) runSimpleCmd(ctx context.Context, cmd uint16, program, label string) {
	s.setProgram(program, 1, 1, label, "running")
	defer s.clearProgram()
	s.addLog("INFO", label)
	s.recordExecutionFact(orchestrator.JournalRecord{
		ExecutionID: executionID(ctx), NodeID: program, Attempt: 1,
		CommandIntent: "cmd:" + itoa(int(cmd)), State: "intent",
	})
	timeout := 300 * time.Second
	if cmd == modbus.CmdSedimentation {
		timeout = 600 * time.Second
	}
	if err := s.modbus.SendCommand(ctx, cmd, timeout); err != nil {
		s.recordExecutionFact(orchestrator.JournalRecord{
			ExecutionID: executionID(ctx), NodeID: program, Attempt: 1,
			CommandIntent: "cmd:" + itoa(int(cmd)), Outcome: err.Error(), State: "unknown",
		})
		s.addLog("ERROR", label+": "+err.Error())
		s.hub.Broadcast(model.WSEvent{
			Type: "error",
			Data: map[string]string{"code": "EXEC", "message": err.Error(), "context": label},
		})
		return
	}
	s.recordExecutionFact(orchestrator.JournalRecord{
		ExecutionID: executionID(ctx), NodeID: program, Attempt: 1,
		CommandIntent: "cmd:" + itoa(int(cmd)), Outcome: "completed", State: "completed",
	})
	s.addLog("INFO", label+": OK")
}

func (s *Server) runSimpleCmdSync(ctx context.Context, cmd uint16, label string) {
	if ctx.Err() != nil {
		return
	}
	s.addLog("INFO", label)
	timeout := 300 * time.Second
	if cmd == modbus.CmdSedimentation {
		timeout = 600 * time.Second
	}
	if err := s.modbus.SendCommand(ctx, cmd, timeout); err != nil {
		s.addLog("ERROR", label+": "+err.Error())
	}
}

func (s *Server) setProgram(name string, cur, total int, stepName, phase string) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	s.state.Program = model.ProgramState{
		Program:     name,
		Running:     true,
		Paused:      false,
		CurrentStep: cur,
		TotalSteps:  total,
		StepName:    stepName,
		Phase:       phase,
	}
}

func (s *Server) clearProgram() {
	s.state.mu.Lock()
	s.state.Program.Running = false
	s.state.Program.Paused = false
	s.state.mu.Unlock()
	s.emitProgress()
}

func (s *Server) progressStep(cur int, name string) {
	s.state.mu.Lock()
	s.state.Program.CurrentStep = cur
	s.state.Program.StepName = name
	s.state.mu.Unlock()
}

func (s *Server) isPaused() bool {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	return s.state.Program.Paused
}

func (s *Server) emitProgress() {
	s.state.mu.RLock()
	prog := s.state.Program
	s.state.mu.RUnlock()
	s.hub.Broadcast(model.WSEvent{Type: "progress", Data: prog})
}

// writeZoneToPLC записывает выбранную зону в регистр 46 перед запуском операции.
func (s *Server) writeZoneToPLC() error {
	s.state.mu.RLock()
	zone := s.state.Zone
	s.state.mu.RUnlock()
	s.modbusMu.RLock()
	mb := s.modbus
	s.modbusMu.RUnlock()
	if mb == nil {
		return nil // нет подключения — зона будет записана при connect
	}
	return mb.WriteZone(zone)
}

func (s *Server) drainOnAbort() {
	s.modbusMu.RLock()
	mb := s.modbus
	s.modbusMu.RUnlock()
	if mb == nil || !mb.Connected() {
		return
	}
	s.addLog("WARN", "Аварийный слив...")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	if err := mb.SendCommand(ctx, modbus.CmdDrainIntermediate, 300*time.Second); err != nil {
		s.addLog("ERROR", "Аварийный слив не удался: "+err.Error())
	}
}

// ─── Custom Sequence ──────────────────────────────────────────────────────────

func (s *Server) handleCustomSequence(w http.ResponseWriter, r *http.Request) {
	var req model.CustomSequenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Некорректный JSON последовательности: "+err.Error())
		return
	}
	if len(req.Steps) == 0 {
		jsonError(w, http.StatusBadRequest, "Последовательность команд не может быть пустой")
		return
	}

	if !s.checkModbus(w) {
		return
	}

	if !s.checkPLCReady(w) {
		return
	}

	ctx, cancel, gen, ok := startProgram()
	if !ok {
		jsonError(w, http.StatusConflict, "Программа уже выполняется")
		return
	}

	go func() {
		defer cancel()
		defer finishProgram(gen)
		defer recoverPanic("custom-sequence")
		s.runCustomSequence(ctx, req)
	}()

	jsonOK(w, map[string]string{"status": "started", "program": "custom_sequence"})
}

func (s *Server) runCustomSequence(ctx context.Context, req model.CustomSequenceRequest) {
	total := len(req.Steps)
	seqName := req.Name
	if seqName == "" {
		seqName = "Пользовательский сценарий"
	}
	s.setProgram("custom_sequence", 1, total, seqName, "running")
	defer s.clearProgram()

	for i, step := range req.Steps {
		select {
		case <-ctx.Done():
			s.drainOnAbort()
			return
		default:
		}

		stepName := step.Name
		if stepName == "" {
			stepName = "Команда " + itoa(int(step.Cmd))
		}
		s.progressStep(i+1, stepName)
		s.addLog("INFO", "Последовательность "+itoa(i+1)+"/"+itoa(total)+": "+stepName+" (cmd="+itoa(int(step.Cmd))+")")
		s.recordExecutionFact(orchestrator.JournalRecord{
			ExecutionID: executionID(ctx), NodeID: "step-" + itoa(i+1), Attempt: 1,
			CommandIntent: "cmd:" + itoa(int(step.Cmd)), State: "intent",
		})

		if step.Zone > 0 {
			s.modbusMu.RLock()
			mb := s.modbus
			s.modbusMu.RUnlock()
			if mb != nil {
				_ = mb.WriteZone(step.Zone)
			}
		}

		timeout := time.Duration(step.TimeoutSec) * time.Second
		if timeout <= 0 {
			timeout = 300 * time.Second
		}

		if err := s.modbus.SendCommand(ctx, step.Cmd, timeout); err != nil {
			s.recordExecutionFact(orchestrator.JournalRecord{
				ExecutionID: executionID(ctx), NodeID: "step-" + itoa(i+1), Attempt: 1,
				CommandIntent: "cmd:" + itoa(int(step.Cmd)), Outcome: err.Error(), State: "unknown",
			})
			s.addLog("ERROR", "Ошибка шага "+itoa(i+1)+" ("+stepName+"): "+err.Error())
			s.hub.Broadcast(model.WSEvent{
				Type: "error",
				Data: map[string]string{"code": "EXEC", "message": err.Error(), "context": stepName},
			})
			if step.Cmd != modbus.CmdDrainIntermediate {
				s.drainOnAbort()
			}
			return
		}
		s.recordExecutionFact(orchestrator.JournalRecord{
			ExecutionID: executionID(ctx), NodeID: "step-" + itoa(i+1), Attempt: 1,
			CommandIntent: "cmd:" + itoa(int(step.Cmd)), Outcome: "completed", State: "completed",
		})

		if step.DelaySec > 0 {
			s.addLog("INFO", "Задержка шага "+itoa(i+1)+": "+itoa(step.DelaySec)+" сек...")
			select {
			case <-ctx.Done():
				s.drainOnAbort()
				return
			case <-time.After(time.Duration(step.DelaySec) * time.Second):
			}
		}
	}
	s.addLog("INFO", seqName+": Успешно завершена")
}

func (s *Server) recordExecutionFact(record orchestrator.JournalRecord) {
	if s.cfg == nil || s.cfg.AppendExecutionFact == nil {
		return
	}
	if record.ExecutionID == "" {
		record.ExecutionID = "unscoped"
	}
	if err := s.cfg.AppendExecutionFact(record); err != nil {
		s.addLog("ERROR", "Не удалось записать execution journal: "+err.Error())
	}
}

func recoverPanic(program string) {
	if r := recover(); r != nil {
		log.Printf("[PANIC] program %s: %v", program, r)
	}
}

// itoa — простая конвертация int в string без fmt.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return sign + string(digits)
}
