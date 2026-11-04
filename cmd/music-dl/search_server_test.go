package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guohuiyuan/music-lib/model"

	"github.com/guohuiyuan/go-music-dl/core"
)

func TestSearchServerCommandIsPartOfMainBinary(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"search-server"})
	if err != nil {
		t.Fatal(err)
	}
	if command != searchServerCmd {
		t.Fatalf("search-server resolved to %q", command.CommandPath())
	}
	if !command.Hidden {
		t.Fatal("internal search server command should be hidden")
	}
}

func TestSearchServerHandlerSearchAndSort(t *testing.T) {
	lookup := func(source string) core.SearchFunc {
		switch source {
		case "qq":
			return func(string) ([]model.Song, error) {
				return []model.Song{
					{ID: "qq-2", Name: "Second", Artist: "QQ Artist"},
					{ID: "qq-1", Name: "First", Album: "QQ Album", Duration: 180},
				}, nil
			}
		case "netease":
			return func(string) ([]model.Song, error) {
				return []model.Song{{ID: "ne-1", Name: "Netease", Cover: "cover.jpg"}}, nil
			}
		case "broken":
			return func(string) ([]model.Song, error) {
				return nil, errors.New("provider unavailable")
			}
		default:
			return nil
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/search?keyword=test&sources=qq,netease,broken,unknown", nil)
	response := httptest.NewRecorder()
	newSearchServerHandler(lookup).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("CORS origin = %q", got)
	}

	var payload struct {
		Keyword string             `json:"keyword"`
		Sources []string           `json:"sources"`
		Songs   []searchServerSong `json:"songs"`
		Total   int                `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Keyword != "test" || payload.Total != 3 || len(payload.Songs) != 3 {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	wantIDs := []string{"ne-1", "qq-1", "qq-2"}
	for i, want := range wantIDs {
		if got := payload.Songs[i].ID; got != want {
			t.Fatalf("song %d id = %q, want %q", i, got, want)
		}
	}
	if payload.Songs[0].Cover != "cover.jpg" {
		t.Fatalf("cover = %q", payload.Songs[0].Cover)
	}
}

func TestSearchServerHandlerRejectsInvalidRequests(t *testing.T) {
	handler := newSearchServerHandler(core.GetSearchFunc)
	tests := []struct {
		name   string
		method string
		target string
		status int
	}{
		{name: "method", method: http.MethodPost, target: "/search", status: http.StatusMethodNotAllowed},
		{name: "keyword", method: http.MethodGet, target: "/search?sources=qq", status: http.StatusBadRequest},
		{name: "sources", method: http.MethodGet, target: "/search?keyword=test", status: http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.target, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}
