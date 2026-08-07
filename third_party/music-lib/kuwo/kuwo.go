package kuwo

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

var (
	kuwoNewLyricKey    = []byte("yeelion")
	kuwoNewLyricLineRe = regexp.MustCompile(`^\[(\d{2}):(\d{2})\.(\d{3})\](.*)$`)
	kuwoNewLyricTagRe  = regexp.MustCompile(`^\[[A-Za-z]+:[^\]]*\]$`)
	kuwoNewLyricWordRe = regexp.MustCompile(`<(-?\d+),(-?\d+)>([^<]*)`)
)

type Kuwo struct {
	cookie string
}

func New(cookie string) *Kuwo { return &Kuwo{cookie: cookie} }

var defaultKuwo = New("")

// 酷我的歌单和专辑搜索共用同一个 legacy 路由，仅通过 ft 参数区分类型。
func (k *Kuwo) searchCollection(keyword, ft string, out interface{}) error {
	params := url.Values{}
	params.Set("all", keyword)
	params.Set("ft", ft)
	params.Set("itemset", "web_2013")
	params.Set("client", "kt")
	params.Set("pcmp4", "1")
	params.Set("geo", "c")
	params.Set("vipver", "1")
	params.Set("pn", "0")
	params.Set("rn", "10")
	params.Set("rformat", "json")
	params.Set("encoding", "utf8")

	apiURL := "http://search.kuwo.cn/r.s?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return err
	}

	if err := parseKuwoLegacyJSON(body, out); err != nil {
		return fmt.Errorf("kuwo %s json parse error: %w", ft, err)
	}

	return nil
}

// fetchPlaylistDetail [内部复用] 获取歌单详情 (Metadata + Songs)
func (k *Kuwo) fetchPlaylistDetail(id string) (*model.Playlist, []model.Song, error) {
	params := url.Values{}
	params.Set("op", "getlistinfo")
	params.Set("pid", id)
	params.Set("pn", "0")
	params.Set("rn", "100")
	params.Set("encode", "utf8")
	params.Set("keyset", "pl2012")
	params.Set("identity", "kuwo")
	params.Set("pcmp4", "1")
	params.Set("vipver", "1")
	params.Set("newver", "1")

	apiURL := "http://nplserver.kuwo.cn/pl.svc?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		MusicList []struct {
			Id         string      `json:"id"`
			Name       string      `json:"name"`
			Artist     string      `json:"artist"`
			Album      string      `json:"album"`
			AlbumPic   string      `json:"albumpic"`
			Duration   interface{} `json:"duration"`
			SongName   string      `json:"song_name"`
			ArtistName string      `json:"artist_name"`
		} `json:"musiclist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("kuwo playlist detail json error: %w", err)
	}

	if len(resp.MusicList) == 0 {
		return nil, nil, errors.New("playlist is empty or id is invalid")
	}

	playlist := &model.Playlist{
		Source:     "kuwo",
		ID:         id,
		Link:       fmt.Sprintf("http://www.kuwo.cn/playlist_detail/%s", id),
		TrackCount: len(resp.MusicList),
	}

	var songs []model.Song
	for _, item := range resp.MusicList {
		name := item.Name
		if name == "" {
			name = item.SongName
		}
		artist := item.Artist
		if artist == "" {
			artist = item.ArtistName
		}

		var duration int
		switch v := item.Duration.(type) {
		case string:
			d, _ := strconv.Atoi(v)
			duration = d
		case float64:
			duration = int(v)
		}

		cover := item.AlbumPic
		if cover != "" {
			if !strings.HasPrefix(cover, "http") {
				cover = "http://" + cover
			}
			if strings.Contains(cover, "_100.") {
				cover = strings.Replace(cover, "_100.", "_500.", 1)
			} else if strings.Contains(cover, "_150.") {
				cover = strings.Replace(cover, "_150.", "_500.", 1)
			} else if strings.Contains(cover, "_120.") {
				cover = strings.Replace(cover, "_120.", "_500.", 1)
			}
		}

		songs = append(songs, model.Song{
			Source:   "kuwo",
			ID:       item.Id,
			Name:     name,
			Artist:   artist,
			Album:    item.Album,
			Duration: duration,
			Cover:    cover,
			Link:     fmt.Sprintf("http://www.kuwo.cn/play_detail/%s", item.Id),
			Extra: map[string]string{
				"rid": item.Id,
			},
		})
	}
	return playlist, songs, nil
}

// Parse 解析链接并获取完整信息
func (k *Kuwo) fetchAlbumDetail(id string) (*model.Playlist, []model.Song, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil, errors.New("album id is empty")
	}

	album, songs, err := k.fetchAlbumDetailFromLegacyAPI(id)
	if err == nil && len(songs) > 0 {
		return album, songs, nil
	}

	pageAlbum, pageSongs, pageErr := k.fetchAlbumDetailFromPage(id)
	if pageErr == nil && len(pageSongs) > 0 {
		return pageAlbum, pageSongs, nil
	}

	if err != nil && pageErr != nil {
		return nil, nil, fmt.Errorf("kuwo album detail failed: legacy api: %v; page parse: %w", err, pageErr)
	}
	if err != nil {
		return nil, nil, err
	}
	if pageErr != nil {
		return nil, nil, pageErr
	}

	return album, songs, nil
}

func (k *Kuwo) fetchAlbumDetailFromLegacyAPI(id string) (*model.Playlist, []model.Song, error) {
	const pageSize = 100

	var album *model.Playlist
	totalSongs := 0
	seen := make(map[string]struct{})
	songs := make([]model.Song, 0, pageSize)

	for page := 0; ; page++ {
		apiURL := kuwoAlbumDetailURL(id, page, pageSize)

		body, err := utils.Get(apiURL,
			utils.WithHeader("User-Agent", UserAgent),
			utils.WithHeader("Cookie", k.cookie),
			utils.WithRandomIPHeader(),
		)
		if err != nil {
			return nil, nil, err
		}

		var resp map[string]interface{}
		if err := parseKuwoLegacyJSON(body, &resp); err != nil {
			return nil, nil, fmt.Errorf("kuwo album detail json error: %w", err)
		}

		if album == nil {
			albumID := firstNonEmpty(parseKuwoAnyString(resp["albumid"]), parseKuwoAnyString(resp["id"]), id)
			totalSongs = parseKuwoAnyInt(resp["songnum"])
			album = &model.Playlist{
				Source:      "kuwo",
				ID:          albumID,
				Name:        normalizeKuwoText(parseKuwoAnyString(resp["name"])),
				Cover:       normalizeKuwoImageURL(firstNonEmpty(parseKuwoAnyString(resp["hts_img"]), parseKuwoAnyString(resp["img"]))),
				TrackCount:  totalSongs,
				Creator:     normalizeKuwoText(firstNonEmpty(parseKuwoAnyString(resp["aartist"]), parseKuwoAnyString(resp["artist"]))),
				Description: normalizeKuwoText(parseKuwoAnyString(resp["info"])),
				Link:        fmt.Sprintf("http://www.kuwo.cn/album_detail/%s", albumID),
				Extra: map[string]string{
					"type":         "album",
					"album_id":     albumID,
					"company":      normalizeKuwoText(parseKuwoAnyString(resp["company"])),
					"publish_time": strings.TrimSpace(parseKuwoAnyString(resp["pub"])),
					"lang":         normalizeKuwoText(parseKuwoAnyString(resp["lang"])),
				},
			}
		}

		musicList := parseKuwoAnySlice(resp["musiclist"])
		if len(musicList) == 0 {
			if page == 0 {
				return nil, nil, fmt.Errorf("album %s detail api returned empty musiclist", id)
			}
			break
		}

		for _, rawItem := range musicList {
			item, ok := rawItem.(map[string]interface{})
			if !ok {
				continue
			}

			rid := firstNonEmpty(parseKuwoAnyString(item["id"]), parseKuwoAnyString(item["musicrid"]))
			if rid == "" {
				continue
			}
			if _, ok := seen[rid]; ok {
				continue
			}
			seen[rid] = struct{}{}

			songCover := normalizeKuwoImageURL(firstNonEmpty(parseKuwoAnyString(item["pic120"]), parseKuwoAnyString(item["web_albumpic_short"])))
			if songCover == "" && album != nil {
				songCover = album.Cover
			}

			song := model.Song{
				Source:   "kuwo",
				ID:       rid,
				Name:     normalizeKuwoText(firstNonEmpty(parseKuwoAnyString(item["name"]), parseKuwoAnyString(item["songname"]))),
				Artist:   normalizeKuwoText(firstNonEmpty(parseKuwoAnyString(item["aartist"]), parseKuwoAnyString(item["artist"]))),
				Album:    normalizeKuwoText(firstNonEmpty(parseKuwoAnyString(item["album"]), album.Name)),
				Duration: parseKuwoAnyInt(item["duration"]),
				Size:     parseSizeFromMInfo(parseKuwoAnyString(item["MINFO"])),
				Bitrate:  parseBitrateFromMInfo(parseKuwoAnyString(item["MINFO"])),
				Cover:    songCover,
				Link:     fmt.Sprintf("http://www.kuwo.cn/play_detail/%s", rid),
				Extra: map[string]string{
					"rid": rid,
				},
			}

			if album != nil {
				song.AlbumID = album.ID
				song.Extra["album_id"] = album.ID
			}
			if track := strings.TrimSpace(parseKuwoAnyString(item["track"])); track != "" {
				song.Extra["track"] = track
			}
			if subtitle := normalizeKuwoText(parseKuwoAnyString(item["subtitle"])); subtitle != "" {
				song.Extra["subtitle"] = subtitle
			}
			if bitSwitch := parseKuwoAnyInt(item["bitSwitch"]); bitSwitch > 0 {
				song.Extra["bit_switch"] = strconv.Itoa(bitSwitch)
			}

			songs = append(songs, song)
		}

		if len(musicList) < pageSize {
			break
		}
		if totalSongs > 0 && len(songs) >= totalSongs {
			break
		}
	}

	if album == nil {
		return nil, nil, errors.New("album not found")
	}
	if album.TrackCount == 0 {
		album.TrackCount = len(songs)
	}

	return album, songs, nil
}

func kuwoAlbumDetailURL(id string, page int, pageSize int) string {
	params := []struct {
		key   string
		value string
	}{
		{"pn", strconv.Itoa(page)},
		{"rn", strconv.Itoa(pageSize)},
		{"stype", "albuminfo"},
		{"albumid", id},
		{"sortby", "0"},
		{"alflac", "1"},
		{"show_copyright_off", "1"},
		{"pcmp4", "1"},
		{"encoding", "utf8"},
	}
	parts := make([]string, 0, len(params))
	for _, param := range params {
		parts = append(parts, url.QueryEscape(param.key)+"="+url.QueryEscape(param.value))
	}
	return "http://search.kuwo.cn/r.s?" + strings.Join(parts, "&")
}

func (k *Kuwo) fetchAlbumDetailFromPage(id string) (*model.Playlist, []model.Song, error) {
	pageURL := fmt.Sprintf("https://www.kuwo.cn/album_detail/%s", id)

	body, err := utils.Get(pageURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
	)
	if err != nil {
		return nil, nil, err
	}

	htmlBody := string(body)
	albumName, artistName := parseKuwoAlbumPageTitle(findKuwoSubmatch(htmlBody, `(?is)<title>(.*?)</title>`))
	if albumName == "" {
		albumName = normalizeKuwoText(stripKuwoHTMLTags(findKuwoSubmatch(htmlBody, `(?is)<p class="song_name"[^>]*>(.*?)</p>`)))
	}
	if artistName == "" {
		artistName = normalizeKuwoText(stripKuwoHTMLTags(findKuwoSubmatch(htmlBody, `(?is)<p class="artist_name"[^>]*>(.*?)</p>`)))
	}

	infoBlock := findKuwoSubmatch(htmlBody, `(?is)<p class="song_info"[^>]*>(.*?)</p>`)
	infoTips := findKuwoSubmatches(infoBlock, `(?is)<span class="tip"[^>]*>(.*?)</span>`)
	lang := ""
	publishTime := ""
	if len(infoTips) > 0 {
		lang = normalizeKuwoText(stripKuwoHTMLTags(infoTips[0]))
	}
	if len(infoTips) > 1 {
		publishTime = normalizeKuwoText(stripKuwoHTMLTags(infoTips[1]))
	}

	description := normalizeKuwoText(stripKuwoHTMLTags(findKuwoSubmatch(htmlBody, `(?is)<p class="intr_txt"[^>]*>.*?<span[^>]*>(.*?)</span>`)))
	cover := normalizeKuwoImageURL(decodeKuwoEscapedString(findKuwoSubmatch(htmlBody, `hts_img:"([^"]+)"`)))
	if cover == "" {
		cover = normalizeKuwoImageURL(decodeKuwoEscapedString(findKuwoSubmatch(htmlBody, `img:"([^"]*albumcover[^"]+)"`)))
	}

	company := normalizeKuwoText(decodeKuwoEscapedString(findKuwoSubmatch(htmlBody, `company:"([^"]+)"`)))
	songBlocks := regexp.MustCompile(`(?is)<li class="song_item[^"]*"[^>]*>.*?</li>`).FindAllString(htmlBody, -1)
	if len(songBlocks) == 0 {
		return nil, nil, errors.New("album page returned no songs")
	}

	album := &model.Playlist{
		Source:      "kuwo",
		ID:          id,
		Name:        albumName,
		Cover:       cover,
		TrackCount:  len(songBlocks),
		Creator:     artistName,
		Description: description,
		Link:        pageURL,
		Extra: map[string]string{
			"type":         "album",
			"album_id":     id,
			"company":      company,
			"publish_time": publishTime,
			"lang":         lang,
		},
	}

	songs := make([]model.Song, 0, len(songBlocks))
	seen := make(map[string]struct{}, len(songBlocks))
	for _, block := range songBlocks {
		rid := findKuwoSubmatch(block, `href="/play_detail/(\d+)"`)
		if rid == "" {
			continue
		}
		if _, ok := seen[rid]; ok {
			continue
		}
		seen[rid] = struct{}{}

		name := normalizeKuwoText(stripKuwoHTMLTags(firstNonEmpty(
			findKuwoSubmatch(block, `title="([^"]+)"`),
			findKuwoSubmatch(block, `(?is)<a[^>]*class="name"[^>]*>(.*?)</a>`),
		)))
		artist := normalizeKuwoText(stripKuwoHTMLTags(firstNonEmpty(
			findKuwoSubmatch(block, `(?is)<div class="song_artist"[^>]*>.*?<span[^>]*title="([^"]+)"`),
			findKuwoSubmatch(block, `(?is)<div class="song_artist"[^>]*>.*?<span[^>]*>(.*?)</span>`),
			artistName,
		)))
		track := firstNonEmpty(
			findKuwoSubmatch(block, `(?is)<div class="rank_num"[^>]*>.*?<span style="display:;?"[^>]*>\s*(\d+)\s*</span>`),
			findKuwoSubmatch(block, `(?is)<div class="rank_num"[^>]*>.*?<span[^>]*>\s*(\d+)\s*</span>`),
		)

		song := model.Song{
			Source:   "kuwo",
			ID:       rid,
			Name:     name,
			Artist:   artist,
			Album:    album.Name,
			AlbumID:  album.ID,
			Cover:    album.Cover,
			Link:     fmt.Sprintf("http://www.kuwo.cn/play_detail/%s", rid),
			Duration: 0,
			Extra: map[string]string{
				"rid":      rid,
				"album_id": album.ID,
			},
		}
		if track != "" {
			song.Extra["track"] = track
		}

		songs = append(songs, song)
	}

	if len(songs) == 0 {
		return nil, nil, errors.New("album page parsed zero songs")
	}
	album.TrackCount = len(songs)

	return album, songs, nil
}

// fetchFullSongInfo 内部聚合：同时获取元数据和下载链接
func (k *Kuwo) fetchFullSongInfo(rid string) (*model.Song, error) {
	params := url.Values{}
	params.Set("musicId", rid)
	params.Set("httpsStatus", "1")
	metaURL := "http://m.kuwo.cn/newh5/singles/songinfoandlrc?" + params.Encode()

	var name, artist, cover string
	metaBody, err := utils.Get(metaURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)

	if err == nil {
		var metaResp struct {
			Data struct {
				SongInfo struct {
					SongName string `json:"songName"`
					Artist   string `json:"artist"`
					Pic      string `json:"pic"`
				} `json:"songinfo"`
			} `json:"data"`
		}
		if json.Unmarshal(metaBody, &metaResp) == nil {
			name = metaResp.Data.SongInfo.SongName
			artist = metaResp.Data.SongInfo.Artist
			cover = metaResp.Data.SongInfo.Pic
		}
	}

	if name == "" {
		name = fmt.Sprintf("Kuwo_Song_%s", rid)
	}

	audioURL, err := k.fetchAudioURL(rid)
	if err != nil {
		return nil, err
	}

	return &model.Song{
		Source: "kuwo",
		ID:     rid,
		Name:   name,
		Artist: artist,
		Cover:  cover,
		URL:    audioURL,
		Link:   fmt.Sprintf("http://www.kuwo.cn/play_detail/%s", rid),
		Extra: map[string]string{
			"rid": rid,
		},
	}, nil
}

// fetchAudioURL 内部核心：仅获取下载链接
func (k *Kuwo) fetchAudioURL(rid string) (string, error) {
	qualities := []string{"128kmp3", "320kmp3", "flac", "2000kflac"}
	randomID := fmt.Sprintf("C_APK_guanwang_%d%d", time.Now().UnixNano(), rand.Intn(1000000))

	for _, br := range qualities {
		params := url.Values{}
		params.Set("f", "web")
		params.Set("source", "kwplayercar_ar_6.0.0.9_B_jiakong_vh.apk")
		params.Set("from", "PC")
		params.Set("type", "convert_url_with_sign")
		params.Set("br", br)
		params.Set("rid", rid)
		params.Set("user", randomID)

		apiURL := "https://mobi.kuwo.cn/mobi.s?" + params.Encode()

		body, err := utils.Get(apiURL,
			utils.WithHeader("User-Agent", UserAgent),
			utils.WithHeader("Cookie", k.cookie),
			utils.WithRandomIPHeader(),
		)
		if err != nil {
			continue
		}

		var resp struct {
			Data struct {
				URL     string `json:"url"`
				Bitrate int    `json:"bitrate"`
				Format  string `json:"format"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}
		if resp.Data.URL != "" {
			return resp.Data.URL, nil
		}
	}

	// [降级策略] 尝试使用 www.kuwo.cn 的备用接口绕过防盗链
	fallbackURL := fmt.Sprintf("http://www.kuwo.cn/api/v1/www/music/playUrl?mid=%s&type=music&httpsStatus=1", rid)

	// 需要伪造 Secret 头部 (简单绕过)
	secret := "kuwo_web_secret"
	cookieWithSecret := k.cookie
	if !strings.Contains(cookieWithSecret, "kw_token") {
		cookieWithSecret += "; kw_token=secret_token"
	}

	fallbackBody, err := utils.Get(fallbackURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", cookieWithSecret),
		utils.WithHeader("Secret", secret), // Web API 需要的签名头
		utils.WithRandomIPHeader(),
	)
	if err == nil {
		var resp struct {
			Data struct {
				Url string `json:"url"`
			} `json:"data"`
		}
		if json.Unmarshal(fallbackBody, &resp) == nil && resp.Data.Url != "" {
			return resp.Data.Url, nil
		}
	}

	return "", errors.New("download url not found (copyright restricted)")
}

func (k *Kuwo) fetchNewLyrics(rid string) (string, error) {
	apiURL := "http://newlyric.kuwo.cn/newlyric.lrc?" + buildKuwoNewLyricParams(rid, true)
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return "", err
	}
	raw, err := decodeKuwoNewLyric(body, true)
	if err != nil {
		return "", err
	}
	lrc := convertKuwoNewLyric(raw)
	if strings.TrimSpace(lrc) == "" || !hasKuwoTimestampedLyric(lrc) {
		return "", errors.New("kuwo newlyric content is empty")
	}
	return lrc, nil
}

func buildKuwoNewLyricParams(musicID string, lyricx bool) string {
	params := "user=12345,web,web,web&requester=localhost&req=1&rid=MUSIC_" + musicID
	if lyricx {
		params += "&lrcx=1"
	}
	return base64.StdEncoding.EncodeToString(xorKuwoNewLyric([]byte(params)))
}

func decodeKuwoNewLyric(buf []byte, lyricx bool) (string, error) {
	if !bytes.HasPrefix(buf, []byte("tp=content")) {
		return "", errors.New("invalid kuwo newlyric response")
	}
	idx := bytes.Index(buf, []byte("\r\n\r\n"))
	if idx < 0 {
		return "", errors.New("invalid kuwo newlyric payload")
	}
	plain, err := inflateKuwoNewLyric(buf[idx+4:])
	if err != nil {
		return "", err
	}

	if lyricx {
		decoded, err := base64.StdEncoding.DecodeString(compactBase64(string(plain)))
		if err != nil {
			return "", err
		}
		plain = xorKuwoNewLyric(decoded)
	}

	text, err := simplifiedchinese.GB18030.NewDecoder().String(string(plain))
	if err != nil {
		return "", err
	}
	return text, nil
}

func inflateKuwoNewLyric(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func xorKuwoNewLyric(data []byte) []byte {
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ kuwoNewLyricKey[i%len(kuwoNewLyricKey)]
	}
	return out
}

func compactBase64(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, s)
}

func convertKuwoNewLyric(raw string) string {
	lines := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\r' || r == '\n'
	})
	var out []string

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		matches := kuwoNewLyricLineRe.FindStringSubmatch(line)
		if matches == nil {
			if kuwoNewLyricTagRe.MatchString(line) {
				out = append(out, line)
			}
			continue
		}

		payload := matches[4]
		if isKuwoChineseTranslationPayload(payload) {
			continue
		}
		text := strings.TrimSpace(kuwoNewLyricPayloadText(payload))
		if text == "" {
			continue
		}

		timestamp := fmt.Sprintf("[%s:%s.%s]", matches[1], matches[2], matches[3])
		out = append(out, timestamp+text)

		var roma, translation string
		for i+1 < len(lines) {
			nextMatches := kuwoNewLyricLineRe.FindStringSubmatch(strings.TrimSpace(lines[i+1]))
			if nextMatches == nil || !strings.HasPrefix(nextMatches[4], "<0,0>") {
				break
			}
			nextText := strings.TrimSpace(kuwoNewLyricPayloadText(nextMatches[4]))
			i++
			if nextText == "" {
				continue
			}
			switch {
			case isKuwoChineseTranslationPayload(nextMatches[4]) && translation == "":
				translation = nextText
			case isKuwoRomajiPayload(nextText) && roma == "":
				roma = nextText
			}
		}
		if roma != "" {
			out = append(out, timestamp+roma)
		}
		if translation != "" {
			out = append(out, timestamp+translation)
		}
	}

	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func kuwoNewLyricPayloadText(payload string) string {
	matches := kuwoNewLyricWordRe.FindAllStringSubmatch(payload, -1)
	if len(matches) == 0 {
		return strings.TrimSpace(payload)
	}
	var b strings.Builder
	for _, match := range matches {
		b.WriteString(match[3])
	}
	return strings.TrimSpace(b.String())
}

func isKuwoChineseTranslationPayload(payload string) bool {
	if !strings.HasPrefix(payload, "<0,0>") {
		return false
	}
	text := kuwoNewLyricPayloadText(payload)
	return containsHan(text) && !containsKana(text)
}

func isKuwoRomajiPayload(text string) bool {
	hasLatin := false
	for _, r := range text {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			hasLatin = true
			continue
		}
		if containsHan(string(r)) || containsKana(string(r)) {
			return false
		}
	}
	return hasLatin
}

func containsHan(s string) bool {
	for _, r := range s {
		if r >= '\u4e00' && r <= '\u9fff' {
			return true
		}
	}
	return false
}

func containsKana(s string) bool {
	for _, r := range s {
		if (r >= '\u3040' && r <= '\u30ff') || (r >= '\uff66' && r <= '\uff9f') {
			return true
		}
	}
	return false
}

func hasKuwoTimestampedLyric(lrc string) bool {
	for _, line := range strings.Split(lrc, "\n") {
		if kuwoNewLyricLineRe.MatchString(strings.TrimSpace(line)) {
			return true
		}
	}
	return false
}

func parseKuwoLegacyJSON(body []byte, out interface{}) error {
	jsonStr := strings.ReplaceAll(string(body), "'", "\"")
	return json.Unmarshal([]byte(jsonStr), out)
}

func findKuwoSubmatch(input, pattern string) string {
	matches := regexp.MustCompile(pattern).FindStringSubmatch(input)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func findKuwoSubmatches(input, pattern string) []string {
	allMatches := regexp.MustCompile(pattern).FindAllStringSubmatch(input, -1)
	values := make([]string, 0, len(allMatches))
	for _, match := range allMatches {
		if len(match) >= 2 {
			values = append(values, match[1])
		}
	}
	return values
}

func stripKuwoHTMLTags(raw string) string {
	if raw == "" {
		return ""
	}

	replacer := strings.NewReplacer(
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"</p>", "\n",
		"</div>", "\n",
	)
	raw = replacer.Replace(raw)
	raw = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(raw, "")

	return normalizeKuwoText(raw)
}

func decodeKuwoEscapedString(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	decoded, err := strconv.Unquote(`"` + raw + `"`)
	if err != nil {
		raw = strings.ReplaceAll(raw, `\/`, `/`)
		return raw
	}

	return decoded
}

func parseKuwoAlbumPageTitle(title string) (string, string) {
	title = normalizeKuwoText(title)
	if title == "" {
		return "", ""
	}

	parts := strings.Split(title, "_")
	if len(parts) < 2 {
		return "", ""
	}

	name := normalizeKuwoText(strings.TrimSuffix(parts[0], "专辑"))
	artist := normalizeKuwoText(parts[1])

	return name, artist
}

func normalizeKuwoText(value string) string {
	if value == "" {
		return ""
	}

	value = html.UnescapeString(value)
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\\n;", "\n")
	value = strings.ReplaceAll(value, "\\n", "\n")
	value = strings.ReplaceAll(value, "\n;", "\n")
	return strings.TrimSpace(value)
}

func normalizeKuwoImageURL(raw string) string {
	raw = normalizeKuwoText(raw)
	if raw == "" {
		return ""
	}

	if strings.HasPrefix(raw, "//") {
		raw = "http:" + raw
	} else if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		switch {
		case strings.HasPrefix(raw, "img"):
			raw = "http://" + raw
		default:
			raw = "http://img1.kuwo.cn/star/albumcover/" + strings.TrimPrefix(raw, "/")
		}
	}

	replacements := []struct {
		old string
		new string
	}{
		{"/120/", "/500/"},
		{"/150/", "/500/"},
		{"/240/", "/500/"},
		{"_100.", "_500."},
		{"_120.", "_500."},
		{"_150.", "_500."},
		{"_240.", "_500."},
	}
	for _, replacement := range replacements {
		if strings.Contains(raw, replacement.old) {
			raw = strings.Replace(raw, replacement.old, replacement.new, 1)
		}
	}

	return raw
}

func parseKuwoStringInt(value string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(value))
	return n
}

func parseKuwoAnyString(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', 0, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func parseKuwoAnyInt(value interface{}) int {
	switch v := value.(type) {
	case nil:
		return 0
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case string:
		return parseKuwoStringInt(v)
	default:
		return parseKuwoStringInt(fmt.Sprint(v))
	}
}

func parseKuwoAnySlice(value interface{}) []interface{} {
	switch v := value.(type) {
	case nil:
		return nil
	case []interface{}:
		return v
	default:
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func parseSizeFromMInfo(minfo string) int64 {
	if minfo == "" {
		return 0
	}
	type FormatInfo struct {
		Format  string
		Bitrate string
		Size    int64
	}
	var formats []FormatInfo
	parts := strings.Split(minfo, ";")
	for _, part := range parts {
		kv := make(map[string]string)
		attrs := strings.Split(part, ",")
		for _, attr := range attrs {
			pair := strings.Split(attr, ":")
			if len(pair) == 2 {
				kv[pair[0]] = pair[1]
			}
		}
		sizeStr := kv["size"]
		if sizeStr == "" {
			continue
		}
		sizeStr = strings.TrimSuffix(strings.ToLower(sizeStr), "mb")
		sizeMb, _ := strconv.ParseFloat(sizeStr, 64)
		sizeBytes := int64(sizeMb * 1024 * 1024)
		formats = append(formats, FormatInfo{Format: kv["format"], Bitrate: kv["bitrate"], Size: sizeBytes})
	}
	for _, f := range formats {
		if f.Format == "mp3" && f.Bitrate == "128" {
			return f.Size
		}
	}
	for _, f := range formats {
		if f.Format == "mp3" && f.Bitrate == "320" {
			return f.Size
		}
	}
	for _, f := range formats {
		if f.Format == "flac" {
			return f.Size
		}
	}
	for _, f := range formats {
		if f.Format == "flac" && f.Bitrate == "2000" {
			return f.Size
		}
	}
	var maxSize int64
	for _, f := range formats {
		if f.Size > maxSize {
			maxSize = f.Size
		}
	}
	return maxSize
}

func parseBitrateFromMInfo(minfo string) int {
	if minfo == "" {
		return 128
	}
	type FormatInfo struct {
		Format  string
		Bitrate string
		Size    int64
	}
	var formats []FormatInfo
	parts := strings.Split(minfo, ";")
	for _, part := range parts {
		kv := make(map[string]string)
		attrs := strings.Split(part, ",")
		for _, attr := range attrs {
			pair := strings.Split(attr, ":")
			if len(pair) == 2 {
				kv[pair[0]] = pair[1]
			}
		}
		sizeStr := kv["size"]
		if sizeStr == "" {
			continue
		}
		sizeStr = strings.TrimSuffix(strings.ToLower(sizeStr), "mb")
		sizeMb, _ := strconv.ParseFloat(sizeStr, 64)
		sizeBytes := int64(sizeMb * 1024 * 1024)
		formats = append(formats, FormatInfo{Format: kv["format"], Bitrate: kv["bitrate"], Size: sizeBytes})
	}
	toInt := func(s string) int { v, _ := strconv.Atoi(s); return v }
	for _, f := range formats {
		if f.Format == "mp3" && f.Bitrate == "128" {
			return 128
		}
	}
	for _, f := range formats {
		if f.Format == "mp3" && f.Bitrate == "320" {
			return 320
		}
	}
	for _, f := range formats {
		if f.Format == "flac" && f.Bitrate == "2000" {
			if val := toInt(f.Bitrate); val > 0 {
				return val
			}
			return 2000
		}
	}
	for _, f := range formats {
		if f.Format == "flac" {
			if val := toInt(f.Bitrate); val > 0 {
				return val
			}
			return 800
		}
	}
	return 128
}
