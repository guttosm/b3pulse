package main

import (
	"context"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

type dummyHandler struct{}

func (d dummyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func TestStartServerAndShutdown(t *testing.T) {
	srv := startServer(dummyHandler{}, "0") // random port
	if srv == nil {
		t.Fatalf("expected server")
	}

	// Give server a moment to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown quickly with short timeout and no-op cleanup
	_, cancel := context.WithCancel(context.Background())
	go func() {
		// trigger gracefulShutdown select by simulating signal via closing after a brief delay
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// We cannot send OS signals easily here; instead, directly call Shutdown to simulate graceful flow.
	// Verify it doesn't panic and completes.
	shutdownCtx, c := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer c()
	if err := srv.Shutdown(shutdownCtx); err != nil && err != http.ErrServerClosed {
		t.Fatalf("shutdown err: %v", err)
	}
}

func TestGracefulShutdown_SignalPath(t *testing.T) {
	// Use a server that responds immediately
	srv := startServer(dummyHandler{}, "0")

	cleaned := make(chan struct{}, 1)
	go func() {
		ctx := context.Background()
		gracefulShutdown(ctx, srv, func() { close(cleaned) })
	}()

	// Give the goroutine time to set up signal notifications
	time.Sleep(50 * time.Millisecond)

	// Send SIGTERM to current process
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGTERM)

	select {
	case <-cleaned:
		// success
	case <-time.After(2 * time.Second):
		t.Fatalf("cleanup not called after SIGTERM")
	}
}
