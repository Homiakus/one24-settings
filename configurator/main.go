package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"modbus-configurator/api"
	"modbus-configurator/modbus"
	"modbus-configurator/ws"
)

func main() {
	configPath := flag.String("config", "configurator.toml", "путь к TOML-конфигурации")
	flag.Parse()

	// Загрузка конфигурации
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Printf("[main] предупреждение: %v — используются значения по умолчанию", err)
		cfg = DefaultConfig()
	}

	hub := ws.NewHub()
	if cfg.Security.MaxWSClients > 0 {
		hub.SetMaxClients(cfg.Security.MaxWSClients)
	}

	// Modbus-клиент: пытаемся подключиться при старте
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
		log.Printf("[main] используйте POST /api/v1/connect для подключения")
	} else {
		mbClient = client
		log.Printf("[main] Modbus: %s @ %d baud, slave=%d", cfg.Modbus.Port, cfg.Modbus.Baudrate, cfg.Modbus.SlaveID)
	}

	srvCfg := &api.ServerConfig{
		RateLimitRPS: cfg.Security.RateLimitPerSec,
		APIKey:       cfg.Security.APIKey,
		MaxWSClients: cfg.Security.MaxWSClients,
	}

	srv, err := api.New(mbClient, hub, uiFS, srvCfg)
	if err != nil {
		log.Fatalf("[main] сервер: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      srv.Handler(),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[main] порт %s: %v", addr, err)
	}

	fmt.Printf("╔══════════════════════════════════════════════╗\n")
	fmt.Printf("║   Modbus Configurator — ONEPAP.24           ║\n")
	fmt.Printf("║   http://%s                           ║\n", addr)
	fmt.Printf("║   API:  /api/v1/                            ║\n")
	fmt.Printf("║   WS:   /ws/events                          ║\n")
	fmt.Printf("║   Конфиг: %-32s ║\n", *configPath)
	fmt.Printf("╚══════════════════════════════════════════════╝\n")

	go func() {
		log.Printf("[main] запущен %s", addr)
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[main] %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Printf("[main] остановка...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeoutSec)*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("[main] shutdown: %v", err)
	}
	if mbClient != nil {
		mbClient.Close()
	}
	log.Printf("[main] остановлен")
}
