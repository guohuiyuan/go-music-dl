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

func Search(keyword string) ([]model.Song, error) { return defaultBilibili.Search(keyword) }

func Parse(link string) (*model.Song, error) { return defaultBilibili.Parse(link) }

// Search 搜索歌曲
func (b *Bilibili) Search(keyword string) ([]model.Song, error) {
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

	var songs []model.Song
	for _, item := range searchResp.Data.Result {
		rootTitle := cleanTitle(item.Title)
		viewResp, err := b.fetchView(item.BVID)
		if err != nil || len(viewResp.Data.Pages) == 0 {
			continue
		}

		cover := normalizeCover(item.Pic)
		page := viewResp.Data.Pages[0]
		displayTitle := page.Part
		if displayTitle == "" {
			displayTitle = rootTitle
		} else if displayTitle != rootTitle {
			displayTitle = fmt.Sprintf("%s - %s", rootTitle, displayTitle)
		}

		songs = append(songs, model.Song{
			Source:   "bilibili",
			ID:       fmt.Sprintf("%s|%d", item.BVID, page.CID),
			Name:     displayTitle,
			Artist:   item.Author,
			Album:    item.BVID,
			Duration: page.Duration,
			Cover:    cover,
			Link:     fmt.Sprintf("https://www.bilibili.com/video/%s?p=1", item.BVID),
			Extra: map[string]string{
				"bvid": item.BVID,
				"cid":  strconv.FormatInt(page.CID, 10),
			},
		})
	}
	return songs, nil
}

// Parse 解析链接并获取完整信息（包括下载链接）
func (b *Bilibili) Parse(link string) (*model.Song, error) {
	// 1. 提取 BVID
	bvidRe := regexp.MustCompile(`(BV\w+)`)
	bvidMatches := bvidRe.FindStringSubmatch(link)
	if len(bvidMatches) < 2 {
		return nil, errors.New("invalid bilibili link: bvid not found")
	}
	bvid := bvidMatches[1]

	// 2. 提取 Page (p=X), 默认为 1
	page := 1
	pageRe := regexp.MustCompile(`[?&]p=(\d+)`)
	pageMatches := pageRe.FindStringSubmatch(link)
	if len(pageMatches) >= 2 {
		if p, err := strconv.Atoi(pageMatches[1]); err == nil && p > 0 {
			page = p
		}
	}

	// 3. 调用 View 接口获取元数据
	viewResp, err := b.fetchView(bvid)
	if err != nil {
		return nil, err
	}
	if viewResp.Data.UgcSeason != nil || len(viewResp.Data.Pages) > 1 {
		return nil, errors.New("playlist link detected")
	}
	if len(viewResp.Data.Pages) <= 1 {
		if pages, err := b.fetchPageList(bvid); err == nil && len(pages) > 1 {
			return nil, errors.New("playlist link detected")
		}
	}
	if len(viewResp.Data.Pages) == 0 {
		return nil, errors.New("no video pages found")
	}

	if page > len(viewResp.Data.Pages) {
		page = 1
	}
	targetPage := viewResp.Data.Pages[page-1]

	displayTitle := targetPage.Part
	if len(viewResp.Data.Pages) == 1 && displayTitle == "" {
		displayTitle = viewResp.Data.Title
	} else if displayTitle != viewResp.Data.Title {
		displayTitle = fmt.Sprintf("%s - %s", viewResp.Data.Title, displayTitle)
	}

	cover := viewResp.Data.Pic
	if strings.HasPrefix(cover, "//") {
		cover = "https:" + cover
	}

	cidStr := strconv.FormatInt(targetPage.CID, 10)

	// 4. 立即获取下载链接
	isVip, _ := b.IsVipAccount()
	audioURL, _ := b.fetchAudioURL(bvid, cidStr, isVip) // 忽略错误，尽可能返回元数据

	return &model.Song{
		Source:   "bilibili",
		ID:       fmt.Sprintf("%s|%d", viewResp.Data.BVID, targetPage.CID),
		Name:     displayTitle,
		Artist:   viewResp.Data.Owner.Name,
		Album:    viewResp.Data.BVID,
		Duration: targetPage.Duration,
		Cover:    cover,
		URL:      audioURL, // 已填充
		Link:     fmt.Sprintf("https://www.bilibili.com/video/%s?p=%d", viewResp.Data.BVID, page),
		Extra: map[string]string{
			"bvid": viewResp.Data.BVID,
			"cid":  cidStr,
		},
	}, nil
}
