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
)

type searchResult struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Duration int    `json:"duration"`
	Source   string `json:"source"`
	Cover    string `json:"cover"`
}

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

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
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
		if source == "" {
			continue
		}

		searchFn := core.GetSearchFunc(source)
		if searchFn == nil {
			continue
		}

		wg.Add(1)
		go func(src string, fn core.SearchFunc) {
			defer wg.Done()

			songs, err := fn(keyword)
			if err != nil {
				return
			}

			mu.Lock()
			for _, song := range songs {
				results = append(results, searchResult{
					ID:       song.ID,
					Name:     song.Name,
					Artist:   song.Artist,
					Album:    song.Album,
					Duration: song.Duration,
					Source:   src,
					Cover:    song.Cover,
				})
			}
			mu.Unlock()
		}(source, searchFn)
	}

	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		if results[i].Source != results[j].Source {
			return results[i].Source < results[j].Source
		}
		return results[i].Name < results[j].Name
	})

	resp := map[string]interface{}{
		"keyword": keyword,
		"sources": sources,
		"songs":   results,
		"total":   len(results),
	}

	json.NewEncoder(w).Encode(resp)
}