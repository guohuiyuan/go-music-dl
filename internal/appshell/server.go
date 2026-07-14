package appshell

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/guohuiyuan/go-music-dl/internal/web"
)

const (
	DefaultPort       = "37777"
	ReadyTimeout      = 15 * time.Second
	readyPollInterval = 200 * time.Millisecond
	requestTimeout    = 800 * time.Millisecond
	loopbackHost      = "127.0.0.1"
)

func AppURL(port string) string {
	return fmt.Sprintf("http://%s:%s%s/", loopbackHost, normalizePort(port), web.RoutePrefix)
}

func HealthURL(port string) string {
	return fmt.Sprintf("http://%s:%s%s/healthz", loopbackHost, normalizePort(port), web.RoutePrefix)
}

func StartDesktopServerAndWait(ctx context.Context, port string) (string, error) {
	go web.StartDesktop(normalizePort(port))
	target := AppURL(port)
	if err := WaitForServerReady(ctx, port); err != nil {
		return target, err
	}
	return target, nil
}

func WaitForServerReady(ctx context.Context, port string) error {
	return WaitForURL(ctx, HealthURL(port))
}

func WaitForURL(ctx context.Context, target string) error {
	client := &http.Client{Timeout: requestTimeout}
	var lastErr error

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("unexpected status %d from %s", resp.StatusCode, target)
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("%w; last probe error: %v", ctx.Err(), lastErr)
			}
			return ctx.Err()
		case <-time.After(readyPollInterval):
		}
	}
}

func StartupErrorDataURL(message, retryURL string) string {
	body := fmt.Sprintf(`<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>MusicDL startup error</title>
  <style>
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #f8fafc; color: #0f172a; }
    main { width: min(680px, calc(100vw - 32px)); }
    h1 { font-size: 24px; margin: 0 0 12px; }
    p { line-height: 1.6; margin: 8px 0; }
    a { display: inline-flex; margin-top: 16px; padding: 10px 14px; border-radius: 8px; background: #0f172a; color: white; text-decoration: none; }
    code { display: block; margin-top: 8px; padding: 10px; background: #e2e8f0; border-radius: 8px; overflow-wrap: anywhere; }
  </style>
</head>
<body>
  <main>
    <h1>MusicDL did not finish starting</h1>
    <p>The local web service was not ready before the startup timeout.</p>
    <code>%s</code>
    <a href="%s">Retry</a>
  </main>
</body>
</html>`, html.EscapeString(message), html.EscapeString(retryURL))
	return "data:text/html;charset=utf-8," + url.PathEscape(body)
}

func normalizePort(port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		return DefaultPort
	}
	return port
}
