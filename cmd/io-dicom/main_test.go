package main

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

func TestStartTCPHealthListener_AcceptsConnections(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	listener, err := startTCPHealthListener(ctx, 0, &wg)
	if err != nil {
		t.Fatalf("startTCPHealthListener returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		cancel()
		wg.Wait()
	})

	conn, err := net.DialTimeout("tcp", listener.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to health listener: %v", err)
	}
	_ = conn.Close()
}

func TestStartTCPHealthListener_StopsAfterContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	listener, err := startTCPHealthListener(ctx, 0, &wg)
	if err != nil {
		t.Fatalf("startTCPHealthListener returned error: %v", err)
	}

	cancel()
	_ = listener.Close()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("health listener did not stop after cancel")
	}
}
