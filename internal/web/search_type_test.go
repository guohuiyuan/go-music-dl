package web

import (
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

func TestDefaultSourcesForSearchType(t *testing.T) {
	wantAlbum := core.GetAlbumSourceNames()
	if got := defaultSourcesForSearchType("album"); !reflect.DeepEqual(got, wantAlbum) {
		t.Fatalf("defaultSourcesForSearchType(album) = %v, want %v", got, wantAlbum)
	}

	if got := defaultSourcesForSearchType("playlist"); len(got) == 0 {
		t.Fatal("defaultSourcesForSearchType(playlist) returned empty sources")
	}

	if got := defaultSourcesForSearchType("song"); len(got) == 0 {
		t.Fatal("defaultSourcesForSearchType(song) returned empty sources")
	}
}

func TestSearchPlaceholderForType(t *testing.T) {
	tests := []struct {
		searchType string
		want       string
	}{
		{searchType: "song", want: "歌曲"},
		{searchType: "playlist", want: "歌单"},
		{searchType: "album", want: "专辑"},
	}

	for _, tt := range tests {
		if got := searchPlaceholderForType(tt.searchType); !strings.Contains(got, tt.want) {
			t.Fatalf("searchPlaceholderForType(%q) = %q, want contains %q", tt.searchType, got, tt.want)
		}
	}
}

func TestCollectionLabelsForSearchType(t *testing.T) {
	if got := collectionLabelForSearchType("album"); got != "专辑" {
		t.Fatalf("collectionLabelForSearchType(album) = %q, want 专辑", got)
	}
	if got := collectionCreatorLabelForSearchType("album"); got != "歌手" {
		t.Fatalf("collectionCreatorLabelForSearchType(album) = %q, want 歌手", got)
	}
	if got := collectionLabelForSearchType("playlist"); got != "歌单" {
		t.Fatalf("collectionLabelForSearchType(playlist) = %q, want 歌单", got)
	}
}

func TestPlaylistDetailURLIncludesCollectionMetadata(t *testing.T) {
	playlist := model.Playlist{
		ID:          "123",
		Name:        "My Playlist",
		Description: "Top picks",
		Cover:       "https://example.com/cover.jpg",
		TrackCount:  42,
		Creator:     "Creator",
		Source:      "qq",
		Link:        "https://y.qq.com/n/ryqq/playlist/123",
	}

	got := playlistDetailURL(RoutePrefix, collectionContentPlaylist, playlist)
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("playlistDetailURL parse: %v", err)
	}

	if parsed.Path != RoutePrefix+"/playlist" {
		t.Fatalf("playlistDetailURL path = %q, want %q", parsed.Path, RoutePrefix+"/playlist")
	}

	values := parsed.Query()
	if values.Get("id") != playlist.ID {
		t.Fatalf("query id = %q, want %q", values.Get("id"), playlist.ID)
	}
	if values.Get("source") != playlist.Source {
		t.Fatalf("query source = %q, want %q", values.Get("source"), playlist.Source)
	}
	if values.Get("name") != playlist.Name {
		t.Fatalf("query name = %q, want %q", values.Get("name"), playlist.Name)
	}
	if values.Get("track_count") != "42" {
		t.Fatalf("query track_count = %q, want 42", values.Get("track_count"))
	}
	if values.Get("link") != playlist.Link {
		t.Fatalf("query link = %q, want %q", values.Get("link"), playlist.Link)
	}
}

func TestImportCollectionFromQueryBuildsSongListImportButton(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest("GET", "/music/playlist?id=123&source=qq&name=Song+List&cover=https%3A%2F%2Fexample.com%2Fcover.jpg&creator=Tester&track_count=99&link=https%3A%2F%2Fy.qq.com%2Fn%2Fryqq%2Fplaylist%2F123", nil)
	ctx.Request = req

	meta := importCollectionFromQuery(ctx, collectionContentPlaylist, "qq", "123", core.GetOriginalLink("qq", "123", "playlist"), 12)
	if meta == nil {
		t.Fatal("importCollectionFromQuery returned nil")
	}
	if !meta.Enabled {
		t.Fatal("importCollectionFromQuery Enabled = false, want true")
	}
	if meta.Name != "Song List" {
		t.Fatalf("meta.Name = %q, want Song List", meta.Name)
	}
	if meta.TrackCount != 99 {
		t.Fatalf("meta.TrackCount = %d, want 99", meta.TrackCount)
	}
	if meta.ContentType != collectionContentPlaylist {
		t.Fatalf("meta.ContentType = %q, want %q", meta.ContentType, collectionContentPlaylist)
	}
	if !strings.Contains(meta.HoverText, "元数据") {
		t.Fatalf("meta.HoverText = %q, want contains 元数据", meta.HoverText)
	}
}

func TestApplyImportCollectionFallbackUsesParsedPlaylistMetadata(t *testing.T) {
	meta := &importCollectionMeta{
		Enabled:     true,
		Name:        "导入歌单",
		ContentType: collectionContentPlaylist,
	}

	pl := &model.Playlist{
		Name:        "流行热歌",
		Description: "自动解析到的歌单",
		Cover:       "https://example.com/cover.jpg",
		Creator:     "官方账号",
		TrackCount:  30,
		Link:        "https://example.com/playlist/1",
	}

	applyImportCollectionFallback(meta, pl, 8, "https://fallback.example.com")

	if meta.Name != "流行热歌" {
		t.Fatalf("meta.Name = %q, want 流行热歌", meta.Name)
	}
	if meta.Description != "自动解析到的歌单" {
		t.Fatalf("meta.Description = %q, want 自动解析到的歌单", meta.Description)
	}
	if meta.Cover != "https://example.com/cover.jpg" {
		t.Fatalf("meta.Cover = %q, want https://example.com/cover.jpg", meta.Cover)
	}
	if meta.Creator != "官方账号" {
		t.Fatalf("meta.Creator = %q, want 官方账号", meta.Creator)
	}
	if meta.TrackCount != 30 {
		t.Fatalf("meta.TrackCount = %d, want 30", meta.TrackCount)
	}
	if meta.Link != "https://example.com/playlist/1" {
		t.Fatalf("meta.Link = %q, want https://example.com/playlist/1", meta.Link)
	}
}
