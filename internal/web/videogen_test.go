package web

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type bufferWriteCloser struct {
	bytes.Buffer
	closed bool
}

func (b *bufferWriteCloser) Close() error {
	b.closed = true
	return nil
}

func TestVideogenFrameMultipartUploadWritesOrderedFrameStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	videoDir := t.TempDir()
	renderDir := t.TempDir()
	sessionID := "multipart-session"
	stream := &bufferWriteCloser{}

	sessMu.Lock()
	sessions[sessionID] = &RenderSession{ID: sessionID, Dir: renderDir, Total: 7, FrameWriter: stream}
	sessMu.Unlock()
	t.Cleanup(func() {
		sessMu.Lock()
		delete(sessions, sessionID)
		sessMu.Unlock()
	})

	router := gin.New()
	RegisterVideogenRoutes(router.Group(RoutePrefix), videoDir)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("session_id", sessionID); err != nil {
		t.Fatalf("write session_id: %v", err)
	}
	if err := writer.WriteField("start_idx", "7"); err != nil {
		t.Fatalf("write start_idx: %v", err)
	}

	payloads := [][]byte{[]byte("first-frame"), []byte("second-frame")}
	for _, payload := range payloads {
		part, err := writer.CreateFormFile("frames", "frame.jpg")
		if err != nil {
			t.Fatalf("create frame part: %v", err)
		}
		if _, err := io.Copy(part, bytes.NewReader(payload)); err != nil {
			t.Fatalf("write frame part: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, RoutePrefix+"/videogen/frame", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /videogen/frame status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if got, want := stream.Bytes(), bytes.Join(payloads, nil); !bytes.Equal(got, want) {
		t.Fatalf("written frame stream = %q, want %q", got, want)
	}
	if stream.closed {
		t.Fatal("frame upload should not close the render stream")
	}

	sessMu.Lock()
	total := sessions[sessionID].Total
	sessMu.Unlock()
	if total != 7+len(payloads) {
		t.Fatalf("session total = %d, want %d", total, 7+len(payloads))
	}
}

func TestCleanupOldRenderSessionsClosesAbandonedEncoder(t *testing.T) {
	sessionID := "stale-session"
	renderDir := t.TempDir()
	outPath := filepath.Join(t.TempDir(), "stale.mp4")
	if err := os.WriteFile(outPath, []byte("partial"), 0644); err != nil {
		t.Fatalf("write output fixture: %v", err)
	}
	stream := &bufferWriteCloser{}
	done := make(chan error, 1)
	done <- nil

	sessMu.Lock()
	sessions[sessionID] = &RenderSession{
		ID:          sessionID,
		Dir:         renderDir,
		OutPath:     outPath,
		FrameWriter: stream,
		EncoderDone: done,
		LastActive:  time.Now().Add(-time.Hour),
	}
	sessMu.Unlock()
	t.Cleanup(func() {
		sessMu.Lock()
		delete(sessions, sessionID)
		sessMu.Unlock()
	})

	CleanupOldRenderSessions(time.Minute)

	sessMu.Lock()
	_, ok := sessions[sessionID]
	sessMu.Unlock()
	if ok {
		t.Fatal("stale render session should be removed")
	}
	if !stream.closed {
		t.Fatal("stale render stream should be closed")
	}
	if _, err := os.Stat(renderDir); !os.IsNotExist(err) {
		t.Fatalf("render temp dir should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("partial output should be removed, stat err=%v", err)
	}
}
