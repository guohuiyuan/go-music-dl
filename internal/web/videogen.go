package web

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

type RenderSession struct {
	ID            string
	Dir           string
	AudioPath     string
	OutName       string
	OutPath       string
	Total         int
	Mutex         sync.Mutex
	FrameWriter   io.WriteCloser
	EncoderDone   chan error
	EncoderOutput *bytes.Buffer
	LastActive    time.Time
}

var (
	sessions = make(map[string]*RenderSession)
	sessMu   sync.Mutex
)

const (
	renderFrameMultipartMemory = 64 << 20
	renderSessionMaxIdle       = 30 * time.Minute
)

func CleanupOldFiles(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > maxAge {
			os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

func decodeBase64Frame(dataURI string) ([]byte, error) {
	if comma := strings.IndexByte(dataURI, ','); comma >= 0 {
		dataURI = dataURI[comma+1:]
	} else if len(dataURI) > 23 {
		dataURI = dataURI[23:]
	}
	return base64.StdEncoding.DecodeString(dataURI)
}

func copyMultipartFile(file *multipart.FileHeader, dst io.Writer) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	_, err = io.Copy(dst, src)
	return err
}

func startRenderEncoder(videoDir, sessionID, audioPath string) (string, string, io.WriteCloser, chan error, *bytes.Buffer, error) {
	absVideoDir, err := filepath.Abs(videoDir)
	if err != nil {
		return "", "", nil, nil, nil, err
	}
	if err := os.MkdirAll(absVideoDir, 0755); err != nil {
		return "", "", nil, nil, nil, err
	}

	outName := fmt.Sprintf("render_%s_%d.mp4", sessionID, time.Now().Unix())
	outPath := filepath.Join(absVideoDir, outName)
	output := &bytes.Buffer{}

	ffmpegPath, err := core.ResolveFFmpegPath()
	if err != nil {
		return "", "", nil, nil, nil, err
	}

	cmd := exec.Command(ffmpegPath,
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "image2pipe",
		"-framerate", "30",
		"-vcodec", "mjpeg",
		"-i", "pipe:0",
		"-i", audioPath,
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-c:a", "aac",
		"-b:a", "320k",
		"-pix_fmt", "yuv420p",
		"-shortest",
		outPath,
	)
	cmd.Stdout = output
	cmd.Stderr = output

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", "", nil, nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return "", "", nil, nil, nil, err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	return outName, outPath, stdin, done, output, nil
}

func writeFrameBatch(sess *RenderSession, frames int, startIdx int, write func(io.Writer, int) error) error {
	sess.Mutex.Lock()
	defer sess.Mutex.Unlock()

	if sess.FrameWriter == nil {
		return fmt.Errorf("render encoder is not ready")
	}
	if startIdx != sess.Total {
		return fmt.Errorf("frame batch out of order: got %d, want %d", startIdx, sess.Total)
	}

	for i := 0; i < frames; i++ {
		if err := write(sess.FrameWriter, i); err != nil {
			return err
		}
	}
	sess.Total += frames
	sess.LastActive = time.Now()
	return nil
}

func abortRenderSession(sess *RenderSession) {
	sess.Mutex.Lock()
	writer := sess.FrameWriter
	done := sess.EncoderDone
	outPath := sess.OutPath
	sess.FrameWriter = nil
	sess.Mutex.Unlock()

	if writer != nil {
		_ = writer.Close()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
	os.RemoveAll(sess.Dir)
	if outPath != "" {
		os.Remove(outPath)
	}
}

func CleanupOldRenderSessions(maxIdle time.Duration) {
	now := time.Now()
	var stale []*RenderSession

	sessMu.Lock()
	for id, sess := range sessions {
		sess.Mutex.Lock()
		lastActive := sess.LastActive
		sess.Mutex.Unlock()
		if !lastActive.IsZero() && now.Sub(lastActive) > maxIdle {
			delete(sessions, id)
			stale = append(stale, sess)
		}
	}
	sessMu.Unlock()

	for _, sess := range stale {
		abortRenderSession(sess)
	}
}

func RegisterVideogenRoutes(api *gin.RouterGroup, videoDir string) {
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			CleanupOldFiles(videoDir, 10*time.Minute)
			CleanupOldRenderSessions(renderSessionMaxIdle)
		}
	}()

	videoApi := api.Group("/videogen")
	videoApi.POST("/init", func(c *gin.Context) {
		var id, source string
		var hasCustomAudio bool

		if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
			id = c.PostForm("id")
			source = c.PostForm("source")
			hasCustomAudio = true
		} else {
			var req struct {
				ID     string `json:"id"`
				Source string `json:"source"`
			}
			if c.ShouldBindJSON(&req) != nil {
				c.JSON(400, gin.H{"error": "Args error"})
				return
			}
			id = req.ID
			source = req.Source
		}

		sanitizedID := strings.NewReplacer("|", "-", "/", "-", "\\", "-", ":", "-").Replace(id)
		sessionID := fmt.Sprintf("%s_%s_%d", source, sanitizedID, time.Now().Unix())
		tempDir, err := os.MkdirTemp("", "vg_render_"+sessionID+"_*")
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to create render session"})
			return
		}
		audioPath := filepath.Join(tempDir, "audio.mp3")

		var proxyAudioUrl string

		if hasCustomAudio {
			file, err := c.FormFile("audio_file")
			if err != nil {
				os.RemoveAll(tempDir)
				c.JSON(400, gin.H{"error": "Failed to receive custom audio"})
				return
			}
			if err := c.SaveUploadedFile(file, audioPath); err != nil {
				os.RemoveAll(tempDir)
				c.JSON(500, gin.H{"error": "Failed to save custom audio"})
				return
			}
			proxyAudioUrl = ""
		} else {
			settings := core.GetWebSettings()
			tempSong := &model.Song{ID: id, Source: source, Name: "render", Artist: "render"}
			result, err := core.SaveSongToFileWithTemplate(tempSong, tempDir, false, false, settings.DownloadFilenameTemplate)
			if err != nil {
				os.RemoveAll(tempDir)
				c.JSON(500, gin.H{"error": "Audio download failed: " + err.Error()})
				return
			}
			audioPath = result.SavedPath
			proxyAudioUrl = fmt.Sprintf("%s/download?id=%s&source=%s", RoutePrefix, url.QueryEscape(id), source)
		}

		outName, outPath, frameWriter, encoderDone, encoderOutput, err := startRenderEncoder(videoDir, sessionID, audioPath)
		if err != nil {
			os.RemoveAll(tempDir)
			c.JSON(500, gin.H{"error": "FFmpeg start failed: " + err.Error()})
			return
		}

		sess := &RenderSession{
			ID:            sessionID,
			Dir:           tempDir,
			AudioPath:     audioPath,
			OutName:       outName,
			OutPath:       outPath,
			FrameWriter:   frameWriter,
			EncoderDone:   encoderDone,
			EncoderOutput: encoderOutput,
			LastActive:    time.Now(),
		}

		sessMu.Lock()
		sessions[sessionID] = sess
		sessMu.Unlock()

		c.JSON(200, gin.H{"session_id": sessionID, "audio_url": proxyAudioUrl})
	})

	videoApi.POST("/frame", func(c *gin.Context) {
		if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
			if err := c.Request.ParseMultipartForm(renderFrameMultipartMemory); err != nil {
				c.JSON(400, gin.H{"error": "Bad multipart request"})
				return
			}
			defer func() {
				if c.Request.MultipartForm != nil {
					_ = c.Request.MultipartForm.RemoveAll()
				}
			}()

			sessionID := c.Request.FormValue("session_id")
			startIdx, err := strconv.Atoi(c.Request.FormValue("start_idx"))
			if sessionID == "" || err != nil {
				c.JSON(400, gin.H{"error": "Bad request"})
				return
			}

			form := c.Request.MultipartForm
			if form == nil || len(form.File["frames"]) == 0 {
				c.JSON(400, gin.H{"error": "No frames uploaded"})
				return
			}

			sessMu.Lock()
			sess, ok := sessions[sessionID]
			sessMu.Unlock()
			if !ok {
				c.JSON(404, gin.H{"error": "Session not found"})
				return
			}

			files := form.File["frames"]
			if err := writeFrameBatch(sess, len(files), startIdx, func(w io.Writer, i int) error {
				return copyMultipartFile(files[i], w)
			}); err != nil {
				c.JSON(500, gin.H{"error": "Failed to write frame stream: " + err.Error()})
				return
			}

			c.JSON(200, gin.H{"status": "ok", "received": len(files)})
			return
		}

		var req struct {
			SessionID string   `json:"session_id"`
			Frames    []string `json:"frames"`
			StartIdx  int      `json:"start_idx"`
		}
		if c.ShouldBindJSON(&req) != nil {
			c.JSON(400, gin.H{"error": "Bad request"})
			return
		}

		sessMu.Lock()
		sess, ok := sessions[req.SessionID]
		sessMu.Unlock()
		if !ok {
			c.JSON(404, gin.H{"error": "Session not found"})
			return
		}

		if err := writeFrameBatch(sess, len(req.Frames), req.StartIdx, func(w io.Writer, i int) error {
			data, err := decodeBase64Frame(req.Frames[i])
			if err != nil {
				return err
			}
			_, err = w.Write(data)
			return err
		}); err != nil {
			c.JSON(500, gin.H{"error": "Failed to write frame stream: " + err.Error()})
			return
		}

		c.JSON(200, gin.H{"status": "ok", "received": len(req.Frames)})
	})

	videoApi.POST("/finish", func(c *gin.Context) {
		var req struct {
			SessionID string `json:"session_id"`
			Name      string `json:"name"`
		}
		c.ShouldBindJSON(&req)

		sessMu.Lock()
		sess, ok := sessions[req.SessionID]
		delete(sessions, req.SessionID)
		sessMu.Unlock()

		if !ok {
			c.JSON(404, gin.H{"error": "Session not found"})
			return
		}

		sess.Mutex.Lock()
		writer := sess.FrameWriter
		done := sess.EncoderDone
		output := sess.EncoderOutput
		outPath := sess.OutPath
		outName := sess.OutName
		sess.FrameWriter = nil
		sess.Mutex.Unlock()

		if writer == nil || done == nil {
			abortRenderSession(sess)
			c.JSON(500, gin.H{"error": "Render encoder is not ready"})
			return
		}

		if err := writer.Close(); err != nil {
			abortRenderSession(sess)
			c.JSON(500, gin.H{"error": "Render stream close failed: " + err.Error()})
			return
		}

		err := <-done
		os.RemoveAll(sess.Dir)

		if err != nil {
			outputText := ""
			if output != nil {
				outputText = strings.TrimSpace(output.String())
			}
			if outputText != "" {
				fmt.Println("FFmpeg Error:", outputText)
			}
			os.Remove(outPath)
			if outputText != "" {
				c.JSON(500, gin.H{"error": "Render failed: " + err.Error() + ", output: " + outputText})
				return
			}
			c.JSON(500, gin.H{"error": "Render failed: " + err.Error()})
			return
		}

		c.JSON(200, gin.H{"url": "/videos/" + outName})
	})
}
