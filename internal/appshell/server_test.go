package appshell

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAppURLsUseIPv4Loopback(t *testing.T) {
	if got := AppURL("37777"); got != "http://127.0.0.1:37777/music/" {
		t.Fatalf("AppURL() = %q", got)
	}
	if got := HealthURL(""); got != "http://127.0.0.1:37777/music/healthz" {
		t.Fatalf("HealthURL() = %q", got)
	}
}

func TestWaitForURL(t *testing.T) {
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-ready:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "not ready", http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()

	go func() {
		time.Sleep(50 * time.Millisecond)
		close(ready)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := WaitForURL(ctx, server.URL); err != nil {
		t.Fatalf("WaitForURL() error = %v", err)
	}
}

func TestStartupErrorDataURL(t *testing.T) {
	got := StartupErrorDataURL("failed <probe>", "http://127.0.0.1:37777/music/")
	if !strings.HasPrefix(got, "data:text/html;charset=utf-8,") {
		t.Fatalf("StartupErrorDataURL() should return a data URL, got %q", got)
	}
	if strings.Contains(got, "<probe>") {
		t.Fatalf("StartupErrorDataURL() must escape the message")
	}
}
