package app

import (
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStartPprofServerDisabled(t *testing.T) {
	t.Setenv("PPROF_ADDR", "")
	if stop := StartPprofServer(); stop == nil {
		t.Fatal("expected a stop func even when disabled")
	}
}

func TestStartPprofServerEnabled(t *testing.T) {
	t.Setenv("PPROF_MEMPROFILERATE", "1")
	original := runtime.MemProfileRate
	t.Cleanup(func() { runtime.MemProfileRate = original })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	t.Setenv("PPROF_ADDR", addr)
	stop := StartPprofServer()
	defer stop()

	url := "http://" + addr + "/debug/pprof/"
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	var resp *http.Response
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err = client.Get(url)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "heap") {
		t.Fatalf("pprof index page missing expected content: %s", body)
	}
}
