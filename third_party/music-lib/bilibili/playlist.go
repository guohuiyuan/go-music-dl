package bilibili

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultBilibili.SearchPlaylist(keyword)
}

func GetPlaylistSongs(id string) ([]model.Song, error) { return defaultBilibili.GetPlaylistSongs(id) }

func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultBilibili.ParsePlaylist(link)
}

func GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return defaultBilibili.GetPlaylistCategories()
}

func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultBilibili.GetCategoryPlaylists(categoryID, page, limit)
}

func (b *Bilibili) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

func (b *Bilibili) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

// SearchPlaylist 搜索合集/分P
func (b *Bilibili) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("search_type", "video")
	params.Set("keyword", keyword)
	params.Set("page", "1")
	params.Set("page_size", "20")

	searchURL := "https://api.bilibili.com/x/web-interface/search/type?" + params.Encode()
	body, err := utils.Get(searchURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return nil, err
	}

	var searchResp struct {
		Data struct {
			Result []struct {
				BVID   string `json:"bvid"`
				Title  string `json:"title"`
				Author string `json:"author"`
				Pic    string `json:"pic"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("bilibili search json error: %w", err)
	}

	plMap := make(map[string]bool)
	var playlists []model.Playlist
	for _, item := range searchResp.Data.Result {
		viewResp, err := b.fetchView(item.BVID)
		if err != nil {
			continue
		}

		cover := normalizeCover(item.Pic)
		if viewResp.Data.UgcSeason != nil {
			seasonID := viewResp.Data.UgcSeason.ID
			mid := viewResp.Data.Owner.Mid
			key := fmt.Sprintf("season:%d:%d:%s", seasonID, mid, item.BVID)
			if plMap[key] {
				continue
			}
			plMap[key] = true

			seasonName := viewResp.Data.UgcSeason.Title
			if seasonName == "" {
				seasonName = cleanTitle(item.Title)
			}
			seasonCover := viewResp.Data.UgcSeason.Cover
			if seasonCover == "" {
				seasonCover = cover
			}

			trackCount := countSeasonEpisodes(viewResp.Data.UgcSeason.Sections)
			if trackCount == 0 && seasonID != 0 && mid != 0 {
				seasonSongs, err := b.fetchSeasonSongs(mid, seasonID)
				if err == nil {
					trackCount = len(seasonSongs)
				}
			}

			playlists = append(playlists, model.Playlist{
				Source:      "bilibili",
				ID:          key,
				Name:        seasonName,
				Cover:       normalizeCover(seasonCover),
				TrackCount:  trackCount,
				PlayCount:   viewResp.Data.UgcSeason.Stat.View,
				Creator:     viewResp.Data.Owner.Name,
				Description: viewResp.Data.UgcSeason.Intro,
				Link:        fmt.Sprintf("https://www.bilibili.com/video/%s", item.BVID),
				Extra: map[string]string{
					"season_id": strconv.FormatInt(seasonID, 10),
					"mid":       strconv.FormatInt(mid, 10),
					"bvid":      item.BVID,
					"type":      "season",
				},
			})
			continue
		}

		if len(viewResp.Data.Pages) > 1 {
			plID := "bvid:" + item.BVID
			if plMap[plID] {
				continue
			}
			plMap[plID] = true
			playlists = append(playlists, model.Playlist{
				Source:     "bilibili",
				ID:         plID,
				Name:       cleanTitle(item.Title),
				Cover:      cover,
				TrackCount: len(viewResp.Data.Pages),
				Creator:    viewResp.Data.Owner.Name,
				Link:       fmt.Sprintf("https://www.bilibili.com/video/%s", item.BVID),
				Extra: map[string]string{
					"bvid": item.BVID,
					"type": "multipart",
				},
			})
		}
	}
	return playlists, nil
}

// GetPlaylistSongs 获取合集/分P所有歌曲
func (b *Bilibili) GetPlaylistSongs(id string) ([]model.Song, error) {
	if strings.HasPrefix(id, "season:") {
		parts := strings.Split(id, ":")
		if len(parts) < 3 {
			return nil, errors.New("invalid season id")
		}
		seasonID, _ := strconv.ParseInt(parts[1], 10, 64)
		mid, _ := strconv.ParseInt(parts[2], 10, 64)
		bvid := ""
		if len(parts) >= 4 {
			bvid = parts[3]
		}
		if bvid != "" {
			viewResp, err := b.fetchView(bvid)
			if err == nil && viewResp.Data.UgcSeason != nil {
				sections := viewResp.Data.UgcSeason.Sections
				archiveIndex := map[string]bilibiliSeasonArchiveMeta{}
				seasonTitle := viewResp.Data.UgcSeason.Title
				seasonCover := viewResp.Data.UgcSeason.Cover
				idx, sTitle, sCover, idxErr := b.fetchSeasonArchiveIndex(mid, seasonID)
				if idxErr == nil {
					archiveIndex = idx
					if seasonTitle == "" {
						seasonTitle = sTitle
					}
					if seasonCover == "" {
						seasonCover = sCover
					}
				}
				songs := b.buildSongsFromSeasonSections(sections, seasonTitle, seasonCover, viewResp.Data.Owner.Name, archiveIndex)
				if len(songs) > 0 {
					return songs, nil
				}
			}
		}
		return b.fetchSeasonSongs(mid, seasonID)
	}

	bvid := strings.TrimPrefix(id, "bvid:")
	if bvid == "" {
		return nil, errors.New("invalid playlist id")
	}
	viewResp, err := b.fetchView(bvid)
	if err != nil {
		return nil, err
	}
	rootTitle := viewResp.Data.Title
	pages := viewResp.Data.Pages
	if len(pages) <= 1 {
		if pageList, err := b.fetchPageList(bvid); err == nil && len(pageList) > 0 {
			pages = pageList
		}
	}
	if len(pages) == 0 {
		return nil, errors.New("no video pages found")
	}
	return b.buildSongsFromPages(bvid, rootTitle, viewResp.Data.Owner.Name, viewResp.Data.Pic, pages), nil
}

// ParsePlaylist 解析合集/分P链接
func (b *Bilibili) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	bvidRe := regexp.MustCompile(`(BV\w+)`)
	bvidMatches := bvidRe.FindStringSubmatch(link)
	if len(bvidMatches) >= 2 {
		bvid := bvidMatches[1]
		viewResp, err := b.fetchView(bvid)
		if err != nil {
			return nil, nil, err
		}
		if viewResp.Data.UgcSeason != nil {
			seasonID := viewResp.Data.UgcSeason.ID
			mid := viewResp.Data.Owner.Mid
			trackCount := countSeasonEpisodes(viewResp.Data.UgcSeason.Sections)
			playlist := &model.Playlist{
				Source:      "bilibili",
				ID:          fmt.Sprintf("season:%d:%d:%s", seasonID, mid, bvid),
				Name:        viewResp.Data.UgcSeason.Title,
				Cover:       normalizeCover(viewResp.Data.UgcSeason.Cover),
				TrackCount:  trackCount,
				PlayCount:   viewResp.Data.UgcSeason.Stat.View,
				Creator:     viewResp.Data.Owner.Name,
				Description: viewResp.Data.UgcSeason.Intro,
				Link:        fmt.Sprintf("https://www.bilibili.com/video/%s", bvid),
				Extra: map[string]string{
					"season_id": strconv.FormatInt(seasonID, 10),
					"mid":       strconv.FormatInt(mid, 10),
					"bvid":      bvid,
					"type":      "season",
				},
			}
			sections := viewResp.Data.UgcSeason.Sections
			archiveIndex := map[string]bilibiliSeasonArchiveMeta{}
			seasonTitle := viewResp.Data.UgcSeason.Title
			seasonCover := viewResp.Data.UgcSeason.Cover
			idx, sTitle, sCover, idxErr := b.fetchSeasonArchiveIndex(mid, seasonID)
			if idxErr == nil {
				archiveIndex = idx
				if seasonTitle == "" {
					seasonTitle = sTitle
				}
				if seasonCover == "" {
					seasonCover = sCover
				}
				if trackCount == 0 {
					trackCount = len(archiveIndex)
					playlist.TrackCount = trackCount
				}
			}
			songs := b.buildSongsFromSeasonSections(sections, seasonTitle, seasonCover, viewResp.Data.Owner.Name, archiveIndex)
			if len(songs) > 0 {
				return playlist, songs, nil
			}
			songs, err := b.fetchSeasonSongs(mid, seasonID)
			return playlist, songs, err
		}

		if len(viewResp.Data.Pages) > 1 {
			playlist := &model.Playlist{
				Source:     "bilibili",
				ID:         "bvid:" + bvid,
				Name:       viewResp.Data.Title,
				Cover:      normalizeCover(viewResp.Data.Pic),
				TrackCount: len(viewResp.Data.Pages),
				Creator:    viewResp.Data.Owner.Name,
				Link:       fmt.Sprintf("https://www.bilibili.com/video/%s", bvid),
				Extra: map[string]string{
					"bvid": bvid,
					"type": "multipart",
				},
			}
			songs := b.buildSongsFromPages(bvid, viewResp.Data.Title, viewResp.Data.Owner.Name, viewResp.Data.Pic, viewResp.Data.Pages)
			return playlist, songs, nil
		}

		if len(viewResp.Data.Pages) == 1 {
			playlist := &model.Playlist{
				Source:     "bilibili",
				ID:         "bvid:" + bvid,
				Name:       viewResp.Data.Title,
				Cover:      normalizeCover(viewResp.Data.Pic),
				TrackCount: 1,
				Creator:    viewResp.Data.Owner.Name,
				Link:       fmt.Sprintf("https://www.bilibili.com/video/%s", bvid),
				Extra: map[string]string{
					"bvid": bvid,
					"type": "single",
				},
			}
			songs := b.buildSongsFromPages(bvid, viewResp.Data.Title, viewResp.Data.Owner.Name, viewResp.Data.Pic, viewResp.Data.Pages)
			return playlist, songs, nil
		}
	}

	return nil, nil, errors.New("invalid bilibili playlist link")
}
