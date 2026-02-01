package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
	"github.com/guohuiyuan/music-lib/utils"
)

// --- 常量与样式 ---
const (
	CookieFile = "cookies.json"
	UA_Common  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

var (
	primaryColor   = lipgloss.Color("#874BFD")
	secondaryColor = lipgloss.Color("#7D56F4")
	subtleColor    = lipgloss.Color("#666666")
	redColor       = lipgloss.Color("#FF5555")
	greenColor     = lipgloss.Color("#50FA7B")
	yellowColor    = lipgloss.Color("#F1FA8C")

	// 表格样式
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(secondaryColor).
			Bold(true).
			Padding(0, 1).
			Border(lipgloss.HiddenBorder(), false, false, false, true) // 保持对齐

	rowStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.HiddenBorder(), false, false, false, true) // 占位隐藏边框，确保对齐

	selectedRowStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				Padding(0, 1).
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(primaryColor)

	checkedStyle = lipgloss.NewStyle().Foreground(greenColor).Bold(true)
)

// --- Cookie 管理 ---
type CookieManager struct {
	mu      sync.RWMutex
	cookies map[string]string
}

var cm = &CookieManager{cookies: make(map[string]string)}

func (m *CookieManager) Load() {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := os.ReadFile(CookieFile)
	if err == nil {
		json.Unmarshal(data, &m.cookies)
	}
}

func (m *CookieManager) Get(source string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cookies[source]
}

// --- 工厂函数 ---

func getSearchFunc(source string) func(string) ([]model.Song, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).Search
	case "qq":
		return qq.New(c).Search
	case "kugou":
		return kugou.New(c).Search
	case "kuwo":
		return kuwo.New(c).Search
	case "migu":
		return migu.New(c).Search
	case "soda":
		return soda.New(c).Search
	case "bilibili":
		return bilibili.New(c).Search
	case "fivesing":
		return fivesing.New(c).Search
	case "jamendo":
		return jamendo.New(c).Search
	case "joox":
		return joox.New(c).Search
	case "qianqian":
		return qianqian.New(c).Search
	default:
		return nil
	}
}

func getDownloadFunc(source string) func(*model.Song) (string, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetDownloadURL
	case "qq":
		return qq.New(c).GetDownloadURL
	case "kugou":
		return kugou.New(c).GetDownloadURL
	case "kuwo":
		return kuwo.New(c).GetDownloadURL
	case "migu":
		return migu.New(c).GetDownloadURL
	case "soda":
		return soda.New(c).GetDownloadURL
	case "bilibili":
		return bilibili.New(c).GetDownloadURL
	case "fivesing":
		return fivesing.New(c).GetDownloadURL
	case "jamendo":
		return jamendo.New(c).GetDownloadURL
	case "joox":
		return joox.New(c).GetDownloadURL
	case "qianqian":
		return qianqian.New(c).GetDownloadURL
	default:
		return nil
	}
}

func getLyricFunc(source string) func(*model.Song) (string, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetLyrics
	case "qq":
		return qq.New(c).GetLyrics
	case "kugou":
		return kugou.New(c).GetLyrics
	case "kuwo":
		return kuwo.New(c).GetLyrics
	case "migu":
		return migu.New(c).GetLyrics
	case "soda":
		return soda.New(c).GetLyrics
	case "bilibili":
		return bilibili.New(c).GetLyrics
	case "fivesing":
		return fivesing.New(c).GetLyrics
	case "jamendo":
		return jamendo.New(c).GetLyrics
	case "joox":
		return joox.New(c).GetLyrics
	case "qianqian":
		return qianqian.New(c).GetLyrics
	default:
		return nil
	}
}

// 新增：Parse 工厂函数
func getParseFunc(source string) func(string) (*model.Song, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).Parse
	case "qq":
		return qq.New(c).Parse
	case "kugou":
		return kugou.New(c).Parse
	case "kuwo":
		return kuwo.New(c).Parse
	case "migu":
		return migu.New(c).Parse
	case "soda":
		return soda.New(c).Parse
	case "bilibili":
		return bilibili.New(c).Parse
	case "fivesing":
		return fivesing.New(c).Parse
	case "jamendo":
		return jamendo.New(c).Parse
	default:
		return nil
	}
}

// 新增：歌单搜索工厂
func getPlaylistSearchFunc(source string) func(string) ([]model.Playlist, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).SearchPlaylist
	case "qq":
		return qq.New(c).SearchPlaylist
	case "kugou":
		return kugou.New(c).SearchPlaylist
	case "kuwo":
		return kuwo.New(c).SearchPlaylist
	case "soda":
		return soda.New(c).SearchPlaylist
	case "fivesing":
		return fivesing.New(c).SearchPlaylist
	default:
		return nil
	}
}

// 新增：歌单详情工厂
func getPlaylistDetailFunc(source string) func(string) ([]model.Song, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetPlaylistSongs
	case "qq":
		return qq.New(c).GetPlaylistSongs
	case "kugou":
		return kugou.New(c).GetPlaylistSongs
	case "kuwo":
		return kuwo.New(c).GetPlaylistSongs
	case "soda":
		return soda.New(c).GetPlaylistSongs
	case "fivesing":
		return fivesing.New(c).GetPlaylistSongs
	default:
		return nil
	}
}

// 新增：歌单解析工厂
func getParsePlaylistFunc(source string) func(string) (*model.Playlist, []model.Song, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).ParsePlaylist
	case "qq":
		return qq.New(c).ParsePlaylist
	case "kugou":
		return kugou.New(c).ParsePlaylist
	case "kuwo":
		return kuwo.New(c).ParsePlaylist
	case "soda":
		return soda.New(c).ParsePlaylist
	case "fivesing":
		return fivesing.New(c).ParsePlaylist
	default:
		return nil
	}
}

// 新增：自动检测链接来源
func detectSource(link string) string {
	if strings.Contains(link, "163.com") {
		return "netease"
	}
	if strings.Contains(link, "qq.com") {
		return "qq"
	}
	if strings.Contains(link, "kugou.com") {
		return "kugou"
	}
	if strings.Contains(link, "kuwo.cn") {
		return "kuwo"
	}
	if strings.Contains(link, "migu.cn") {
		return "migu"
	}
	if strings.Contains(link, "bilibili.com") || strings.Contains(link, "b23.tv") {
		return "bilibili"
	}
	if strings.Contains(link, "douyin.com") || strings.Contains(link, "qishui") {
		return "soda"
	}
	if strings.Contains(link, "5sing") {
		return "fivesing"
	}
	if strings.Contains(link, "jamendo.com") {
		return "jamendo"
	}
	return ""
}

// --- 程序状态 ---
type sessionState int

const (
	stateInput          sessionState = iota // 输入搜索词
	stateLoading                            // 搜索中
	stateList                               // 歌曲结果列表 & 选择
	statePlaylistResult                     // 歌单结果列表
	stateDownloading                        // 下载中
)

// --- 主模型 ---
type modelState struct {
	state     sessionState
	textInput textinput.Model // 搜索输入框
	spinner   spinner.Model   // 加载动画
	progress  progress.Model  // 进度条组件

	searchType string           // "song" or "playlist"
	songs      []model.Song     // 歌曲结果
	playlists  []model.Playlist // 歌单结果
	selected   map[int]struct{} // 已选中的索引集合 (多选)
	cursor     int              // 当前光标位置

	// 配置参数
	sources    []string // 指定搜索源
	outDir     string
	withCover  bool
	withLyrics bool

	// 下载队列管理
	downloadQueue []model.Song // 待下载队列
	totalToDl     int          // 总共需要下载的数量
	downloaded    int          // 已完成数量

	err       error
	statusMsg string // 底部状态栏消息

	windowWidth int
}

// 启动 UI 的入口
func StartUI(initialKeyword string, sources []string, outDir string, withCover bool, withLyrics bool) {
	// 1. 加载 Cookies
	cm.Load()

	ti := textinput.New()
	ti.Placeholder = "输入歌名、歌手或粘贴分享链接 (Tab 切换搜歌单)..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(primaryColor)

	prog := progress.New(progress.WithDefaultGradient())

	initialState := stateInput
	if initialKeyword != "" {
		ti.SetValue(initialKeyword)
		initialState = stateLoading
	}

	m := modelState{
		state:      initialState,
		searchType: "song",
		textInput:  ti,
		spinner:    sp,
		progress:   prog,
		selected:   make(map[int]struct{}),
		sources:    sources,
		outDir:     outDir,
		withCover:  withCover,
		withLyrics: withLyrics,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
	}
}

func (m modelState) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, textinput.Blink)
	if m.state == stateLoading {
		cmds = append(cmds, m.spinner.Tick, searchCmd(m.textInput.Value(), m.searchType, m.sources))
	}
	return tea.Batch(cmds...)
}

func (m modelState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.progress.Width = msg.Width - 10
		if m.progress.Width > 50 {
			m.progress.Width = 50
		}
	}

	switch m.state {
	case stateInput:
		return m.updateInput(msg)
	case stateLoading:
		return m.updateLoading(msg)
	case stateList:
		return m.updateList(msg)
	case statePlaylistResult: // 新增
		return m.updatePlaylistResult(msg)
	case stateDownloading:
		return m.updateDownloading(msg)
	}

	return m, nil
}

// --- 1. 输入状态逻辑 ---
func (m modelState) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab: // 切换搜索类型
			if m.searchType == "song" {
				m.searchType = "playlist"
				m.textInput.Placeholder = "输入歌单关键词或粘贴歌单链接..."
			} else {
				m.searchType = "song"
				m.textInput.Placeholder = "输入歌名、歌手或粘贴分享链接 (Tab 切换)..."
			}
		case tea.KeyEnter:
			val := m.textInput.Value()
			if strings.TrimSpace(val) != "" {
				m.state = stateLoading
				// 重新加载 Cookie 以防外部文件变动
				cm.Load()
				// 清空旧数据
				m.songs = nil
				m.playlists = nil
				return m, tea.Batch(m.spinner.Tick, searchCmd(val, m.searchType, m.sources))
			}
		case tea.KeyEsc:
			return m, tea.Quit
		}
	}
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// --- 2. 加载状态逻辑 ---
type searchResultMsg []model.Song
type playlistResultMsg []model.Playlist
type searchErrorMsg error

func (m modelState) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case searchResultMsg:
		m.songs = msg
		m.state = stateList
		m.cursor = 0
		m.selected = make(map[int]struct{})

		// 如果是单曲解析（通常通过 URL），自动选中
		if len(m.songs) == 1 && strings.HasPrefix(m.textInput.Value(), "http") {
			m.selected[0] = struct{}{}
			m.statusMsg = fmt.Sprintf("解析成功: %s。按回车下载。", m.songs[0].Name)
		} else {
			if m.searchType == "playlist" { // 从歌单进入
				m.statusMsg = fmt.Sprintf("歌单解析完成，包含 %d 首歌曲。空格选择，回车下载。", len(m.songs))
			} else {
				m.statusMsg = fmt.Sprintf("找到 %d 首歌曲。空格选择，回车下载。", len(m.songs))
			}
		}
		return m, nil
	case playlistResultMsg:
		m.playlists = msg
		m.state = statePlaylistResult
		m.cursor = 0
		m.statusMsg = fmt.Sprintf("找到 %d 个歌单。回车查看详情。", len(m.playlists))
		return m, textinput.Blink
	case searchErrorMsg:
		m.state = stateInput
		m.statusMsg = fmt.Sprintf("搜索失败: %v", msg)
		return m, textinput.Blink
	}
	return m, nil
}

// --- 3.5 歌单结果逻辑 ---
func (m modelState) updatePlaylistResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.playlists)-1 {
				m.cursor++
			}
		case "q":
			return m, tea.Quit
		case "esc", "b":
			m.state = stateInput
			m.textInput.SetValue("")
			m.textInput.Focus()
			return m, textinput.Blink
		case "enter":
			if len(m.playlists) > 0 {
				target := m.playlists[m.cursor]
				m.state = stateLoading
				m.statusMsg = fmt.Sprintf("正在获取歌单 [%s] 详情...", target.Name)
				return m, tea.Batch(
					m.spinner.Tick,
					fetchPlaylistSongsCmd(target.ID, target.Source),
				)
			}
		}
	}
	return m, nil
}

// --- 3. 列表状态逻辑 ---
func (m modelState) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.songs)-1 {
				m.cursor++
			}
		case " ":
			if _, ok := m.selected[m.cursor]; ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		case "a":
			if len(m.selected) == len(m.songs) && len(m.songs) > 0 {
				m.selected = make(map[int]struct{})
				m.statusMsg = "已取消全部选择"
			} else {
				for i := range m.songs {
					m.selected[i] = struct{}{}
				}
				m.statusMsg = fmt.Sprintf("已选中全部 %d 首歌曲", len(m.songs))
			}
		case "q":
			return m, tea.Quit
		case "esc", "b":
			m.state = stateInput
			m.textInput.SetValue("")
			m.textInput.Focus()
			return m, textinput.Blink
		case "enter":
			if len(m.selected) == 0 {
				m.selected[m.cursor] = struct{}{}
			}

			m.downloadQueue = []model.Song{}
			for idx := range m.selected {
				if idx >= 0 && idx < len(m.songs) {
					m.downloadQueue = append(m.downloadQueue, m.songs[idx])
				}
			}

			m.totalToDl = len(m.downloadQueue)
			m.downloaded = 0
			m.state = stateDownloading
			m.statusMsg = "正在准备下载..."

			return m, tea.Batch(
				m.spinner.Tick,
				downloadNextCmd(m.downloadQueue, m.outDir, m.withCover, m.withLyrics),
			)
		}
	}
	return m, nil
}

// --- 4. 下载状态逻辑 ---
type downloadOneFinishedMsg struct {
	err  error
	song model.Song
}

func (m modelState) updateDownloading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case downloadOneFinishedMsg:
		m.downloaded++

		resultStr := fmt.Sprintf("已完成: %s - %s", msg.song.Artist, msg.song.Name)
		if msg.err != nil {
			resultStr = fmt.Sprintf("❌ 失败: %s - %s (%v)", msg.song.Artist, msg.song.Name, msg.err)
		}
		m.statusMsg = resultStr

		pct := float64(m.downloaded) / float64(m.totalToDl)
		if len(m.downloadQueue) > 0 {
			m.downloadQueue = m.downloadQueue[1:]
		}

		cmds := []tea.Cmd{m.progress.SetPercent(pct)}

		if m.downloaded >= m.totalToDl {
			m.state = stateList
			m.selected = make(map[int]struct{})
			m.statusMsg = fmt.Sprintf("✅ 任务结束，共下载 %d 首歌曲", m.downloaded)
			return m, nil
		}

		cmds = append(cmds, downloadNextCmd(m.downloadQueue, m.outDir, m.withCover, m.withLyrics))
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// --- 辅助命令 ---

// 核心改进：探测歌曲详情（填充大小和码率）
func probeSongDetails(song *model.Song) {
	dlFunc := getDownloadFunc(song.Source)
	if dlFunc == nil {
		return
	}

	urlStr, err := dlFunc(song)
	if err != nil || urlStr == "" {
		return
	}

	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("Range", "bytes=0-1") // 只请求前2字节
	req.Header.Set("User-Agent", UA_Common)
	if song.Source == "bilibili" {
		req.Header.Set("Referer", "https://www.bilibili.com/")
	}
	if song.Source == "migu" {
		req.Header.Set("Referer", "http://music.migu.cn/")
	}
	if song.Source == "qq" {
		req.Header.Set("Referer", "http://y.qq.com")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var size int64
	if resp.StatusCode == 200 || resp.StatusCode == 206 {
		// 优先从 Content-Range 获取总大小
		cr := resp.Header.Get("Content-Range")
		if parts := strings.Split(cr, "/"); len(parts) == 2 {
			fmt.Sscanf(parts[1], "%d", &size)
		}
		// 降级使用 Content-Length
		if size == 0 {
			size = resp.ContentLength
		}
	}

	if size > 0 {
		song.Size = size
		// 计算码率: Size(bytes) * 8 / Duration(seconds) / 1000 = kbps
		if song.Duration > 0 {
			song.Bitrate = int((size * 8) / int64(song.Duration) / 1000)
		}
	}
}

// 批量并发探测
func probeSongsBatch(songs []model.Song) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // 限制并发数为 5

	for i := range songs {
		if songs[i].Size == 0 {
			wg.Add(1)
			go func(s *model.Song) {
				defer wg.Done()
				sem <- struct{}{}        // 获取令牌
				defer func() { <-sem }() // 释放令牌
				probeSongDetails(s)
			}(&songs[i])
		}
	}
	wg.Wait()
}

// 异步搜索/解析命令 (修改版)
func searchCmd(keyword string, searchType string, sources []string) tea.Cmd {
	return func() tea.Msg {
		// 1. 链接解析模式
		if strings.HasPrefix(keyword, "http") {
			src := detectSource(keyword)
			if src == "" {
				return searchErrorMsg(fmt.Errorf("不支持该链接的解析，或无法识别来源"))
			}

			// 优先尝试单曲解析
			parseFn := getParseFunc(src)
			if parseFn != nil {
				if song, err := parseFn(keyword); err == nil {
					probeSongDetails(song)
					return searchResultMsg([]model.Song{*song})
				}
			}

			// 尝试歌单解析
			parsePlFn := getParsePlaylistFunc(src)
			if parsePlFn != nil {
				if _, songs, err := parsePlFn(keyword); err == nil && len(songs) > 0 {
					probeSongsBatch(songs)
					return searchResultMsg(songs)
				}
			}

			return searchErrorMsg(fmt.Errorf("解析失败: 暂不支持 %s 平台的此链接类型或解析出错", src))
		}

		// 2. 关键词搜索模式
		targetSources := sources
		if len(targetSources) == 0 {
			if searchType == "playlist" {
				targetSources = core.GetPlaylistSourceNames()
			} else {
				targetSources = core.GetDefaultSourceNames()
			}
		}

		var wg sync.WaitGroup
		var mu sync.Mutex

		// 2.1 歌单搜索
		if searchType == "playlist" {
			var allPlaylists []model.Playlist
			for _, src := range targetSources {
				fn := getPlaylistSearchFunc(src)
				if fn == nil {
					continue
				}
				wg.Add(1)
				go func(s string, f func(string) ([]model.Playlist, error)) {
					defer wg.Done()
					if res, err := f(keyword); err == nil {
						for i := range res {
							res[i].Source = s
						}
						mu.Lock()
						allPlaylists = append(allPlaylists, res...)
						mu.Unlock()
					}
				}(src, fn)
			}
			wg.Wait()
			if len(allPlaylists) == 0 {
				return searchErrorMsg(fmt.Errorf("未找到歌单"))
			}
			return playlistResultMsg(allPlaylists)
		}

		// 2.2 单曲搜索
		var allSongs []model.Song
		for _, src := range targetSources {
			fn := getSearchFunc(src)
			if fn == nil {
				continue
			}

			wg.Add(1)
			go func(s string, f func(string) ([]model.Song, error)) {
				defer wg.Done()
				res, err := f(keyword)
				if err == nil && len(res) > 0 {
					for i := range res {
						res[i].Source = s
					}
					if len(res) > 10 {
						res = res[:10]
					}
					mu.Lock()
					allSongs = append(allSongs, res...)
					mu.Unlock()
				}
			}(src, fn)
		}
		wg.Wait()

		if len(allSongs) == 0 {
			return searchErrorMsg(fmt.Errorf("未找到结果"))
		}
		return searchResultMsg(allSongs)
	}
}

func fetchPlaylistSongsCmd(id, source string) tea.Cmd {
	return func() tea.Msg {
		fn := getPlaylistDetailFunc(source)
		if fn == nil {
			return searchErrorMsg(fmt.Errorf("Go source %s not support playlist detail", source))
		}
		songs, err := fn(id)
		if err != nil {
			return searchErrorMsg(err)
		}
		if len(songs) == 0 {
			return searchErrorMsg(fmt.Errorf("歌单为空"))
		}

		// 批量探测详情
		probeSongsBatch(songs)

		return searchResultMsg(songs)
	}
}

// 单曲下载命令
func downloadNextCmd(queue []model.Song, outDir string, withCover bool, withLyrics bool) tea.Cmd {
	return func() tea.Msg {
		if len(queue) == 0 {
			return nil
		}
		target := queue[0]
		err := downloadSongWithCookie(&target, outDir, withCover, withLyrics)
		return downloadOneFinishedMsg{err: err, song: target}
	}
}

// 内部下载实现
func downloadSongWithCookie(song *model.Song, outDir string, withCover bool, withLyrics bool) error {
	// 1. 准备目录
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	fileName := fmt.Sprintf("%s - %s", utils.SanitizeFilename(song.Artist), utils.SanitizeFilename(song.Name))
	filePath := filepath.Join(outDir, fileName+".mp3")

	// 2. 获取下载数据
	var finalData []byte

	// Soda 特殊处理 (加密)
	if song.Source == "soda" {
		cookie := cm.Get("soda")
		sodaInst := soda.New(cookie)
		info, err := sodaInst.GetDownloadInfo(song)
		if err != nil {
			return err
		}

		req, _ := http.NewRequest("GET", info.URL, nil)
		req.Header.Set("User-Agent", UA_Common)
		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		encryptedData, _ := io.ReadAll(resp.Body)
		finalData, err = soda.DecryptAudio(encryptedData, info.PlayAuth)
		if err != nil {
			return err
		}
	} else {
		// 常规源处理
		dlFunc := getDownloadFunc(song.Source)
		if dlFunc == nil {
			return fmt.Errorf("不支持的源: %s", song.Source)
		}

		urlStr, err := dlFunc(song)
		if err != nil {
			return err
		}
		if urlStr == "" {
			return fmt.Errorf("下载链接为空")
		}

		req, _ := http.NewRequest("GET", urlStr, nil)
		req.Header.Set("User-Agent", UA_Common)
		if song.Source == "bilibili" {
			req.Header.Set("Referer", "https://www.bilibili.com/")
		}
		if song.Source == "qq" {
			req.Header.Set("Referer", "http://y.qq.com")
		}
		if song.Source == "migu" {
			req.Header.Set("Referer", "http://music.migu.cn/")
		}

		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		finalData, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
	}

	// 3. 写入文件
	if err := os.WriteFile(filePath, finalData, 0644); err != nil {
		return err
	}

	// 4. 下载封面 (可选)
	if withCover && song.Cover != "" {
		go func() {
			coverPath := filepath.Join(outDir, fileName+".jpg")
			if data, err := utils.Get(song.Cover); err == nil {
				_ = os.WriteFile(coverPath, data, 0644)
			}
		}()
	}

	// 5. 下载歌词 (可选)
	if withLyrics {
		go func() {
			if lrcFunc := getLyricFunc(song.Source); lrcFunc != nil {
				if lrc, err := lrcFunc(song); err == nil && lrc != "" {
					lrcPath := filepath.Join(outDir, fileName+".lrc")
					_ = os.WriteFile(lrcPath, []byte(lrc), 0644)
				}
			}
		}()
	}

	return nil
}

// ... truncate, getSourceDisplay, View, renderTable 保持不变 ...
func truncate(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-1]) + "…"
	}
	return s
}

func getSourceDisplay(s []string) string {
	if len(s) == 0 {
		return "默认源"
	}
	return strings.Join(s, ", ")
}

func (m modelState) View() string {
	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("\n🎵 Go Music DL TUI") + "\n\n")

	switch m.state {
	case stateInput:
		s.WriteString("请输入搜索关键字:\n")
		s.WriteString(m.textInput.View())
		s.WriteString(fmt.Sprintf("\n\n(当前源: %v)", getSourceDisplay(m.sources)))
		s.WriteString("\n(按 Enter 搜索/解析, Ctrl+C 退出)")
		cm.mu.RLock()
		if len(cm.cookies) > 0 {
			var loadedSources []string
			for k := range cm.cookies {
				loadedSources = append(loadedSources, k)
			}
			cookieHint := fmt.Sprintf("\n(已加载 Cookie: %s)", strings.Join(loadedSources, ", "))
			s.WriteString(lipgloss.NewStyle().Foreground(greenColor).Render(cookieHint))
		}
		cm.mu.RUnlock()

		if m.err != nil {
			s.WriteString(lipgloss.NewStyle().Foreground(redColor).Render(fmt.Sprintf("\n\n❌ %v", m.err)))
		}
	case stateLoading:
		s.WriteString(fmt.Sprintf("\n %s 正在处理 '%s' ...\n", m.spinner.View(), m.textInput.Value()))
	case stateList:
		s.WriteString(m.renderTable())
		s.WriteString("\n")
		statusStyle := lipgloss.NewStyle().Foreground(subtleColor)
		s.WriteString(statusStyle.Render(m.statusMsg))
		s.WriteString("\n\n")
		s.WriteString(statusStyle.Render("↑/↓: 移动 • 空格: 选择 • a: 全选/清空 • Enter: 下载 • b: 返回 • q: 退出"))
	case statePlaylistResult: // 新增
		s.WriteString(m.renderPlaylistTable())
		s.WriteString("\n")
		statusStyle := lipgloss.NewStyle().Foreground(subtleColor)
		s.WriteString(statusStyle.Render(m.statusMsg))
		s.WriteString("\n\n")
		s.WriteString(statusStyle.Render("↑/↓: 移动 • Enter: 查看详情 • b: 返回 • q: 退出"))
	case stateDownloading:
		s.WriteString("\n")
		s.WriteString(m.progress.View() + "\n\n")
		s.WriteString(fmt.Sprintf("%s 正在处理: %d/%d\n", m.spinner.View(), m.downloaded, m.totalToDl))
		if len(m.downloadQueue) > 0 {
			current := m.downloadQueue[0]
			s.WriteString(lipgloss.NewStyle().Foreground(yellowColor).Render(fmt.Sprintf("-> %s - %s", current.Artist, current.Name)))
		}
		s.WriteString("\n\n" + lipgloss.NewStyle().Foreground(subtleColor).Render(m.statusMsg))
	}
	return s.String()
}

func (m modelState) renderTable() string {
	const (
		colCheck  = 6
		colIdx    = 4
		colTitle  = 25
		colArtist = 15
		colAlbum  = 15
		colDur    = 8
		colSize   = 10
		colBit    = 10
		colSrc    = 10
	)
	var b strings.Builder
	header := lipgloss.JoinHorizontal(lipgloss.Left,
		headerStyle.Width(colCheck).Render("[选]"),
		headerStyle.Width(colIdx).Render("ID"),
		headerStyle.Width(colTitle).Render("歌名"),
		headerStyle.Width(colArtist).Render("歌手"),
		headerStyle.Width(colAlbum).Render("专辑"),
		headerStyle.Width(colDur).Render("时长"),
		headerStyle.Width(colSize).Render("大小"),
		headerStyle.Width(colBit).Render("码率"),
		headerStyle.Width(colSrc).Render("来源"),
	)
	b.WriteString(header + "\n")
	start, end := m.calculatePagination()
	for i := start; i < end; i++ {
		song := m.songs[i]
		isCursor := (m.cursor == i)
		_, isSelected := m.selected[i]
		checkStr := "[ ]"
		if isSelected {
			checkStr = checkedStyle.Render("[✓]")
		}
		idxStr := fmt.Sprintf("%d", i+1)
		title := truncate(song.Name, colTitle-4)
		artist := truncate(song.Artist, colArtist-2)
		album := truncate(song.Album, colAlbum-2)
		dur := song.FormatDuration()
		size := song.FormatSize()
		bitrate := "-"
		if song.Bitrate > 0 {
			bitrate = fmt.Sprintf("%d kbps", song.Bitrate)
		}
		src := song.Source
		style := rowStyle
		if isCursor {
			style = selectedRowStyle
		}
		renderCell := func(text string, width int, style lipgloss.Style) string {
			return style.Width(width).MaxHeight(1).Render(text)
		}
		row := lipgloss.JoinHorizontal(lipgloss.Left,
			renderCell(checkStr, colCheck, style),
			renderCell(idxStr, colIdx, style),
			renderCell(title, colTitle, style),
			renderCell(artist, colArtist, style),
			renderCell(album, colAlbum, style),
			renderCell(dur, colDur, style),
			renderCell(size, colSize, style),
			renderCell(bitrate, colBit, style),
			renderCell(src, colSrc, style),
		)
		b.WriteString(row + "\n")
	}
	return b.String()
}

func (m modelState) renderPlaylistTable() string {
	const (
		colIdx     = 4
		colTitle   = 40
		colCount   = 10
		colCreator = 20
		colSrc     = 10
	)
	var b strings.Builder
	header := lipgloss.JoinHorizontal(lipgloss.Left,
		headerStyle.Width(colIdx).Render("ID"),
		headerStyle.Width(colTitle).Render("歌单名称"),
		headerStyle.Width(colCount).Render("歌曲数"),
		headerStyle.Width(colCreator).Render("创建者"),
		headerStyle.Width(colSrc).Render("来源"),
	)
	b.WriteString(header + "\n")

	height := 15
	start := 0
	end := len(m.playlists)
	if len(m.playlists) > height {
		if m.cursor >= height {
			start = m.cursor - height + 1
		}
		end = start + height
		if end > len(m.playlists) {
			end = len(m.playlists)
		}
	}

	for i := start; i < end; i++ {
		pl := m.playlists[i]
		isCursor := (m.cursor == i)

		idxStr := fmt.Sprintf("%d", i+1)
		title := truncate(pl.Name, colTitle-2)
		count := fmt.Sprintf("%d", pl.TrackCount)
		creator := truncate(pl.Creator, colCreator-2)
		src := pl.Source

		style := rowStyle
		if isCursor {
			style = selectedRowStyle
		}
		renderCell := func(text string, width int, style lipgloss.Style) string {
			return style.Width(width).MaxHeight(1).Render(text)
		}
		row := lipgloss.JoinHorizontal(lipgloss.Left,
			renderCell(idxStr, colIdx, style),
			renderCell(title, colTitle, style),
			renderCell(count, colCount, style),
			renderCell(creator, colCreator, style),
			renderCell(src, colSrc, style),
		)
		b.WriteString(row + "\n")
	}
	return b.String()
}

func (m modelState) calculatePagination() (int, int) {
	height := 15
	start := 0
	end := len(m.songs)
	if len(m.songs) > height {
		if m.cursor >= height {
			start = m.cursor - height + 1
		}
		end = start + height
		if end > len(m.songs) {
			end = len(m.songs)
		}
	}
	return start, end
}
