package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"modbus-configurator/model"
	"modbus-configurator/modbus"
)

var programMu sync.Mutex

type programCtx struct {
	ctx    context.Context
	cancel context.CancelFunc
}

var activeProgram *programCtx

func startProgram() (context.Context, context.CancelFunc, bool) {
	programMu.Lock()
	defer programMu.Unlock()
	if activeProgram != nil {
		return nil, nil, false
	}
	ctx, cancel := context.WithCancel(context.Background())
	activeProgram = &programCtx{ctx, cancel}
	return ctx, cancel, true
}

func stopProgram() {
	programMu.Lock()
	defer programMu.Unlock()
	if activeProgram != nil {
		activeProgram.cancel()
		activeProgram = nil
	}
}

func isProgramRunning() bool {
	programMu.Lock()
	defer programMu.Unlock()
	return activeProgram != nil
}

// ─── System Check ─────────────────────────────────────────────────────────────

func (s *Server) handleSystemCheck(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) { return }

	// Проверить, что PLC не занят перед запуском
	ready, err := s.modbus.ReadRegister(1)
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "PLC не отвечает: "+err.Error())
		return
	}
	if ready != 0 {
		jsonError(w, http.StatusConflict, "PLC занят (ready_status="+itoa(int(ready))+"). Дождитесь завершения операции или выполните сброс.")
		return
	}

	ctx, cancel, ok := startProgram()
	if !ok {
		jsonError(w, http.StatusConflict, "Программа уже выполняется")
		return
	}
	go func() {
		defer cancel()
		defer stopProgram()
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
			s.drainOnAbort()
			return
		default:
		}
		s.progressStep(i+1, step.name)
		s.addLog("INFO", "Проверка: "+step.name)

		if err := s.modbus.SendCommand(ctx, step.cmd, 120*time.Second); err != nil {
			s.addLog("ERROR", "Ошибка проверки: "+step.name+": "+err.Error())
			s.hub.Broadcast(model.WSEvent{
				Type: "error",
				Data: map[string]string{"code": "EXEC", "message": err.Error(), "context": step.name},
			})
			s.drainOnAbort()
			return
		}
	}
	s.addLog("INFO", "Проверка системы: OK")
}

// ─── Load / Sedimentation ─────────────────────────────────────────────────────

func (s *Server) handleLoad(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) { return }
	ctx, cancel, ok := startProgram()
	if !ok {
		jsonError(w, http.StatusConflict, "Программа уже выполняется")
		return
	}
	go func() {
		defer cancel()
		defer stopProgram()
		defer recoverPanic("load")
		s.runSimpleCmd(ctx, modbus.CmdLoadMaterial, "load", "Загрузка образцов")
	}()
	jsonOK(w, map[string]string{"status": "started", "program": "load"})
}

func (s *Server) handleSedimentation(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) { return }
	ctx, cancel, ok := startProgram()
	if !ok {
		jsonError(w, http.StatusConflict, "Программа уже выполняется")
		return
	}
	go func() {
		defer cancel()
		defer stopProgram()
		defer recoverPanic("sedimentation")
		s.runSimpleCmd(ctx, modbus.CmdSedimentation, "sedimentation", "Осаждение")
	}()
	jsonOK(w, map[string]string{"status": "started", "program": "sedimentation"})
}

// ─── Stain Cycle ──────────────────────────────────────────────────────────────

func (s *Server) handleStainStart(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) { return }
	ctx, cancel, ok := startProgram()
	if !ok {
		jsonError(w, http.StatusConflict, "Программа уже выполняется")
		return
	}
	go func() {
		defer cancel()
		defer stopProgram()
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
		s.state.Program.TotalSec = step.ExposureTime + 120
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
				replace := <-ch
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
	if !s.checkModbus(w) { return }
	ctx, cancel, ok := startProgram()
	if !ok {
		jsonError(w, http.StatusConflict, "Программа уже выполняется")
		return
	}
	go func() {
		defer cancel()
		defer stopProgram()
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

	liquids := []struct {
		code uint16
		name string
		sec  int
	}{
		{18529, "Хлорка (1)", 20},
		{18529, "Хлорка (2)", 20},
		{4744, "Вода (1)", 10},
		{4744, "Вода (2)", 10},
	}

	for i, liq := range liquids {
		select {
		case <-ctx.Done():
			return
		default:
		}
		s.progressStep(i+1, liq.name)
		s.state.Program.TotalSec = liq.sec + 60
		s.emitProgress()

		s.addLog("INFO", "Промывка "+itoa(i+1)+"/4: "+liq.name)

		if err := s.modbus.SafeWriteRegister(modbus.RegCurrentStep, liq.code); err != nil {
			s.addLog("ERROR", "Ошибка промывки: "+err.Error())
			return
		}

		if err := s.modbus.SendCommand(ctx, modbus.CmdWashSystem, time.Duration(liq.sec+60)*time.Second); err != nil {
			s.addLog("ERROR", "Ошибка промывки: "+err.Error())
			return
		}
	}
	s.addLog("INFO", "Промывка завершена (4/4)")
}

// ─── Full Cycle ───────────────────────────────────────────────────────────────

func (s *Server) handleFullStart(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) { return }
	ctx, cancel, ok := startProgram()
	if !ok {
		jsonError(w, http.StatusConflict, "Программа уже выполняется")
		return
	}
	go func() {
		defer cancel()
		defer stopProgram()
		defer recoverPanic("full-cycle")
		s.runFullCycle(ctx)
	}()
	jsonOK(w, map[string]string{"status": "started", "program": "full"})
}

func (s *Server) runFullCycle(ctx context.Context) {
	s.runSystemCheck(ctx)
	if ctx.Err() != nil { return }

	s.runSimpleCmdSync(ctx, modbus.CmdLoadMaterial, "Загрузка образцов")
	if ctx.Err() != nil { return }

	s.runSimpleCmdSync(ctx, modbus.CmdSedimentation, "Осаждение")
	if ctx.Err() != nil { return }

	s.runStainCycle(ctx)
	if ctx.Err() != nil { return }

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
	ch <- true
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
	ch <- false
	jsonOK(w, map[string]string{"status": "cancelled"})
}

// ─── Reset PLC ────────────────────────────────────────────────────────────────

func (s *Server) handleResetPLC(w http.ResponseWriter, r *http.Request) {
	if s.modbus == nil {
		jsonError(w, http.StatusServiceUnavailable, "Modbus не подключён")
		return
	}

	s.addLog("WARN", "Сброс PLC (запись 111 в регистр 1)")

	if err := s.modbus.WriteRegister(1, 111); err != nil {
		s.addLog("ERROR", "Сброс PLC: "+err.Error())
		jsonError(w, http.StatusInternalServerError, "Сброс PLC не удался: "+err.Error())
		return
	}

	// Ждать перезагрузки
	time.Sleep(2 * time.Second)

	// Проверить, что PLC вернулся
	ready, err := s.modbus.ReadRegister(1)
	if err != nil {
		s.addLog("WARN", "PLC не отвечает после сброса: "+err.Error())
	} else {
		s.addLog("INFO", "PLC после сброса: ready_status="+itoa(int(ready)))
	}

	jsonOK(w, map[string]any{
		"status":       "reset",
		"ready_status": ready,
	})
}

// ─── Emergency Stop ───────────────────────────────────────────────────────────

func (s *Server) handleEmergencyStop(w http.ResponseWriter, r *http.Request) {
	if !s.checkModbus(w) { return }
	stopProgram()

	s.state.mu.Lock()
	s.state.Program.Running = false
	s.state.Program.Paused = false
	s.state.mu.Unlock()

	if err := s.modbus.WriteRegister(1, 111); err != nil {
		s.addLog("ERROR", "Аварийный стоп: "+err.Error())
		jsonError(w, http.StatusInternalServerError, "Аварийный стоп: "+err.Error())
		return
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
	if err := s.modbus.SendCommand(ctx, cmd, 120*time.Second); err != nil {
		s.addLog("ERROR", label+": "+err.Error())
		s.hub.Broadcast(model.WSEvent{
			Type: "error",
			Data: map[string]string{"code": "EXEC", "message": err.Error(), "context": label},
		})
		return
	}
	s.addLog("INFO", label+": OK")
}

func (s *Server) runSimpleCmdSync(ctx context.Context, cmd uint16, label string) {
	if ctx.Err() != nil { return }
	s.addLog("INFO", label)
	if err := s.modbus.SendCommand(ctx, cmd, 120*time.Second); err != nil {
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

func (s *Server) drainOnAbort() {
	if s.modbus == nil || !s.modbus.Connected() {
		return
	}
	s.addLog("WARN", "Аварийный слив...")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := s.modbus.SendCommand(ctx, modbus.CmdDrainIntermediate, 60*time.Second); err != nil {
		s.addLog("ERROR", "Аварийный слив не удался: "+err.Error())
	} else {
		s.addLog("INFO", "Аварийный слив: OK")
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
