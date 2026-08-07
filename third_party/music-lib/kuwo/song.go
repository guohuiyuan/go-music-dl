package kuwo

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

func Search(keyword string) ([]model.Song, error) { return defaultKuwo.Search(keyword) }

func Parse(link string) (*model.Song, error) { return defaultKuwo.Parse(link) }

// Search 搜索歌曲
func (k *Kuwo) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("vipver", "1")
	params.Set("client", "kt")
	params.Set("ft", "music")
	params.Set("cluster", "0")
	params.Set("strategy", "2012")
	params.Set("encoding", "utf8")
	params.Set("rformat", "json")
	params.Set("mobi", "1")
	params.Set("issubtitle", "1")
	params.Set("show_copyright_off", "1")
	params.Set("pn", "0")
	params.Set("rn", "10")
	params.Set("all", keyword)

	apiURL := "http://www.kuwo.cn/search/searchMusicBykeyWord?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		AbsList []struct {
			MusicRID  string `json:"MUSICRID"`
			SongName  string `json:"SONGNAME"`
			Artist    string `json:"ARTIST"`
			Album     string `json:"ALBUM"`
			Duration  string `json:"DURATION"`
			HtsMVPic  string `json:"hts_MVPIC"`
			MInfo     string `json:"MINFO"`
			PayInfo   string `json:"PAY"`
			BitSwitch int    `json:"bitSwitch"`
		} `json:"abslist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kuwo json parse error: %w", err)
	}

	var songs []model.Song
	for _, item := range resp.AbsList {
		if item.BitSwitch == 0 {
			continue
		}

		cleanID := strings.TrimPrefix(item.MusicRID, "MUSIC_")
		duration, _ := strconv.Atoi(item.Duration)
		size := parseSizeFromMInfo(item.MInfo)
		bitrate := parseBitrateFromMInfo(item.MInfo)

		songs = append(songs, model.Song{
			Source:   "kuwo",
			ID:       cleanID,
			Name:     item.SongName,
			Artist:   item.Artist,
			Album:    item.Album,
			Duration: duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    item.HtsMVPic,
			Link:     fmt.Sprintf("http://www.kuwo.cn/play_detail/%s", cleanID),
			Extra: map[string]string{
				"rid": cleanID,
			},
		})
	}

	return songs, nil
}

func (k *Kuwo) Parse(link string) (*model.Song, error) {
	re := regexp.MustCompile(`play_detail/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid kuwo link, rid not found")
	}
	rid := matches[1]

	return k.fetchFullSongInfo(rid)
}
