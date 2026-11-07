package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/music-lib/model"
)

func newAIRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterAIRoutes(router.Group("/music"))
	return router
}

func TestAIIndexUsesExistingRoutesAndOnlyPlayerControls(t *testing.T) {
	router := newAIRouter()
	request := httptest.NewRequest(http.MethodGet, "/music/api/ai/index", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
	var payload struct {
		Name          string `json:"name"`
		SchemaVersion int    `json:"schema_version"`
		Search        string `json:"search"`
		SwitchSource  string `json:"switch_source"`
		LLMSTxt       string `json:"llms_txt"`
		Endpoints     []struct {
			Method string `json:"method"`
			Path   string `json:"path"`
			Write  bool   `json:"write"`
			Auth   string `json:"auth"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Name != "go-music-dl" || payload.SchemaVersion != 2 ||
		payload.Search != "/music/search" ||
		payload.SwitchSource != "/music/switch_source" ||
		payload.LLMSTxt != "/music/llms.txt" {
		t.Fatalf("unexpected index: %+v", payload)
	}

	wantRoutes := []string{
		"GET /music/search",
		"GET /music/playlist",
		"GET /music/album",
		"GET /music/toplists",
		"GET /music/toplist_songs",
		"GET /music/playlist_categories",
		"GET /music/category_playlists",
		"GET /music/recommend",
		"GET /music/user_playlists",
		"GET /music/switch_source",
		"GET /music/inspect",
		"GET /music/download",
		"GET /music/lyric",
		"GET /music/local_music",
		"GET /music/collections",
		"GET /music/collections/{id}/songs",
		"GET /music/api/downloads/queue",
		"POST /music/api/downloads/queue",
		"POST /music/api/downloads/queue/container",
		"GET /music/api/ai/player/state",
		"GET /music/api/ai/player/events",
		"POST /music/api/ai/player/play",
		"POST /music/api/ai/player/command",
		"POST /music/api/ai/player/state",
	}
	routes := make(map[string]struct{}, len(payload.Endpoints))
	paths := make(map[string]struct{}, len(payload.Endpoints))
	var queueWriteAuth string
	for _, endpoint := range payload.Endpoints {
		key := endpoint.Method + " " + endpoint.Path
		routes[key] = struct{}{}
		paths[endpoint.Path] = struct{}{}
		if strings.HasPrefix(endpoint.Path, "/music/api/ai/") &&
			endpoint.Path != "/music/api/ai/index" &&
			!strings.HasPrefix(endpoint.Path, "/music/api/ai/player/") {
			t.Fatalf("index advertises non-player AI wrapper %q", endpoint.Path)
		}
		if key == "POST /music/api/downloads/queue" && endpoint.Write {
			queueWriteAuth = endpoint.Auth
		}
	}
	for _, want := range wantRoutes {
		if _, ok := routes[want]; !ok {
			t.Fatalf("index missing existing route %q", want)
		}
	}
	if queueWriteAuth != "X-Requested-With: XMLHttpRequest" {
		t.Fatalf("queue write auth = %q", queueWriteAuth)
	}

	for _, removed := range []string{
		"/music/api/ai/search",
		"/music/api/ai/playlist",
		"/music/api/ai/album",
		"/music/api/ai/toplists",
		"/music/api/ai/toplist",
		"/music/api/ai/playlist-categories",
		"/music/api/ai/category-playlists",
		"/music/api/ai/recommend",
		"/music/api/ai/user-playlists",
		"/music/api/ai/local-music",
	} {
		if _, ok := paths[removed]; ok {
			t.Fatalf("index still advertises duplicate AI wrapper %q", removed)
		}
	}
}

func TestAIRegistersNoDuplicateResourceRoutes(t *testing.T) {
	router := newAIRouter()
	got := make([]string, 0, len(router.Routes()))
	for _, route := range router.Routes() {
		got = append(got, route.Method+" "+route.Path)
	}
	sort.Strings(got)
	want := []string{
		"GET /music/api/ai/index",
		"GET /music/api/ai/player/events",
		"GET /music/api/ai/player/state",
		"GET /music/llms.txt",
		"POST /music/api/ai/player/command",
		"POST /music/api/ai/player/play",
		"POST /music/api/ai/player/state",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registered AI routes = %v, want %v", got, want)
	}
}

func TestAILLMSTextPointsToExistingRoutes(t *testing.T) {
	router := newAIRouter()
	request := httptest.NewRequest(http.MethodGet, "/music/llms.txt", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	body := response.Body.String()
	for _, want := range []string{
		"GET /music/search",
		"GET /music/switch_source",
		"GET /music/toplists",
		"POST /music/api/ai/player/play",
		"POST /music/api/ai/player/command",
		"409 player_not_connected",
		"Existing search, playlist, album, ranking, recommendation, and category routes return the normal rendered page",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("llms.txt missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "/api/ai/search") ||
		strings.Contains(body, "/api/ai/playlist") ||
		strings.Contains(body, "/api/ai/local-music") {
		t.Fatalf("llms.txt advertises removed duplicate routes: %s", body)
	}
}

func TestParseAIPlayerSourcesSupportsCommaAndAll(t *testing.T) {
	got, err := parseAIPlayerSources([]string{"qq,netease", "qq"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"qq", "netease"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %v, want %v", got, want)
	}

	all, err := parseAIPlayerSources([]string{"all"})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) == 0 || !containsStringValue(all, "qq") ||
		!containsStringValue(all, "apple") || !containsStringValue(all, "spotify") ||
		!containsStringValue(all, "local") {
		t.Fatalf("unexpected all sources: %v", all)
	}

	if _, err := parseAIPlayerSources([]string{"unknown"}); err == nil {
		t.Fatal("expected unknown source to fail")
	}
}

func TestExecuteAISongSearchCompactsResultsAndKeepsSourceErrors(t *testing.T) {
	providers := aiSearchProviders{
		SearchSongs: func(source string) func(string) ([]model.Song, error) {
			switch source {
			case "qq":
				return func(string) ([]model.Song, error) {
					return []model.Song{
						{
							ID:       "qq-1",
							Source:   "wrong",
							Name:     "Song 1",
							Artist:   "Artist",
							Album:    "Album",
							Duration: 200,
							Extra: map[string]string{
								"rank":          "1",
								"songmid":       "mid-1",
								"ignored_noise": "omit",
							},
						},
						{ID: "qq-2", Name: "Song 2", Artist: "Artist"},
					}, nil
				}
			case "netease":
				return func(string) ([]model.Song, error) {
					return []model.Song{
						{ID: "ne-1", Name: "Song 3", Artist: "Artist"},
						{ID: "ne-2", Name: "Song 4", Artist: "Artist"},
					}, nil
				}
			case "kugou":
				return func(string) ([]model.Song, error) {
					return nil, errors.New("upstream unavailable")
				}
			default:
				return nil
			}
		},
	}

	result := executeAISongSearch("test", []string{"qq", "netease", "kugou"}, "", 3, providers)
	if result.Total != 4 || len(result.Songs) != 3 || !result.Truncated {
		t.Fatalf("unexpected result: %+v", result)
	}
	gotIDs := []string{result.Songs[0].ID, result.Songs[1].ID, result.Songs[2].ID}
	if want := []string{"qq-1", "ne-1", "qq-2"}; !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("song ids = %v, want %v", gotIDs, want)
	}
	if len(result.Errors) != 1 || result.Errors[0].Source != "kugou" ||
		!strings.Contains(result.Errors[0].Error, "unavailable") {
		t.Fatalf("unexpected source errors: %+v", result.Errors)
	}
	if result.Songs[0].Source != "qq" ||
		result.Songs[0].Rank != "1" ||
		result.Songs[0].Extra["songmid"] != "mid-1" {
		t.Fatalf("result was not normalized/compacted: %+v", result.Songs[0])
	}
	if _, ok := result.Songs[0].Extra["ignored_noise"]; ok {
		t.Fatalf("unexpected noise extra: %+v", result.Songs[0].Extra)
	}
}

func TestExecuteAISongSearchSupportsLocalSongs(t *testing.T) {
	providers := aiSearchProviders{
		SearchLocalSongs: func(keyword string, limit int) []model.Song {
			return []model.Song{{ID: "local-1", Name: keyword, Artist: "Artist"}}
		},
	}
	result := executeAISongSearch("local", []string{"local"}, "", 5, providers)
	if len(result.Songs) != 1 || result.Songs[0].ID != "local-1" || result.Songs[0].Source != "local" {
		t.Fatalf("unexpected local search result: %+v", result)
	}
}
