package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
)

func TestDownloadQueueAPIPersistsAppendAndRejectsCrossOrigin(t *testing.T) {
	core.CloseConfigDB()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(t.TempDir(), "settings.db"))
	t.Cleanup(func() { core.CloseConfigDB() })
	q, err := core.OpenDownloadQueue()
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterDownloadQueueRoutes(r.Group("/music"), q)
	request := func(method, path string, body []byte, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://music.test/music/api/downloads/queue"+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	payload := []byte(`{"request_id":"one","songs":[{"id":"1","source":"kuwo","name":"One"},{"id":"2","source":"kuwo","name":"Two"}]}`)
	if w := request("POST", "", payload, "http://other.test"); w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "", payload, "http://music.test"); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "", payload, "http://music.test"); w.Code != 202 || !bytes.Contains(w.Body.Bytes(), []byte(`"added":0`)) {
		t.Fatal(w.Code, w.Body.String())
	}
	second := []byte(`{"request_id":"two","songs":[{"id":"3","source":"kuwo","name":"Three"}]}`)
	if w := request("POST", "", second, "http://music.test"); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := core.CloseConfigDB(); err != nil {
		t.Fatal(err)
	}
	q, err = core.OpenDownloadQueue()
	if err != nil {
		t.Fatal(err)
	}
	r = gin.New()
	RegisterDownloadQueueRoutes(r.Group("/music"), q)
	w := request("GET", "?status=active", nil, "http://music.test")
	var response struct {
		Total  int
		Tasks  []core.DownloadTask
		Counts map[string]int
	}
	if err = json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || response.Total != 3 || len(response.Tasks) != 3 || response.Counts[core.TaskWaiting] != 3 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "/1/cancel", nil, "http://music.test"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "/1/retry", nil, "http://music.test"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	invalid := []byte(`{"request_id":"invalid","songs":[{"id":"4","source":"kuwo"},{"source":"kuwo"}]}`)
	if w := request("POST", "", invalid, "http://music.test"); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	_, total, _, err := q.List(1, 50, "")
	if err != nil || total != 3 {
		t.Fatal(total, err)
	}
}
