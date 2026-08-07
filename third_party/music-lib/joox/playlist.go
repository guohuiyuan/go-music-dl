package joox

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultJoox.SearchPlaylist(keyword)
}

func GetPlaylistSongs(id string) ([]model.Song, error) { return defaultJoox.GetPlaylistSongs(id) }

func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultJoox.ParsePlaylist(link)
}

func GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return defaultJoox.GetPlaylistCategories()
}

func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultJoox.GetCategoryPlaylists(categoryID, page, limit)
}

func (j *Joox) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	data, err := j.fetchJooxPlaylistCategoriesPage()
	if err != nil {
		return nil, err
	}

	categories := []model.PlaylistCategory{{
		Source: "joox",
		ID:     "",
		Name:   "全部",
		Group:  "全部",
	}}
	for index, category := range data.Categories {
		name := decodeJooxBase64Text(category.Title)
		if name == "" || len(category.ItemList) == 0 {
			continue
		}
		categoryID := strconv.Itoa(index)
		categories = append(categories, model.PlaylistCategory{
			Source: "joox",
			ID:     categoryID,
			Name:   name,
			Group:  "JOOX",
			Count:  len(category.ItemList),
			Extra: map[string]string{
				"title": category.Title,
				"type":  strconv.Itoa(category.Type),
			},
		})
	}
	if len(categories) == 1 {
		return nil, errors.New("no playlist categories found")
	}
	return categories, nil
}

func (j *Joox) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	categoryID = strings.TrimSpace(categoryID)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	data, err := j.fetchJooxPlaylistCategoriesPage()
	if err != nil {
		return nil, err
	}

	var items []jooxCategoryPlaylistItem
	categoryName := "全部"
	if categoryID == "" {
		seen := map[string]struct{}{}
		for _, category := range data.Categories {
			for _, item := range category.ItemList {
				playlistID := jooxCategoryPlaylistID(item.ID)
				if playlistID == "" {
					continue
				}
				if _, ok := seen[playlistID]; ok {
					continue
				}
				seen[playlistID] = struct{}{}
				items = append(items, item)
			}
		}
	} else {
		for index, category := range data.Categories {
			name := decodeJooxBase64Text(category.Title)
			if strconv.Itoa(index) == categoryID || strings.EqualFold(name, categoryID) {
				items = category.ItemList
				categoryName = name
				break
			}
		}
		if len(items) == 0 {
			return nil, errors.New("playlist category not found")
		}
	}

	start := (page - 1) * limit
	if start >= len(items) {
		return nil, errors.New("no category playlists found")
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}

	playlists := make([]model.Playlist, 0, end-start)
	for _, item := range items[start:end] {
		playlistID := jooxCategoryPlaylistID(item.ID)
		name := decodeJooxBase64Text(item.Title)
		if playlistID == "" || name == "" {
			continue
		}
		cover := strings.ReplaceAll(item.PicURL, "%d", "300")
		playlists = append(playlists, model.Playlist{
			Source: "joox",
			ID:     playlistID,
			Name:   name,
			Cover:  cover,
			Link:   jooxPlaylistLink(playlistID),
			Extra: map[string]string{
				"category_id":   categoryID,
				"category_name": categoryName,
				"playlist_id":   playlistID,
			},
		})
	}
	if len(playlists) == 0 {
		return nil, errors.New("no category playlists found")
	}
	return playlists, nil
}

func (j *Joox) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")
	params.Set("keyword", keyword)
	apiURL := "https://cache.api.joox.com/openjoox/v3/search?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	// Update struct to match joox_playlist_search.json structure
	var resp struct {
		SectionList []struct {
			SectionTitle string `json:"section_title"`
			SectionType  int    `json:"section_type"`
			ItemList     []struct {
				Type           int `json:"type"` // 1: Editor Playlist, 2: Album, 5: Song
				EditorPlaylist struct {
					ID     string `json:"id"`
					Name   string `json:"name"`
					Images []struct {
						Width int    `json:"width"`
						URL   string `json:"url"`
					} `json:"images"`
				} `json:"editor_playlist"`
			} `json:"item_list"`
		} `json:"section_list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("joox search playlist json error: %w", err)
	}

	var playlists []model.Playlist
	for _, section := range resp.SectionList {
		for _, item := range section.ItemList {
			// According to the JSON, Type 1 is a playlist (Editor Playlist)
			// We skip Albums (Type 2) or Songs (Type 5)
			if item.Type != 1 {
				continue
			}

			info := item.EditorPlaylist
			playlistID := normalizeJooxID(info.ID)
			if playlistID == "" {
				continue
			}

			// Image selection logic: prioritize 300px, fallback to first available
			var cover string
			for _, img := range info.Images {
				if img.Width == 300 {
					cover = img.URL
					break
				}
			}
			if cover == "" && len(info.Images) > 0 {
				cover = info.Images[0].URL
			}

			// Generate the public link
			link := jooxPlaylistLink(playlistID)

			// Populate the Playlist model
			playlists = append(playlists, model.Playlist{
				Source: "joox", // Essential for universal player logic
				ID:     playlistID,
				Name:   info.Name,
				Cover:  cover,
				Link:   link,

				// Fields not provided in the Search API response (JSON):
				// TrackCount:  0,
				// PlayCount:   0,
				// Creator:     "",
				// Description: "",

				// Optional: Store raw ID in Extra if needed for specific logic later
				Extra: map[string]string{
					"playlist_id": playlistID,
				},
			})
		}
	}
	return playlists, nil
}

func (j *Joox) GetPlaylistSongs(id string) ([]model.Song, error) {
	params := url.Values{}
	// The new v3 API uses "id" instead of "playlistid"
	params.Set("id", id)
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")

	// Use the same host/path structure as Search
	// Guessing the endpoint is /playlist based on /search pattern
	apiURL := "https://cache.api.joox.com/openjoox/v3/playlist?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		if fallbackSongs, fallbackErr := j.fetchPlaylistSongsFromPage(id); fallbackErr == nil && len(fallbackSongs) > 0 {
			return fallbackSongs, nil
		}
		return nil, err
	}

	// We reuse the generic "Section" structure from the Search API
	// because modern Joox APIs (v3) return "Pages" composed of "Sections".
	var resp struct {
		SectionList []struct {
			ItemList []struct {
				Type int `json:"type"` // We look for Type 5 (Song)
				Song []struct {
					SongInfo struct {
						ID         string `json:"id"`
						Name       string `json:"name"`
						AlbumName  string `json:"album_name"`
						AlbumID    string `json:"album_id"`
						ArtistList []struct {
							Name string `json:"name"`
						} `json:"artist_list"`
						PlayDuration int `json:"play_duration"`
						Images       []struct {
							Width int    `json:"width"`
							URL   string `json:"url"`
						} `json:"images"`
						VipFlag int `json:"vip_flag"`
					} `json:"song_info"`
				} `json:"song"`
			} `json:"item_list"`
		} `json:"section_list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		if fallbackSongs, fallbackErr := j.fetchPlaylistSongsFromPage(id); fallbackErr == nil && len(fallbackSongs) > 0 {
			return fallbackSongs, nil
		}
		return nil, fmt.Errorf("joox playlist json error: %w", err)
	}

	var songs []model.Song
	foundSongs := false

	// Iterate through all sections to find songs
	for _, section := range resp.SectionList {
		for _, item := range section.ItemList {
			// Type 5 corresponds to Songs in the v3 API (as seen in your search json)
			if item.Type == 5 {
				for _, songItem := range item.Song {
					info := songItem.SongInfo
					if info.ID == "" {
						continue
					}

					var artistNames []string
					for _, ar := range info.ArtistList {
						artistNames = append(artistNames, ar.Name)
					}

					var cover string
					for _, img := range info.Images {
						if img.Width == 300 {
							cover = img.URL
							break
						}
					}
					if cover == "" && len(info.Images) > 0 {
						cover = info.Images[0].URL
					}

					// Fallback for missing cover using AlbumID if available
					if cover == "" && info.AlbumID != "" {
						// Standard Joox album cover pattern
						cover = fmt.Sprintf("https://imgcache.joox.com/music/joox/photo/mid_album_300/%s/%s/%s.jpg",
							info.AlbumID[len(info.AlbumID)-2:],
							info.AlbumID[len(info.AlbumID)-1:],
							info.AlbumID)
					}

					songs = append(songs, model.Song{
						Source:   "joox",
						ID:       info.ID,
						Name:     info.Name,
						Artist:   strings.Join(artistNames, " / "),
						Album:    info.AlbumName,
						Duration: info.PlayDuration,
						Cover:    cover,
						Link:     fmt.Sprintf("https://www.joox.com/hk/single/%s", info.ID),
						Extra: map[string]string{
							"songid": info.ID,
						},
					})
					foundSongs = true
				}
			}
		}
	}

	if !foundSongs {
		if fallbackSongs, fallbackErr := j.fetchPlaylistSongsFromPage(id); fallbackErr == nil && len(fallbackSongs) > 0 {
			return fallbackSongs, nil
		}
		// If no songs found, the ID might be invalid or the playlist is empty
		return nil, errors.New("no songs found in playlist or invalid playlist ID")
	}

	return songs, nil
}

func (j *Joox) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`joox\.com/.*/playlist/([^/?#]+)`),
		regexp.MustCompile(`(?:playlistid|playlist_id|id)=([^&]+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(link)
		if len(matches) >= 2 {
			return j.fetchPlaylistDetail(matches[1])
		}
	}

	if len(link) > 8 && !strings.Contains(link, "/") {
		return j.fetchPlaylistDetail(link)
	}

	return nil, nil, errors.New("invalid joox playlist link")
}

type jooxPlaylistCategoryPage struct {
	Categories []jooxPlaylistCategory `json:"category"`
}

type jooxPlaylistCategory struct {
	Type     int                        `json:"type"`
	Title    string                     `json:"title"`
	ItemList []jooxCategoryPlaylistItem `json:"itemlist"`
}

type jooxCategoryPlaylistItem struct {
	ID     interface{} `json:"id"`
	Title  string      `json:"title"`
	PicURL string      `json:"picurl"`
}

func (j *Joox) fetchJooxPlaylistCategoriesPage() (*jooxPlaylistCategoryPage, error) {
	body, err := utils.Get("https://www.joox.com/sg/playlist",
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	matches := regexp.MustCompile(`(?s)<script[^>]*id="__NEXT_DATA__"[^>]*>(.*?)</script>`).FindSubmatch(body)
	if len(matches) < 2 {
		return nil, errors.New("joox playlist category page data not found")
	}

	var nextData struct {
		Props struct {
			PageProps struct {
				MLList jooxPlaylistCategoryPage `json:"mlList"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(matches[1], &nextData); err != nil {
		return nil, fmt.Errorf("joox playlist category page json error: %w", err)
	}
	if len(nextData.Props.PageProps.MLList.Categories) == 0 {
		return nil, errors.New("joox playlist categories not found")
	}
	return &nextData.Props.PageProps.MLList, nil
}

func decodeJooxBase64Text(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	padded := value
	if rem := len(padded) % 4; rem != 0 {
		padded += strings.Repeat("=", 4-rem)
	}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding} {
		if decoded, err := encoding.DecodeString(padded); err == nil {
			return strings.TrimSpace(string(decoded))
		}
	}
	return value
}

func jooxCategoryPlaylistID(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return normalizeJooxID(v)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int:
		return strconv.Itoa(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
