package httpserver

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeShutsDownWhenContextIsCancelled(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	server := &http.Server{
		Handler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusOK)
		}),
	}
	readiness := &readiness{
		database: pinger{},
	}
	result := make(chan error, 1)
	t.Cleanup(func() {
		cancel()
		_ = server.Close()
		_ = listener.Close()
	})
	go func() {
		result <- serve(ctx, server, listener, readiness)
	}()

	client := &http.Client{Timeout: time.Second}
	response, err := client.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatalf("GET server: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close response: %v", err)
	}

	cancel()

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("serve returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serve did not stop")
	}
	if !readiness.isShuttingDown.Load() {
		t.Fatal("readiness was not disabled")
	}

}
