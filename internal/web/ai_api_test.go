package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestAIIndexDescribesCompactSearchAPI(t *testing.T) {
	router := newAIRouter()
	request := httptest.NewRequest(http.MethodGet, "/music/api/ai/index", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
	var payload struct {
		Name      string `json:"name"`
		LLMSTxt   string `json:"llms_txt"`
		Endpoints []struct {
			Method string `json:"method"`
			Path   string `json:"path"`
			Write  bool   `json:"write"`
			Auth   string `json:"auth"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Name != "go-music-dl" || payload.LLMSTxt != "/music/llms.txt" {
		t.Fatalf("unexpected index: %+v", payload)
	}
	if len(payload.Endpoints) < 2 || payload.Endpoints[0].Path != "/music/api/ai/search" {
		t.Fatalf("unexpected endpoints: %+v", payload.Endpoints)
	}
	wantPaths := []string{
		"/music/api/ai/playlist",
		"/music/api/ai/album",
		"/music/api/ai/toplists",
		"/music/api/ai/toplist",
		"/music/api/ai/playlist-categories",
		"/music/api/ai/category-playlists",
		"/music/api/ai/recommend",
		"/music/api/ai/user-playlists",
		"/music/inspect",
		"/music/download",
		"/music/api/downloads/records",
		"/music/api/downloads/precheck",
		"/music/api/downloads/queue",
		"/music/api/downloads/queue/container",
		"/music/api/ai/local-music",
		"/music/local_music",
		"/music/collections",
		"/music/healthz",
	}
	paths := make(map[string]struct{}, len(payload.Endpoints))
	var queueWriteAuth string
	for _, endpoint := range payload.Endpoints {
		paths[endpoint.Path] = struct{}{}
		if endpoint.Method == http.MethodPost && endpoint.Path == "/music/api/downloads/queue" && endpoint.Write {
			queueWriteAuth = endpoint.Auth
		}
	}
	for _, want := range wantPaths {
		if _, ok := paths[want]; !ok {
			t.Fatalf("index missing endpoint %q", want)
		}
	}
	if queueWriteAuth != "X-Requested-With: XMLHttpRequest" {
		t.Fatalf("queue write auth = %q", queueWriteAuth)
	}
}

func TestAILLMSTextPointsToSearchAndIndex(t *testing.T) {
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
	for _, want := range []string{"/music/api/ai/index", "/music/api/ai/search", "Search limit is 1-50"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("llms.txt missing %q: %s", want, response.Body.String())
		}
	}
}

func TestAISearchReturnsCompactJSONAndSourceErrors(t *testing.T) {
	providers := aiSearchProviders{
		SearchSongs: func(source string) func(string) ([]model.Song, error) {
			switch source {
			case "qq":
				return func(string) ([]model.Song, error) {
					return []model.Song{
						{
							ID:       "qq-1",
							Source:   "qq",
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
						{ID: "qq-2", Source: "qq", Name: "Song 2", Artist: "Artist"},
					}, nil
				}
			case "netease":
				return func(string) ([]model.Song, error) {
					return []model.Song{
						{ID: "ne-1", Source: "netease", Name: "Song 3", Artist: "Artist"},
						{ID: "ne-2", Source: "netease", Name: "Song 4", Artist: "Artist"},
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

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/music/api/ai/search", aiSearchHandler("/music", providers))
	request := httptest.NewRequest(http.MethodGet, "/music/api/ai/search?q=test&type=song&sources=qq,netease,kugou&limit=3", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
	var payload aiSearchResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total != 4 || payload.Count != 3 || !payload.Truncated {
		t.Fatalf("unexpected counts: %+v", payload)
	}
	gotIDs := []string{payload.Songs[0].ID, payload.Songs[1].ID, payload.Songs[2].ID}
	if want := []string{"qq-1", "ne-1", "qq-2"}; !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("song ids = %v, want %v", gotIDs, want)
	}
	if len(payload.Errors) != 1 || payload.Errors[0].Source != "kugou" || !strings.Contains(payload.Errors[0].Error, "unavailable") {
		t.Fatalf("unexpected source errors: %+v", payload.Errors)
	}
	if payload.NextAction != "/music/download?id={id}&source={source}&name={name}&artist={artist}&stream=1&extra={extra_json}" {
		t.Fatalf("next action = %q", payload.NextAction)
	}
	if payload.Songs[0].Rank != "1" || payload.Songs[0].Extra["songmid"] != "mid-1" {
		t.Fatalf("missing compact playback extra: %+v", payload.Songs[0])
	}
	if _, ok := payload.Songs[0].Extra["ignored_noise"]; ok {
		t.Fatalf("unexpected noise extra: %+v", payload.Songs[0].Extra)
	}
}

func TestAISearchSupportsPlaylistType(t *testing.T) {
	providers := aiSearchProviders{
		SearchPlaylists: func(source string) func(string) ([]model.Playlist, error) {
			return func(string) ([]model.Playlist, error) {
				return []model.Playlist{{ID: "playlist-1", Source: source, Name: "Mix", Creator: "User", TrackCount: 12}}, nil
			}
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/music/api/ai/search", aiSearchHandler("/music", providers))
	request := httptest.NewRequest(http.MethodGet, "/music/api/ai/search?q=mix&type=playlist&sources=qq&limit=5", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
	var payload aiSearchResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Playlists) != 1 || payload.Playlists[0].ID != "playlist-1" || payload.NextAction != "/music/api/ai/playlist?id={id}&source={source}&offset=0&limit=50" {
		t.Fatalf("unexpected playlist response: %+v", payload)
	}
}

func TestAISearchSupportsLocalSongs(t *testing.T) {
	providers := aiSearchProviders{
		SearchLocalSongs: func(keyword string, limit int) []model.Song {
			return []model.Song{{ID: "local-1", Name: keyword, Artist: "Artist"}}
		},
	}
	router := newAIResourceRouter(providers)
	var payload aiSearchResponse
	getAIJSON(t, router, "/music/api/ai/search?q=local&sources=local&limit=5", &payload)
	if payload.Count != 1 || payload.Songs[0].ID != "local-1" || payload.Songs[0].Source != "local" {
		t.Fatalf("unexpected local search response: %+v", payload)
	}
}

func TestAISearchInputValidation(t *testing.T) {
	router := newAIRouter()
	tests := []struct {
		path string
		code string
	}{
		{path: "/music/api/ai/search", code: "missing_q"},
		{path: "/music/api/ai/search?q=test&type=movie", code: "invalid_type"},
		{path: "/music/api/ai/search?q=test&limit=51", code: "invalid_limit"},
		{path: "/music/api/ai/search?q=test&sources=unknown", code: "invalid_sources"},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"error":"`+test.code+`"`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func newAIResourceRouter(providers aiSearchProviders) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/music")
	api.GET("/api/ai/search", aiSearchHandler("/music", providers))
	registerAIResourceRoutes(api, "/music", providers)
	return router
}

func getAIJSON(t *testing.T, router http.Handler, path string, target any) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s status=%d body=%s", path, response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode GET %s: %v; body=%s", path, err, response.Body.String())
	}
}

func TestAIResourceRoutesReturnCompactPaginatedSongs(t *testing.T) {
	providers := aiSearchProviders{
		GetPlaylistSongs: func(source string) func(string) ([]model.Song, error) {
			return func(string) ([]model.Song, error) {
				return []model.Song{
					{ID: "one", Source: "wrong", Name: "One"},
					{ID: "two", Source: "wrong", Name: "Two", Extra: map[string]string{"rid": "rid-2", "noise": "omit"}},
					{ID: "three", Source: "wrong", Name: "Three"},
				}, nil
			}
		},
		GetAlbumSongs: func(source string) func(string) ([]model.Song, error) {
			return func(string) ([]model.Song, error) {
				return []model.Song{
					{ID: "a1", Name: "A1"},
					{ID: "a2", Name: "A2"},
				}, nil
			}
		},
		GetLocalCollectionSongs: func(string) ([]model.Song, error) {
			return []model.Song{{ID: "local-1", Name: "Local", Album: "Disk"}}, nil
		},
		ListLocalSongs: func(offset, limit int) ([]model.Song, int, error) {
			return []model.Song{
				{ID: "local-a", Name: "Local A"},
				{ID: "local-b", Name: "Local B"},
			}, 4, nil
		},
	}
	router := newAIResourceRouter(providers)

	var playlist aiCollectionSongsResponse
	getAIJSON(t, router, "/music/api/ai/playlist?id=p1&source=kuwo&offset=1&limit=1", &playlist)
	if playlist.Kind != "playlist" || playlist.Source != "kuwo" || playlist.Total != 3 ||
		playlist.Count != 1 || !playlist.HasMore || playlist.Songs[0].ID != "two" {
		t.Fatalf("unexpected playlist response: %+v", playlist)
	}
	if playlist.Songs[0].Source != "kuwo" || playlist.Songs[0].Extra["rid"] != "rid-2" {
		t.Fatalf("playlist song not normalized/compacted: %+v", playlist.Songs[0])
	}
	if _, ok := playlist.Songs[0].Extra["noise"]; ok {
		t.Fatalf("playlist song retained unsupported extra: %+v", playlist.Songs[0].Extra)
	}

	var album aiCollectionSongsResponse
	getAIJSON(t, router, "/music/api/ai/album?id=al1&source=netease&limit=2", &album)
	if album.Kind != "album" || album.Total != 2 || album.Count != 2 || album.Songs[0].Source != "netease" {
		t.Fatalf("unexpected album response: %+v", album)
	}

	var local aiCollectionSongsResponse
	getAIJSON(t, router, "/music/api/ai/playlist?id=7&source=local&limit=5", &local)
	if local.Count != 1 || local.Songs[0].ID != "local-1" || local.Songs[0].Source != "local" {
		t.Fatalf("unexpected local playlist response: %+v", local)
	}

	var localLibrary aiLocalMusicResponse
	getAIJSON(t, router, "/music/api/ai/local-music?offset=0&limit=2", &localLibrary)
	if localLibrary.Total != 4 || localLibrary.Count != 2 || !localLibrary.HasMore ||
		localLibrary.Songs[0].Source != "local" {
		t.Fatalf("unexpected compact local music response: %+v", localLibrary)
	}
}

func TestAIResourceRoutesDiscoverRankingsCategoriesAndRecommendations(t *testing.T) {
	providers := aiSearchProviders{
		GetTopLists: func(source string) func() ([]model.TopList, error) {
			return func() ([]model.TopList, error) {
				return []model.TopList{{
					ID:              source + "-chart",
					Name:            source + " chart",
					TrackCount:      100,
					UpdateFrequency: "daily",
					Extra:           map[string]string{"group": "hot"},
				}}, nil
			}
		},
		GetTopListSongs: func(source string) func(string) ([]model.Song, error) {
			return func(string) ([]model.Song, error) {
				return []model.Song{{
					ID:    source + "-song",
					Name:  "Ranked",
					Extra: map[string]string{"rank": "7", "toplist_id": source + "-chart"},
				}}, nil
			}
		},
		GetPlaylistCategories: func(source string) func() ([]model.PlaylistCategory, error) {
			return func() ([]model.PlaylistCategory, error) {
				return []model.PlaylistCategory{{
					ID:    source + "-cat",
					Name:  "Pop",
					Group: "Genre",
					Count: 10,
					Hot:   true,
				}}, nil
			}
		},
		GetCategoryPlaylists: func(source string) func(string, int, int) ([]model.Playlist, error) {
			return func(categoryID string, page, limit int) ([]model.Playlist, error) {
				return []model.Playlist{{ID: "category-playlist", Name: categoryID, TrackCount: limit}}, nil
			}
		},
		GetRecommended: func(source string) func() ([]model.Playlist, error) {
			return func() ([]model.Playlist, error) {
				return []model.Playlist{{ID: source + "-recommend", Name: "Recommended"}}, nil
			}
		},
		GetUserPlaylists: func(source string) func(int, int) ([]model.Playlist, error) {
			if source == "qq" {
				return func(int, int) ([]model.Playlist, error) {
					return nil, errors.New("login required")
				}
			}
			return func(page, limit int) ([]model.Playlist, error) {
				return []model.Playlist{{ID: "mine", Name: "Mine", TrackCount: limit}}, nil
			}
		},
	}
	router := newAIResourceRouter(providers)

	var topLists aiTopListsResponse
	getAIJSON(t, router, "/music/api/ai/toplists?sources=netease,qq&limit=10", &topLists)
	if topLists.Total != 2 || topLists.Count != 2 || topLists.TopLists[0].Source != "netease" ||
		topLists.TopLists[1].Source != "qq" || topLists.TopLists[0].Group != "hot" {
		t.Fatalf("unexpected toplists response: %+v", topLists)
	}

	var topListSongs aiCollectionSongsResponse
	getAIJSON(t, router, "/music/api/ai/toplist?id=qq-chart&source=qq&limit=5", &topListSongs)
	if topListSongs.Kind != "toplist" || topListSongs.Count != 1 ||
		topListSongs.Songs[0].Rank != "7" || topListSongs.Songs[0].Source != "qq" {
		t.Fatalf("unexpected toplist songs: %+v", topListSongs)
	}

	var categories aiPlaylistCategoriesResponse
	getAIJSON(t, router, "/music/api/ai/playlist-categories?sources=netease,qq&limit=10", &categories)
	if categories.Count != 2 || categories.Categories[0].Source != "netease" ||
		!categories.Categories[0].Hot || categories.NextAction == "" {
		t.Fatalf("unexpected categories response: %+v", categories)
	}

	var categoryPlaylists aiPlaylistListResponse
	getAIJSON(t, router, "/music/api/ai/category-playlists?source=qq&category_id=pop&page=1&limit=7", &categoryPlaylists)
	if categoryPlaylists.Source != "qq" || categoryPlaylists.Count != 1 ||
		categoryPlaylists.Playlists[0].Source != "qq" || categoryPlaylists.Playlists[0].TrackCount != 7 {
		t.Fatalf("unexpected category playlists response: %+v", categoryPlaylists)
	}

	var allCategoryPlaylists aiPlaylistListResponse
	getAIJSON(t, router, "/music/api/ai/category-playlists?source=qq&category_id=&page=1&limit=2", &allCategoryPlaylists)
	if allCategoryPlaylists.Count != 1 || allCategoryPlaylists.Playlists[0].Source != "qq" {
		t.Fatalf("empty category id should mean all: %+v", allCategoryPlaylists)
	}

	var recommendations aiPlaylistListResponse
	getAIJSON(t, router, "/music/api/ai/recommend?sources=netease,qq&limit=10", &recommendations)
	if recommendations.Count != 2 || recommendations.Playlists[0].Source != "netease" ||
		recommendations.Playlists[1].Source != "qq" {
		t.Fatalf("unexpected recommendations response: %+v", recommendations)
	}

	var userPlaylists aiPlaylistListResponse
	getAIJSON(t, router, "/music/api/ai/user-playlists?sources=netease,qq&page=1&limit=5", &userPlaylists)
	if userPlaylists.Count != 1 || userPlaylists.Playlists[0].Source != "netease" ||
		len(userPlaylists.Errors) != 1 || userPlaylists.Errors[0].Source != "qq" {
		t.Fatalf("unexpected user playlists response: %+v", userPlaylists)
	}
}

func TestAIResourceInputValidation(t *testing.T) {
	router := newAIResourceRouter(aiSearchProviders{})
	tests := []struct {
		path string
		code string
	}{
		{path: "/music/api/ai/playlist?source=qq", code: "missing_id"},
		{path: "/music/api/ai/playlist?id=1&source=unknown", code: "invalid_source"},
		{path: "/music/api/ai/playlist?id=1&source=qq&limit=201", code: "invalid_limit"},
		{path: "/music/api/ai/album?id=1&source=qq&offset=-1", code: "invalid_offset"},
		{path: "/music/api/ai/toplists?sources=unknown", code: "invalid_sources"},
		{path: "/music/api/ai/category-playlists?source=netease", code: "missing_category_id"},
		{path: "/music/api/ai/user-playlists?sources=qq&page=0", code: "invalid_page"},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"error":"`+test.code+`"`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestParseAISearchSourcesSupportsCommaAndDeduplication(t *testing.T) {
	got, err := parseAISearchSources([]string{"qq,netease", "qq"}, "song")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"qq", "netease"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %v, want %v", got, want)
	}

	all, err := parseAISearchSources([]string{"all"}, "album")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) == 0 || !containsStringValue(all, "qq") || !containsStringValue(all, "apple") {
		t.Fatalf("unexpected all sources: %v", all)
	}
}
