package bilibili

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"strconv"
	"strings"
)

const (
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 Edg/121.0.0.0"
	Referer   = "https://www.bilibili.com/"
)

// Bilibili 结构体
type Bilibili struct {
	cookie     string
	isVipCache *bool
}

// New 初始化函数
func New(cookie string) *Bilibili {
	if cookie == "" {
		cookie = "buvid3=2E109C72-251F-3827-FA8E-921FA0D7EC5291319infoc; SESSDATA=your_sessdata;"
	}
	return &Bilibili{
		cookie: cookie,
	}
}

var defaultBilibili = New("buvid3=2E109C72-251F-3827-FA8E-921FA0D7EC5291319infoc; SESSDATA=your_sessdata;")

type bilibiliViewResponse struct {
	Data struct {
		BVID  string `json:"bvid"`
		Title string `json:"title"`
		Pic   string `json:"pic"`
		Owner struct {
			Name string `json:"name"`
			Mid  int64  `json:"mid"`
		} `json:"owner"`
		Pages     []bilibiliPage `json:"pages"`
		UgcSeason *struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
			Cover string `json:"cover"`
			Intro string `json:"intro"`
			Stat  struct {
				View int `json:"view"`
			} `json:"stat"`
			Sections []bilibiliSeasonSection `json:"sections"`
		} `json:"ugc_season"`
	} `json:"data"`
}

type bilibiliPage struct {
	CID      int64  `json:"cid"`
	Part     string `json:"part"`
	Duration int    `json:"duration"`
}

type bilibiliSeasonResp struct {
	Code int `json:"code"`
	Data struct {
		Season struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
			Cover string `json:"cover"`
			Intro string `json:"intro"`
		} `json:"season"`
		Page struct {
			Total int `json:"total"`
		} `json:"page"`
		Archives []struct {
			BVID     string `json:"bvid"`
			Title    string `json:"title"`
			Cover    string `json:"cover"`
			Duration int    `json:"duration"`
			CID      int64  `json:"cid"`
		} `json:"archives"`
	} `json:"data"`
}

type bilibiliPageListResp struct {
	Code int            `json:"code"`
	Data []bilibiliPage `json:"data"`
}

func cleanTitle(t string) string {
	if t == "" {
		return ""
	}
	t = strings.ReplaceAll(t, "<em class=\"keyword\">", "")
	t = strings.ReplaceAll(t, "</em>", "")
	return t
}

type bilibiliSeasonSection struct {
	Episodes []struct {
		BVID     string `json:"bvid"`
		CID      int64  `json:"cid"`
		Title    string `json:"title"`
		Cover    string `json:"cover"`
		Duration int    `json:"duration"`
		Arc      struct {
			Pic      string `json:"pic"`
			Title    string `json:"title"`
			Duration int    `json:"duration"`
		} `json:"arc"`
		Page struct {
			Part     string `json:"part"`
			Duration int    `json:"duration"`
		} `json:"page"`
	} `json:"episodes"`
}

type bilibiliSeasonArchiveMeta struct {
	Title    string
	Cover    string
	Duration int
}

func countSeasonEpisodes(sections []bilibiliSeasonSection) int {
	count := 0
	for _, sec := range sections {
		count += len(sec.Episodes)
	}
	return count
}

func (b *Bilibili) buildSongsFromSeasonSections(sections []bilibiliSeasonSection, seasonTitle, seasonCover, artistName string, archiveIndex map[string]bilibiliSeasonArchiveMeta) []model.Song {
	var songs []model.Song
	if artistName == "" {
		artistName = seasonTitle
	}
	for _, sec := range sections {
		for _, ep := range sec.Episodes {
			cover := ep.Cover
			meta, hasMeta := archiveIndex["bvid:"+ep.BVID]
			if !hasMeta && ep.CID != 0 {
				meta, hasMeta = archiveIndex["cid:"+strconv.FormatInt(ep.CID, 10)]
			}
			if cover == "" {
				cover = ep.Arc.Pic
			}
			if cover == "" {
				cover = seasonCover
				if cover == "" && hasMeta {
					cover = meta.Cover
				}
			}
			cover = normalizeCover(cover)
			duration := ep.Duration
			if duration == 0 {
				duration = ep.Page.Duration
			}
			if duration == 0 {
				duration = ep.Arc.Duration
			}
			if duration == 0 && hasMeta {
				duration = meta.Duration
			}
			name := ep.Title
			if name == "" {
				name = ep.Arc.Title
			}
			if name == "" {
				name = ep.Page.Part
			}
			if name == "" && hasMeta {
				name = meta.Title
			}
			songs = append(songs, model.Song{
				Source:   "bilibili",
				ID:       fmt.Sprintf("%s|%d", ep.BVID, ep.CID),
				Name:     name,
				Artist:   artistName,
				Album:    ep.BVID,
				Duration: duration,
				Cover:    cover,
				Link:     fmt.Sprintf("https://www.bilibili.com/video/%s", ep.BVID),
				Extra: map[string]string{
					"bvid": ep.BVID,
					"cid":  strconv.FormatInt(ep.CID, 10),
				},
			})
		}
	}
	return songs
}

func findPageDuration(pages []bilibiliPage, cid int64) int {
	if cid == 0 {
		return 0
	}
	for _, p := range pages {
		if p.CID == cid {
			return p.Duration
		}
	}
	return 0
}

func needsSeasonArchiveFallback(sections []bilibiliSeasonSection) bool {
	for _, sec := range sections {
		for _, ep := range sec.Episodes {
			if ep.Duration == 0 || ep.Cover == "" || ep.Title == "" {
				return true
			}
		}
	}
	return false
}

func (b *Bilibili) fetchSeasonArchiveIndex(mid, seasonID int64) (map[string]bilibiliSeasonArchiveMeta, string, string, error) {
	if seasonID == 0 || mid == 0 {
		return nil, "", "", errors.New("invalid season info")
	}

	index := make(map[string]bilibiliSeasonArchiveMeta)
	processedArchives := 0
	var seasonTitle string
	var seasonCover string
	pageNum := 1
	pageSize := 30
	for {
		apiURL := fmt.Sprintf("https://api.bilibili.com/x/space/ugc/season?mid=%d&season_id=%d&page_num=%d&page_size=%d", mid, seasonID, pageNum, pageSize)
		body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
		if err != nil {
			return nil, "", "", err
		}

		var resp bilibiliSeasonResp
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, "", "", err
		}
		if resp.Code != 0 {
			return nil, "", "", fmt.Errorf("bilibili season api error: %d", resp.Code)
		}
		if seasonTitle == "" {
			seasonTitle = resp.Data.Season.Title
		}
		if seasonCover == "" {
			seasonCover = resp.Data.Season.Cover
		}
		if len(resp.Data.Archives) == 0 {
			break
		}

		for _, arc := range resp.Data.Archives {
			meta := bilibiliSeasonArchiveMeta{
				Title:    arc.Title,
				Cover:    normalizeCover(arc.Cover),
				Duration: arc.Duration,
			}
			if arc.BVID != "" {
				index["bvid:"+arc.BVID] = meta
			}
			if arc.CID != 0 {
				index["cid:"+strconv.FormatInt(arc.CID, 10)] = meta
			}
		}
		processedArchives += len(resp.Data.Archives)

		total := resp.Data.Page.Total
		if total == 0 || processedArchives >= total {
			break
		}
		pageNum++
	}
	return index, seasonTitle, seasonCover, nil
}

func normalizeCover(cover string) string {
	if strings.HasPrefix(cover, "//") {
		return "https:" + cover
	}
	return cover
}

func (b *Bilibili) fetchView(bvid string) (*bilibiliViewResponse, error) {
	viewURL := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", bvid)
	viewBody, err := utils.Get(viewURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return nil, err
	}

	var viewResp bilibiliViewResponse
	if err := json.Unmarshal(viewBody, &viewResp); err != nil {
		return nil, err
	}
	return &viewResp, nil
}

func (b *Bilibili) fetchPageList(bvid string) ([]bilibiliPage, error) {
	pageURL := fmt.Sprintf("https://api.bilibili.com/x/player/pagelist?bvid=%s", bvid)
	body, err := utils.Get(pageURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return nil, err
	}

	var resp bilibiliPageListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("bilibili pagelist api error: %d", resp.Code)
	}
	return resp.Data, nil
}

func (b *Bilibili) buildSongsFromPages(bvid, rootTitle, author, cover string, pages []bilibiliPage) []model.Song {
	var songs []model.Song
	cover = normalizeCover(cover)

	for i, page := range pages {
		displayTitle := page.Part
		if len(pages) == 1 && displayTitle == "" {
			displayTitle = rootTitle
		} else if displayTitle != rootTitle {
			displayTitle = fmt.Sprintf("%s - %s", rootTitle, displayTitle)
		}

		songs = append(songs, model.Song{
			Source:   "bilibili",
			ID:       fmt.Sprintf("%s|%d", bvid, page.CID),
			Name:     displayTitle,
			Artist:   author,
			Album:    bvid,
			Duration: page.Duration,
			Cover:    cover,
			Link:     fmt.Sprintf("https://www.bilibili.com/video/%s?p=%d", bvid, i+1),
			Extra: map[string]string{
				"bvid": bvid,
				"cid":  strconv.FormatInt(page.CID, 10),
			},
		})
	}
	return songs
}

func (b *Bilibili) fetchSeasonSongs(mid, seasonID int64) ([]model.Song, error) {
	if seasonID == 0 || mid == 0 {
		return nil, errors.New("invalid season info")
	}

	var allSongs []model.Song
	processedArchives := 0
	var seasonTitle string
	var seasonCover string
	pageNum := 1
	pageSize := 30
	for {
		apiURL := fmt.Sprintf("https://api.bilibili.com/x/space/ugc/season?mid=%d&season_id=%d&page_num=%d&page_size=%d", mid, seasonID, pageNum, pageSize)
		body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
		if err != nil {
			return nil, err
		}

		var resp bilibiliSeasonResp
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			return nil, fmt.Errorf("bilibili season api error: %d", resp.Code)
		}
		if seasonTitle == "" {
			seasonTitle = resp.Data.Season.Title
		}
		if seasonCover == "" {
			seasonCover = resp.Data.Season.Cover
		}
		if len(resp.Data.Archives) == 0 {
			break
		}

		for _, arc := range resp.Data.Archives {
			if arc.CID != 0 {
				cover := arc.Cover
				if cover == "" {
					cover = seasonCover
				}
				allSongs = append(allSongs, model.Song{
					Source:   "bilibili",
					ID:       fmt.Sprintf("%s|%d", arc.BVID, arc.CID),
					Name:     arc.Title,
					Artist:   "",
					Album:    seasonTitle,
					Duration: arc.Duration,
					Cover:    normalizeCover(cover),
					Link:     fmt.Sprintf("https://www.bilibili.com/video/%s", arc.BVID),
					Extra: map[string]string{
						"bvid": arc.BVID,
						"cid":  strconv.FormatInt(arc.CID, 10),
					},
				})
				continue
			}
		}
		processedArchives += len(resp.Data.Archives)

		total := resp.Data.Page.Total
		if total == 0 || processedArchives >= total {
			break
		}
		pageNum++
	}
	return allSongs, nil
}

// fetchAudioURL 内部逻辑提取
func (b *Bilibili) fetchAudioURL(bvid, cid string, isVip bool) (string, error) {
	fnval := 80
	if isVip {
		// 4048 allows requesting FLAC/Hi-Res/Dolby formats instead of default standard 80
		fnval = 4048
	}

	apiURL := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?fnval=%d&qn=127&bvid=%s&cid=%s", fnval, bvid, cid)
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return "", err
	}

	var resp struct {
		Data struct {
			Durl []struct {
				URL string `json:"url"`
			} `json:"durl"`
			Dash struct {
				Audio []struct {
					ID      int    `json:"id"`
					BaseURL string `json:"baseUrl"`
				} `json:"audio"`
				Flac struct {
					Audio struct {
						ID      int    `json:"id"`
						BaseURL string `json:"baseUrl"`
					} `json:"audio"`
				} `json:"flac"`
				Dolby struct {
					Audio []struct {
						ID      int    `json:"id"`
						BaseURL string `json:"baseUrl"`
					} `json:"audio"`
				} `json:"dolby"`
			} `json:"dash"`
		} `json:"data"`
	}
	json.Unmarshal(body, &resp)

	// 1. Highest Priority: Hi-Res FLAC format specifically marked by id 30251
	if resp.Data.Dash.Flac.Audio.ID == 30251 && resp.Data.Dash.Flac.Audio.BaseURL != "" {
		return resp.Data.Dash.Flac.Audio.BaseURL, nil
	}

	// 2. Secondary Priority: Dolby Atmos format specifically marked by id 30250
	for _, a := range resp.Data.Dash.Dolby.Audio {
		if a.ID == 30250 && a.BaseURL != "" {
			return a.BaseURL, nil
		}
	}

	// 3. Loop through standard DASH audio evaluating the highest ID quality natively (e.g. 30280 for 192kbps vs 30232 for 132kbps)
	var bestURL string
	var highestID = -1
	for _, a := range resp.Data.Dash.Audio {
		if a.ID > highestID && a.BaseURL != "" {
			highestID = a.ID
			bestURL = a.BaseURL
		}
	}
	if bestURL != "" {
		return bestURL, nil
	}

	// 4. Fallback to generic unsegmented DURL format if no Dash
	if len(resp.Data.Durl) > 0 {
		return resp.Data.Durl[0].URL, nil
	}

	return "", errors.New("no audio found")
}
