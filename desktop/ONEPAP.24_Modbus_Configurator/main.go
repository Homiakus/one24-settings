package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed backend/modbus-backend.exe
var backendBinary []byte

type runtimeInfo struct { URL string `json:"url"` }

func extractBackend() (string, error) {
	dir := filepath.Join(os.TempDir(), "onepap-24-wails")
	if err := os.MkdirAll(dir, 0o700); err != nil { return "", err }
	path := filepath.Join(dir, "modbus-backend.exe")
	if data, err := os.ReadFile(path); err == nil && string(data) == string(backendBinary) { return path, nil }
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, backendBinary, 0o700); err != nil { return "", err }
	if err := os.Rename(tmp, path); err != nil { return "", err }
	return path, nil
}

func waitBackend(file string, child *exec.Cmd) (string, error) {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if child.ProcessState != nil { return "", fmt.Errorf("backend exited: %v", child.ProcessState) }
		if data, err := os.ReadFile(file); err == nil {
			var info runtimeInfo
			if json.Unmarshal(data, &info) == nil && info.URL != "" {
				resp, requestErr := http.Get(info.URL + "/api/v1/status")
				if requestErr == nil { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close(); if resp.StatusCode == http.StatusOK { return info.URL, nil } }
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", fmt.Errorf("timeout waiting for backend")
}

func main() {
	configArg := flag.String("config", "", "path to configurator.toml")
	runtimeArg := flag.String("runtime-file", "", "runtime JSON path")
	hostArg := flag.String("host", "127.0.0.1", "backend host")
	portArg := flag.Int("port", 0, "backend port")
	serverOnly := flag.Bool("server-only", false, "run backend without a window")
	flag.Parse()
	backend, err := extractBackend()
	if err != nil { log.Fatal(err) }
	runtimeDir := filepath.Join(os.TempDir(), "onepap-24-wails-runtime")
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil { log.Fatal(err) }
	runtimeFile := *runtimeArg
	if runtimeFile == "" { runtimeFile = filepath.Join(runtimeDir, fmt.Sprintf("server-%d.json", os.Getpid())) }
	config := *configArg
	if config == "" { config = filepath.Join(filepath.Dir(os.Args[0]), "configurator.toml") }
	if _, err := os.Stat(config); err != nil { config = filepath.Join("configurator", "configurator.toml") }
	childArgs := []string{"-config", config, "-runtime-file", runtimeFile, "-host", *hostArg}
	if *portArg >= 0 { childArgs = append(childArgs, "-port", fmt.Sprint(*portArg)) }
	child := exec.Command(backend, childArgs...)
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	if err := child.Start(); err != nil { log.Fatal(err) }
	defer func() { _ = child.Process.Kill(); _ = child.Wait(); _ = os.Remove(runtimeFile) }()
	url, err := waitBackend(runtimeFile, child)
	if err != nil { log.Fatal(err) }
	if *serverOnly { if err := child.Wait(); err != nil { log.Fatal(err) }; return }
	app := application.New(application.Options{
		Name: "ONEPAP.24 Modbus Configurator", Description: "ONEPAP.24 Modbus RTU configurator",
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "ONEPAP.24 — Modbus Configurator", Width: 1360, Height: 860,
		MinWidth: 900, MinHeight: 600, URL: "about:blank",
	})
	window.SetURL(url)
	if err := app.Run(); err != nil { log.Fatal(err) }
}
