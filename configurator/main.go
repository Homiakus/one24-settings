package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"modbus-configurator/api"
	"modbus-configurator/modbus"
	"modbus-configurator/ws"
)

func main() {
	if err := run(); err != nil {
		log.Printf("[main] ошибка запуска: %v", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "configurator.toml", "путь к TOML-конфигурации")
	hostOverride := flag.String("host", "", "переопределить HTTP host из конфигурации")
	portOverride := flag.Int("port", -1, "переопределить HTTP port; 0 — выбрать свободный автоматически")
	strictPort := flag.Bool("strict-port", false, "не выбирать запасной порт, если запрошенный недоступен")
	runtimeFile := flag.String("runtime-file", "", "записать PID, URL и фактический порт в JSON-файл")
	flag.Parse()

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Printf("[main] предупреждение: %v — используются значения по умолчанию", err)
		cfg = DefaultConfig()
	}
	if *hostOverride != "" {
		cfg.Server.Host = *hostOverride
	}
	if *portOverride >= 0 {
		cfg.Server.Port = *portOverride
	}

	listenInfo, err := listenHTTP(cfg.Server.Host, cfg.Server.Port, !*strictPort)
	if err != nil {
		return fmt.Errorf("HTTP-адрес недоступен: %w; запустите onepap.ps1 doctor для диагностики портов", err)
	}
	defer listenInfo.Listener.Close()

	actualPort, err := addressPort(listenInfo.BoundAddress)
	if err != nil {
		return fmt.Errorf("определить фактический HTTP-порт: %w", err)
	}
	url := serverURL(cfg.Server.Host, actualPort)
	if listenInfo.UsedFallback {
		log.Printf(
			"[main] адрес %s недоступен: %s",
			listenInfo.RequestedAddress,
			explainListenError(listenInfo.OriginalError),
		)
		log.Printf("[main] автоматически выбран свободный адрес %s", listenInfo.BoundAddress)
	}

	hub := ws.NewHub()
	if cfg.Security.MaxWSClients > 0 {
		hub.SetMaxClients(cfg.Security.MaxWSClients)
	}

	var mbClient *modbus.Client
	mbCfg := modbus.Config{
		Port:     cfg.Modbus.Port,
		Baudrate: cfg.Modbus.Baudrate,
		DataBits: cfg.Modbus.DataBits,
		StopBits: cfg.Modbus.StopBits,
		Parity:   cfg.Modbus.Parity,
		SlaveID:  cfg.Modbus.SlaveID,
		Timeout:  cfg.Modbus.Duration(),
	}
	retry := modbus.RetryConfig{
		MaxRetries: cfg.Modbus.Retries,
		Delay:      cfg.Modbus.RetryDelay(),
	}

	client, err := modbus.NewClient(mbCfg, retry)
	if err != nil {
		log.Printf("[main] предупреждение: Modbus не подключён: %v", err)
		log.Printf("[main] интерфейс всё равно запущен; подключение доступно через UI или POST /api/v1/connect")
	} else {
		mbClient = client
		defer mbClient.Close()
		log.Printf("[main] Modbus: %s @ %d baud, slave=%d", cfg.Modbus.Port, cfg.Modbus.Baudrate, cfg.Modbus.SlaveID)
	}

	srvCfg := &api.ServerConfig{
		RateLimitRPS: cfg.Security.RateLimitPerSec,
		APIKey:       cfg.Security.APIKey,
		MaxWSClients: cfg.Security.MaxWSClients,
	}
	srv, err := api.New(mbClient, hub, uiFS, srvCfg)
	if err != nil {
		return fmt.Errorf("создать API-сервер: %w", err)
	}

	httpServer := &http.Server{
		Addr:         listenInfo.BoundAddress,
		Handler:      srv.Handler(),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSec) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	absoluteConfigPath, err := filepath.Abs(*configPath)
	if err != nil {
		absoluteConfigPath = *configPath
	}
	info := runtimeInfo{
		PID:              os.Getpid(),
		URL:              url,
		Address:          listenInfo.BoundAddress,
		Host:             cfg.Server.Host,
		Port:             actualPort,
		RequestedAddress: listenInfo.RequestedAddress,
		ConfigPath:       absoluteConfigPath,
		StartedAt:        time.Now().UTC(),
	}
	if err := writeRuntimeInfo(*runtimeFile, info); err != nil {
		return fmt.Errorf("записать runtime-файл: %w", err)
	}
	defer removeRuntimeInfo(*runtimeFile, os.Getpid())

	fmt.Println("ONEPAP.24 Modbus Configurator")
	fmt.Printf("URL:     %s\n", url)
	fmt.Printf("API:     %s/api/v1/\n", url)
	fmt.Printf("Events:  %s/ws/events\n", url)
	fmt.Printf("Config:  %s\n", absoluteConfigPath)
	if *runtimeFile != "" {
		fmt.Printf("Runtime: %s\n", *runtimeFile)
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("[main] запущен %s", listenInfo.BoundAddress)
		serveErr <- httpServer.Serve(listenInfo.Listener)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)

	select {
	case <-stop:
		log.Printf("[main] остановка...")
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP-сервер остановлен: %w", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(cfg.Server.ShutdownTimeoutSec)*time.Second,
	)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("корректная остановка HTTP-сервера: %w", err)
	}

	log.Printf("[main] остановлен")
	return nil
}
