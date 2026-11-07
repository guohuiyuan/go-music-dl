package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/guohuiyuan/go-music-dl/core"
)

type searchServerSong struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Duration int    `json:"duration"`
	Source   string `json:"source"`
	Cover    string `json:"cover"`
}

type searchProviderLookup func(string) core.SearchFunc

var searchServerCmd = &cobra.Command{
	Use:    "search-server",
	Short:  "启动本地搜索服务",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		core.CM.Load()
		return runSearchServer(cmd.OutOrStdout(), core.GetSearchFunc)
	},
}

func init() {
	rootCmd.AddCommand(searchServerCmd)
}

func runSearchServer(stdout io.Writer, lookup searchProviderLookup) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen for local search: %w", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	if _, err := fmt.Fprintf(stdout, "PORT:%d\n", port); err != nil {
		return fmt.Errorf("write local search port: %w", err)
	}
	if syncer, ok := stdout.(interface{ Sync() error }); ok {
		_ = syncer.Sync()
	}

	server := &http.Server{
		Handler:      newSearchServerHandler(lookup),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve local search: %w", err)
	}
	return nil
}

func newSearchServerHandler(lookup searchProviderLookup) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		handleSearchServerRequest(w, r, lookup)
	})
	return mux
}

func handleSearchServerRequest(w http.ResponseWriter, r *http.Request, lookup searchProviderLookup) {
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
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results []searchServerSong
	)

	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}

		search := lookup(source)
		if search == nil {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			songs, err := search(keyword)
			if err != nil {
				return
			}

			mu.Lock()
			defer mu.Unlock()
			for _, song := range songs {
				results = append(results, searchServerSong{
					ID:       song.ID,
					Name:     song.Name,
					Artist:   song.Artist,
					Album:    song.Album,
					Duration: song.Duration,
					Source:   source,
					Cover:    song.Cover,
				})
			}
		}()
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		if results[i].Source != results[j].Source {
			return results[i].Source < results[j].Source
		}
		return results[i].Name < results[j].Name
	})

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"keyword": keyword,
		"sources": sources,
		"songs":   results,
		"total":   len(results),
	})
}
