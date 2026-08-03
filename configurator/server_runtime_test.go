package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestListenHTTPUsesPreferredPort(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port, err := addressPort(probe.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = probe.Close()

	result, err := listenHTTP("127.0.0.1", port, false)
	if err != nil {
		t.Fatalf("listenHTTP: %v", err)
	}
	defer result.Listener.Close()

	actualPort, err := addressPort(result.BoundAddress)
	if err != nil {
		t.Fatal(err)
	}
	if actualPort != port {
		t.Fatalf("actual port = %d, want %d", actualPort, port)
	}
	if result.UsedFallback {
		t.Fatal("unexpected fallback")
	}
}

func TestListenHTTPFallsBackWhenPreferredPortIsBusy(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()

	occupiedPort, err := addressPort(occupied.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	result, err := listenHTTP("127.0.0.1", occupiedPort, true)
	if err != nil {
		t.Fatalf("listenHTTP: %v", err)
	}
	defer result.Listener.Close()

	actualPort, err := addressPort(result.BoundAddress)
	if err != nil {
		t.Fatal(err)
	}
	if actualPort == occupiedPort {
		t.Fatalf("fallback reused occupied port %d", occupiedPort)
	}
	if !result.UsedFallback {
		t.Fatal("expected fallback")
	}
	if result.OriginalError == nil {
		t.Fatal("expected original listen error")
	}
}

func TestListenHTTPStrictPortReturnsError(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()

	port, err := addressPort(occupied.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	_, err = listenHTTP("127.0.0.1", port, false)
	if err == nil {
		t.Fatal("expected strict-port error")
	}
}

func TestServerURLUsesLoopbackForWildcardAddress(t *testing.T) {
	if got := serverURL("0.0.0.0", 8080); got != "http://127.0.0.1:8080" {
		t.Fatalf("serverURL = %q", got)
	}
}

func TestExplainListenError(t *testing.T) {
	got := explainListenError(assertError("bind: An attempt was made to access a socket in a way forbidden by its access permissions."))
	if !strings.Contains(got, "excluded/reserved") {
		t.Fatalf("unexpected explanation: %q", got)
	}
}

func TestRuntimeInfoLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".runtime", "server.json")
	info := runtimeInfo{
		PID:       123,
		URL:       "http://127.0.0.1:8080",
		Address:   "127.0.0.1:8080",
		Host:      "127.0.0.1",
		Port:      8080,
		StartedAt: time.Now().UTC(),
	}

	if err := writeRuntimeInfo(path, info); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded runtimeInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.PID != info.PID || decoded.URL != info.URL {
		t.Fatalf("decoded runtime info = %+v", decoded)
	}

	removeRuntimeInfo(path, 999)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("runtime file removed by wrong PID: %v", err)
	}
	removeRuntimeInfo(path, info.PID)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("runtime file still exists: %v", err)
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }
