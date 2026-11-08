package web

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestInspectHTTPFailureClassifiesTransientAndPermanentResponses(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		retryable bool
		reason    string
	}{
		{name: "rate limited", status: http.StatusTooManyRequests, retryable: true, reason: "rate_limited"},
		{name: "forbidden", status: http.StatusForbidden, retryable: true, reason: "access_denied"},
		{name: "upstream unavailable", status: http.StatusServiceUnavailable, retryable: true, reason: "upstream_unavailable"},
		{name: "not found", status: http.StatusNotFound, retryable: false, reason: "not_found"},
		{name: "gone", status: http.StatusGone, retryable: false, reason: "not_found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := inspectHTTPFailure(tt.status, 0)
			if got := payload["retryable"]; got != tt.retryable {
				t.Fatalf("retryable = %#v, want %v", got, tt.retryable)
			}
			if got := payload["reason"]; got != tt.reason {
				t.Fatalf("reason = %#v, want %q", got, tt.reason)
			}
			if tt.retryable {
				if got := payload["status"]; got != "retryable" {
					t.Fatalf("status = %#v, want retryable", got)
				}
				if got, ok := payload["retry_after_ms"].(int64); !ok || got <= 0 {
					t.Fatalf("retry_after_ms = %#v, want positive int64", payload["retry_after_ms"])
				}
			}
		})
	}
}

func TestInspectResolutionFailureDefaultsUnknownErrorsToRetryable(t *testing.T) {
	payload := inspectResolutionFailure(errors.New("temporary network failure"))
	if got := payload["retryable"]; got != true {
		t.Fatalf("retryable = %#v, want true", got)
	}
	if got := payload["reason"]; got != "resolution_failed" {
		t.Fatalf("reason = %#v, want resolution_failed", got)
	}

	payload = inspectResolutionFailure(errors.New("song not found"))
	if got := payload["retryable"]; got != false {
		t.Fatalf("retryable = %#v, want false for a definitive missing song", got)
	}
	if got := payload["status"]; got != "invalid" {
		t.Fatalf("status = %#v, want invalid", got)
	}

	payload = inspectResolutionFailure(errors.New("upstream returned HTTP 429 too many requests"))
	if got := payload["reason"]; got != "rate_limited" {
		t.Fatalf("reason = %#v, want rate_limited", got)
	}
	if got, ok := payload["retry_after_ms"].(int64); !ok || got < int64(3*time.Second/time.Millisecond) {
		t.Fatalf("retry_after_ms = %#v, want at least 3s", payload["retry_after_ms"])
	}
}

func TestInspectResolutionFailureStopsRetryingDefinitiveUnavailableSongs(t *testing.T) {
	tests := []string{
		"no valid download url found or vip required",
		"download url not found (might be paid song)",
		"kugou song info unavailable, errcode=0",
		"kugou songinfo v2 requires cookie t and KugooID",
		"spotify track unavailable in the current region (HTTP 451)",
	}

	for _, message := range tests {
		t.Run(message, func(t *testing.T) {
			payload := inspectResolutionFailure(errors.New(message))
			if got := payload["retryable"]; got != false {
				t.Fatalf("retryable = %#v, want false for %q", got, message)
			}
			if got := payload["status"]; got != "invalid" {
				t.Fatalf("status = %#v, want invalid for %q", got, message)
			}
			if _, exists := payload["retry_after_ms"]; exists {
				t.Fatalf("retry_after_ms should be omitted for %q", message)
			}
		})
	}
}

func TestInspectRetryAfterDelayAcceptsSecondsAndDates(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Retry-After", "3")
	if got := inspectRetryAfterDelay(resp); got != 3*time.Second {
		t.Fatalf("seconds Retry-After = %s, want 3s", got)
	}

	retryAt := time.Now().Add(4 * time.Second).UTC().Format(http.TimeFormat)
	resp.Header.Set("Retry-After", retryAt)
	got := inspectRetryAfterDelay(resp)
	if got < 3*time.Second || got > 4*time.Second {
		t.Fatalf("date Retry-After = %s, want about 4s", got)
	}
}
