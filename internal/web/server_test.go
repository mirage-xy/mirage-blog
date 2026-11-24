package web

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// startServe 起一个真实 listener 跑 Serve，测试结束取消 ctx 并等它退出。
func startServe(t *testing.T, s *Server) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Serve returned error: %v", err)
			}
		case <-time.After(shutdownTimeout + time.Second):
			t.Error("Serve did not return after cancel")
		}
	})
	return ln.Addr().String()
}

func waitHealthy(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s did not become healthy", addr)
}

func TestHealthz(t *testing.T) {
	s := New("127.0.0.1:0", testLogger())
	addr := startServe(t, s)

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status in body = %q, want ok", body.Status)
	}
}

func TestUnknownPathIsNotFound(t *testing.T) {
	s := New("127.0.0.1:0", testLogger())
	addr := startServe(t, s)

	resp, err := http.Get("http://" + addr + "/nope")
	if err != nil {
		t.Fatalf("GET /nope: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestHealthzRejectsPost(t *testing.T) {
	s := New("127.0.0.1:0", testLogger())
	addr := startServe(t, s)

	resp, err := http.Post("http://"+addr+"/healthz", "text/plain", nil)
	if err != nil {
		t.Fatalf("POST /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestServeGracefulShutdown(t *testing.T) {
	s := New("127.0.0.1:0", testLogger())
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()

	addr := ln.Addr().String()
	waitHealthy(t, addr)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(shutdownTimeout + time.Second):
		t.Fatal("Serve did not return after context cancel")
	}

	if conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
		conn.Close()
		t.Fatal("port still accepting connections after shutdown")
	}
}

func TestServeWaitsForInflightRequest(t *testing.T) {
	s := New("127.0.0.1:0", testLogger())
	started := make(chan struct{})
	release := make(chan struct{})
	s.mux.HandleFunc("GET /slow/", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()

	addr := ln.Addr().String()
	respCh := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/slow/")
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}()

	<-started
	cancel()
	close(release)

	select {
	case resp := <-respCh:
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("in-flight request status = %d, want 204", resp.StatusCode)
		}
	case err := <-errCh:
		t.Fatalf("in-flight request failed: %v", err)
	case <-time.After(shutdownTimeout):
		t.Fatal("in-flight request did not finish")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(shutdownTimeout + time.Second):
		t.Fatal("Serve did not return after in-flight request finished")
	}
}
