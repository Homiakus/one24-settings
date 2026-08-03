package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const fallbackPortAttempts = 32

type listenResult struct {
	Listener         net.Listener
	RequestedAddress string
	BoundAddress     string
	UsedFallback     bool
	OriginalError    error
}

type runtimeInfo struct {
	PID              int       `json:"pid"`
	URL              string    `json:"url"`
	Address          string    `json:"address"`
	Host             string    `json:"host"`
	Port             int       `json:"port"`
	RequestedAddress string    `json:"requested_address"`
	ConfigPath       string    `json:"config_path"`
	StartedAt        time.Time `json:"started_at"`
}

func listenHTTP(host string, preferredPort int, allowFallback bool) (listenResult, error) {
	if strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}
	if preferredPort < 0 || preferredPort > 65535 {
		return listenResult{}, fmt.Errorf("HTTP port must be in range 0..65535, got %d", preferredPort)
	}

	requestedAddress := net.JoinHostPort(host, strconv.Itoa(preferredPort))
	listener, err := net.Listen("tcp", requestedAddress)
	if err == nil {
		return listenResult{
			Listener:         listener,
			RequestedAddress: requestedAddress,
			BoundAddress:     listener.Addr().String(),
		}, nil
	}

	if !allowFallback || preferredPort == 0 {
		return listenResult{}, fmt.Errorf("listen %s: %w", requestedAddress, err)
	}

	firstError := err
	lastPort := min(preferredPort+fallbackPortAttempts, 65535)
	for port := preferredPort + 1; port <= lastPort; port++ {
		candidate := net.JoinHostPort(host, strconv.Itoa(port))
		listener, candidateErr := net.Listen("tcp", candidate)
		if candidateErr == nil {
			return listenResult{
				Listener:         listener,
				RequestedAddress: requestedAddress,
				BoundAddress:     listener.Addr().String(),
				UsedFallback:     true,
				OriginalError:    firstError,
			}, nil
		}
	}

	listener, dynamicErr := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if dynamicErr == nil {
		return listenResult{
			Listener:         listener,
			RequestedAddress: requestedAddress,
			BoundAddress:     listener.Addr().String(),
			UsedFallback:     true,
			OriginalError:    firstError,
		}, nil
	}

	return listenResult{}, errors.Join(
		fmt.Errorf("listen %s: %w", requestedAddress, firstError),
		fmt.Errorf("listen on automatic fallback port: %w", dynamicErr),
	)
}

func explainListenError(err error) string {
	if err == nil {
		return "unknown error"
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "forbidden by its access permissions"),
		strings.Contains(message, "permission denied"),
		strings.Contains(message, "access is denied"):
		return "Windows blocked this port or the port is in an excluded/reserved range"
	case strings.Contains(message, "address already in use"),
		strings.Contains(message, "only one usage of each socket address"):
		return "the port is already used by another process"
	default:
		return err.Error()
	}
}

func addressPort(address string) (int, error) {
	_, portText, err := net.SplitHostPort(address)
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return 0, fmt.Errorf("parse port %q: %w", portText, err)
	}
	return port, nil
}

func serverURL(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
}

func writeRuntimeInfo(path string, info runtimeInfo) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create runtime directory: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("encode runtime info: %w", err)
	}
	data = append(data, '\n')

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return fmt.Errorf("write runtime info: %w", err)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace runtime info: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("publish runtime info: %w", err)
	}
	return nil
}

func removeRuntimeInfo(path string, pid int) {
	if strings.TrimSpace(path) == "" {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var info runtimeInfo
	if err := json.Unmarshal(data, &info); err != nil || info.PID != pid {
		return
	}
	_ = os.Remove(path)
}
