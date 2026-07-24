package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

type searchResult struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Artist    string            `json:"artist"`
	Album     string            `json:"album"`
	AlbumID   string            `json:"album_id"`
	Duration  int               `json:"duration"`
	Size      int64             `json:"size"`
	Bitrate   int               `json:"bitrate"`
	Source    string            `json:"source"`
	URL       string            `json:"url"`
	Ext       string            `json:"ext"`
	Cover     string            `json:"cover"`
	Link      string            `json:"link"`
	Extra     map[string]string `json:"extra,omitempty"`
	IsInvalid bool              `json:"is_invalid,omitempty"`
	IsVIP     bool              `json:"is_vip,omitempty"`
}

type downloadRequest struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Artist     string            `json:"artist"`
	Album      string            `json:"album"`
	AlbumID    string            `json:"album_id"`
	Duration   int               `json:"duration"`
	Size       int64             `json:"size"`
	Bitrate    int               `json:"bitrate"`
	Source     string            `json:"source"`
	URL        string            `json:"url"`
	Ext        string            `json:"ext"`
	Cover      string            `json:"cover"`
	Link       string            `json:"link"`
	Extra      map[string]string `json:"extra,omitempty"`
	IsInvalid  bool              `json:"is_invalid,omitempty"`
	IsVIP      bool              `json:"is_vip,omitempty"`
	OutputDir  string            `json:"outputDir"`
}

type downloadResponse struct {
	Success  bool   `json:"success"`
	FilePath string `json:"filePath,omitempty"`
	FileName string `json:"fileName,omitempty"`
	Error    string `json:"error,omitempty"`
}

// 优先尝试下载的源顺序
var fallbackSources = []string{"kugou", "kuwo", "qq", "netease", "migu", "qianqian"}

func main() {
	core.CM.Load()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("can not listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	fmt.Printf("PORT:%d\n", port)
	os.Stdout.Sync()
	mux := http.NewServeMux()
	mux.HandleFunc("/search", handleSearch)
	mux.HandleFunc("/download", handleDownload)
	mux.HandleFunc("/validate", handleValidate)
	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
	}
	if err := server.Serve(listener); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	sourcesParam := strings.TrimSpace(r.URL.Query().Get("sources"))
	if keyword == "" {
		http.Error(w, "missing keyword", http.StatusBadRequest)
		return
	}
	if sourcesParam == "" {
		http.Error(w, "missing sources", http.StatusBadRequest)
		return
	}
	sources := strings.Split(sourcesParam, ",")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results []searchResult
	)
	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source == "" { continue }
		searchFn := core.GetSearchFunc(source)
		if searchFn == nil { continue }
		wg.Add(1)
		go func(src string, fn core.SearchFunc) {
			defer wg.Done()
			songs, err := fn(keyword)
			if err != nil { return }
			mu.Lock()
			for _, song := range songs {
				results = append(results, searchResult{
					ID: song.ID, Name: song.Name, Artist: song.Artist,
					Album: song.Album, AlbumID: song.AlbumID,
					Duration: song.Duration, Size: song.Size, Bitrate: song.Bitrate,
					Source: src, URL: song.URL, Ext: song.Ext,
					Cover: song.Cover, Link: song.Link,
					Extra: song.Extra, IsInvalid: song.IsInvalid, IsVIP: song.IsVIP,
				})
			}
			mu.Unlock()
		}(source, searchFn)
	}
	wg.Wait()
	sort.Slice(results, func(i, j int) bool {
		if results[i].Source != results[j].Source { return results[i].Source < results[j].Source }
		return results[i].Name < results[j].Name
	})

	json.NewEncoder(w).Encode(map[string]interface{}{
		"keyword": keyword, "sources": sources,
		"songs": results, "total": len(results),
	})
}

func tryDownload(song *model.Song, outputDir string) (*core.DownloadedSong, error) {
	result, err := core.SaveSongToFileWithTemplate(song, outputDir, true, true, "{artist} - {name}.{ext}")
	if err != nil {
		return nil, err
	}
	return result, nil
}

func searchOtherSources(name, artist string, excludeSource string) []*model.Song {
	keyword := name
	if artist != "" {
		keyword = name + " " + artist
	}
	var candidates []*model.Song
	for _, src := range fallbackSources {
		if src == excludeSource { continue }
		fn := core.GetSearchFunc(src)
		if fn == nil { continue }
		songs, err := fn(keyword)
		if err != nil { continue }
		for i := range songs {
			s := songs[i]
			candidates = append(candidates, &s)
		}
	}
	return candidates
}

func handleValidate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		json.NewEncoder(w).Encode(downloadResponse{Success: false, Error: "only POST allowed"})
		return
	}
	var req downloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(downloadResponse{Success: false, Error: "invalid JSON: " + err.Error()})
		return
	}
	if req.Name == "" || req.Source == "" {
		json.NewEncoder(w).Encode(downloadResponse{Success: false, Error: "name and source required"})
		return
	}
	song := &model.Song{
		ID: req.ID, Name: req.Name, Artist: req.Artist,
		Album: req.Album, AlbumID: req.AlbumID,
		Duration: req.Duration, Size: req.Size, Bitrate: req.Bitrate,
		Source: req.Source, URL: req.URL, Ext: req.Ext,
		Cover: req.Cover, Link: req.Link,
		Extra: req.Extra, IsInvalid: req.IsInvalid, IsVIP: req.IsVIP,
	}
	playable := core.ValidatePlayable(song)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true, "playable": playable,
	})
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost {
		json.NewEncoder(w).Encode(downloadResponse{Success: false, Error: "only POST allowed"})
		return
	}
	var req downloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(downloadResponse{Success: false, Error: "invalid JSON: " + err.Error()})
		return
	}
	if req.Name == "" || req.Source == "" {
		json.NewEncoder(w).Encode(downloadResponse{Success: false, Error: "name and source required"})
		return
	}
	if req.OutputDir == "" {
		req.OutputDir = "."
	}

	// 先尝试用请求的源下载
	song := &model.Song{
		ID: req.ID, Name: req.Name, Artist: req.Artist,
		Album: req.Album, AlbumID: req.AlbumID,
		Duration: req.Duration, Size: req.Size, Bitrate: req.Bitrate,
		Source: req.Source, URL: req.URL, Ext: req.Ext,
		Cover: req.Cover, Link: req.Link,
		Extra: req.Extra, IsInvalid: req.IsInvalid, IsVIP: req.IsVIP,
	}
	result, err := tryDownload(song, req.OutputDir)
	if err == nil {
		json.NewEncoder(w).Encode(downloadResponse{Success: true, FilePath: result.SavedPath, FileName: result.Filename})
		return
	}

	// 原源下载失败，自动换源重试
	log.Printf("[download] %s failed for %s, trying fallback sources", req.Source, req.Name)
	candidates := searchOtherSources(req.Name, req.Artist, req.Source)
	for _, candidate := range candidates {
		log.Printf("[download] trying fallback: %s (ID: %s)", candidate.Source, candidate.ID)
		result, err := tryDownload(candidate, req.OutputDir)
		if err == nil {
			log.Printf("[download] fallback success: %s -> %s", req.Source, candidate.Source)
			json.NewEncoder(w).Encode(downloadResponse{Success: true, FilePath: result.SavedPath, FileName: result.Filename})
			return
		}
	}

	json.NewEncoder(w).Encode(downloadResponse{Success: false, Error: err.Error()})
}