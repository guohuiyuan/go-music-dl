package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/apple"
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
	UA_Common                = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
	searchTypeSong           = "song"
	searchTypePlaylist       = "playlist"
	searchTypeAlbum          = "album"
	legacyCLIDefaultPageSize = 50
	listViewReservedRows     = 10
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
}

var cm = &CookieManager{}

func (m *CookieManager) Load() {
	core.CM.Load()
}

func (m *CookieManager) Get(source string) string {
	return core.CM.Get(source)
}

func (m *CookieManager) GetAll() map[string]string {
	return core.CM.GetAll()
}

func nextSearchType(current string) string {
	switch current {
	case searchTypeSong:
		return searchTypePlaylist
	case searchTypePlaylist:
		return searchTypeAlbum
	default:
		return searchTypeSong
	}
}

func placeholderForSearchType(searchType string) string {
	switch searchType {
	case searchTypePlaylist:
		return "输入歌单关键词或粘贴歌单链接..."
	case searchTypeAlbum:
		return "输入专辑关键词或粘贴专辑链接..."
	default:
		return "输入歌名、歌手或粘贴分享链接 (Tab 切换)..."
	}
}

func searchTypeLabel(searchType string) string {
	switch searchType {
	case searchTypePlaylist:
		return "歌单"
	case searchTypeAlbum:
		return "专辑"
	default:
		return "单曲"
	}
}

func isCollectionSearchType(searchType string) bool {
	return searchType == searchTypePlaylist || searchType == searchTypeAlbum
}

func collectionLabel(searchType string) string {
	if searchType == searchTypeAlbum {
		return "专辑"
	}
	return "歌单"
}

func collectionCreatorLabel(searchType string) string {
	if searchType == searchTypeAlbum {
		return "歌手"
	}
	return "创建者"
}

func collectionCountLabel(searchType string) string {
	if searchType == searchTypeAlbum {
		return "曲目数"
	}
	return "歌曲数"
}

func defaultSourcesForSearchType(searchType string) []string {
	switch searchType {
	case searchTypePlaylist:
		return core.GetPlaylistSourceNames()
	case searchTypeAlbum:
		return core.GetAlbumSourceNames()
	default:
		return core.GetDefaultSourceNames()
	}
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
	case "apple":
		return apple.New(c).Search
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
	case "apple":
		return apple.New(c).GetDownloadURL
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
	case "apple":
		return apple.New(c).GetLyrics
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
	case "joox":
		return joox.New(c).Parse
	case "qianqian":
		return qianqian.New(c).Parse
	case "apple":
		return apple.New(c).Parse
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
	case "migu":
		return migu.New(c).SearchPlaylist
	case "jamendo":
		return jamendo.New(c).SearchPlaylist
	case "joox":
		return joox.New(c).SearchPlaylist
	case "qianqian":
		return qianqian.New(c).SearchPlaylist
	case "bilibili":
		return bilibili.New(c).SearchPlaylist
	case "soda":
		return soda.New(c).SearchPlaylist
	case "fivesing":
		return fivesing.New(c).SearchPlaylist
	case "apple":
		return apple.New(c).SearchPlaylist
	default:
		return nil
	}
}

func getAlbumSearchFunc(source string) func(string) ([]model.Playlist, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).SearchAlbum
	case "qq":
		return qq.New(c).SearchAlbum
	case "kugou":
		return kugou.New(c).SearchAlbum
	case "kuwo":
		return kuwo.New(c).SearchAlbum
	case "migu":
		return migu.New(c).SearchAlbum
	case "jamendo":
		return jamendo.New(c).SearchAlbum
	case "joox":
		return joox.New(c).SearchAlbum
	case "qianqian":
		return qianqian.New(c).SearchAlbum
	case "soda":
		return soda.New(c).SearchAlbum
	case "apple":
		return apple.New(c).SearchAlbum
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
	case "migu":
		return migu.New(c).GetPlaylistSongs
	case "jamendo":
		return jamendo.New(c).GetPlaylistSongs
	case "joox":
		return joox.New(c).GetPlaylistSongs
	case "qianqian":
		return qianqian.New(c).GetPlaylistSongs
	case "bilibili":
		return bilibili.New(c).GetPlaylistSongs
	case "soda":
		return soda.New(c).GetPlaylistSongs
	case "fivesing":
		return fivesing.New(c).GetPlaylistSongs
	case "apple":
		return apple.New(c).GetPlaylistSongs
	default:
		return nil
	}
}

func getAlbumDetailFunc(source string) func(string) ([]model.Song, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetAlbumSongs
	case "qq":
		return qq.New(c).GetAlbumSongs
	case "kugou":
		return kugou.New(c).GetAlbumSongs
	case "kuwo":
		return kuwo.New(c).GetAlbumSongs
	case "migu":
		return migu.New(c).GetAlbumSongs
	case "jamendo":
		return jamendo.New(c).GetAlbumSongs
	case "joox":
		return joox.New(c).GetAlbumSongs
	case "qianqian":
		return qianqian.New(c).GetAlbumSongs
	case "soda":
		return soda.New(c).GetAlbumSongs
	case "apple":
		return apple.New(c).GetAlbumSongs
	default:
		return nil
	}
}

// 新增：每日推荐歌单工厂 (仅支持 qq, netease, kuwo, kugou)
func getRecommendFunc(source string) func() ([]model.Playlist, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).GetRecommendedPlaylists
	case "qq":
		return qq.New(c).GetRecommendedPlaylists
	case "kugou":
		return kugou.New(c).GetRecommendedPlaylists
	case "kuwo":
		return kuwo.New(c).GetRecommendedPlaylists
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
	case "migu":
		return migu.New(c).ParsePlaylist
	case "jamendo":
		return jamendo.New(c).ParsePlaylist
	case "joox":
		return joox.New(c).ParsePlaylist
	case "qianqian":
		return qianqian.New(c).ParsePlaylist
	case "bilibili":
		return bilibili.New(c).ParsePlaylist
	case "soda":
		return soda.New(c).ParsePlaylist
	case "fivesing":
		return fivesing.New(c).ParsePlaylist
	case "apple":
		return apple.New(c).ParsePlaylist
	default:
		return nil
	}
}

func getParseAlbumFunc(source string) func(string) (*model.Playlist, []model.Song, error) {
	c := cm.Get(source)
	switch source {
	case "netease":
		return netease.New(c).ParseAlbum
	case "qq":
		return qq.New(c).ParseAlbum
	case "kugou":
		return kugou.New(c).ParseAlbum
	case "kuwo":
		return kuwo.New(c).ParseAlbum
	case "migu":
		return migu.New(c).ParseAlbum
	case "jamendo":
		return jamendo.New(c).ParseAlbum
	case "joox":
		return joox.New(c).ParseAlbum
	case "qianqian":
		return qianqian.New(c).ParseAlbum
	case "soda":
		return soda.New(c).ParseAlbum
	case "apple":
		return apple.New(c).ParseAlbum
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
	if strings.Contains(link, "joox.com") {
		return "joox"
	}
	if strings.Contains(link, "bilibili.com") || strings.Contains(link, "b23.tv") {
		return "bilibili"
	}
	if strings.Contains(link, "douyin.com") || strings.Contains(link, "qishui") {
		return "soda"
	}
	if strings.Contains(link, "91q.com") {
		return "qianqian"
	}
	if strings.Contains(link, "5sing") {
		return "fivesing"
	}
	if strings.Contains(link, "jamendo.com") {
		return "jamendo"
	}
	if strings.Contains(link, "music.apple.com") || strings.Contains(link, "itunes.apple.com") {
		return "apple"
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
	stateSwitching                          // 换源中
)

// --- 主模型 ---
type modelState struct {
	state     sessionState
	textInput textinput.Model // 搜索输入框
	spinner   spinner.Model   // 加载动画
	progress  progress.Model  // 进度条组件

	searchType string           // "song", "playlist" or "album"
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

	// 换源队列管理
	switchQueue []int
	switchTotal int
	switched    int

	err       error
	statusMsg string // 底部状态栏消息

	windowWidth  int
	windowHeight int
	pageSize     int
}

// 启动 UI 的入口
func StartUI(initialKeyword string, sources []string, outDir string, withCover bool, withLyrics bool) {
	// 1. 加载 Cookies
	cm.Load()

	ti := textinput.New()
	ti.Placeholder = "输入歌名、歌手或粘贴分享链接 (Tab 切换搜歌单)..."
	ti.Placeholder = placeholderForSearchType(searchTypeSong)
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(primaryColor)

	prog := progress.New(progress.WithDefaultGradient())

	settings := core.GetWebSettings()
	pageSize := settings.CliPageSize
	if pageSize <= 0 {
		pageSize = core.DefaultCLIPageSize
	}

	initialState := stateInput
	if initialKeyword != "" {
		ti.SetValue(initialKeyword)
		initialState = stateLoading
	}

	m := modelState{
		state:      initialState,
		searchType: searchTypeSong,
		textInput:  ti,
		spinner:    sp,
		progress:   prog,
		selected:   make(map[int]struct{}),
		sources:    sources,
		outDir:     outDir,
		withCover:  withCover,
		withLyrics: withLyrics,
		pageSize:   pageSize,
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
		m.windowHeight = msg.Height
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
	case stateSwitching:
		return m.updateSwitching(msg)
	}

	return m, nil
}

// --- 1. 输入状态逻辑 ---
func (m modelState) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyTab {
			m.searchType = nextSearchType(m.searchType)
			m.textInput.Placeholder = placeholderForSearchType(m.searchType)
			return m, nil
		}
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
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "w":
			m.state = stateLoading
			m.searchType = searchTypePlaylist
			m.textInput.Placeholder = placeholderForSearchType(m.searchType)
			m.songs = nil
			m.playlists = nil
			m.statusMsg = "正在获取每日推荐歌单..."
			// 重新加载 Cookie 以防外部文件变动
			cm.Load()
			return m, tea.Batch(m.spinner.Tick, recommendPlaylistsCmd(m.sources))
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
		m.playlists = nil
		m.state = stateList
		m.cursor = 0
		m.selected = make(map[int]struct{})

		// 如果是单曲解析（通常通过 URL），自动选中
		if len(m.songs) == 1 && strings.HasPrefix(m.textInput.Value(), "http") {
			m.selected[0] = struct{}{}
			m.statusMsg = fmt.Sprintf("解析成功: %s。按回车下载。", m.songs[0].Name)
		} else {
			if m.searchType == "playlist" { // 从歌单进入
				m.statusMsg = fmt.Sprintf("歌单解析完成，包含 %d 首歌曲（每页 %d）。空格选择，回车下载。", len(m.songs), m.currentPageSize())
			} else {
				m.statusMsg = fmt.Sprintf("找到 %d 首歌曲（每页 %d）。空格选择，回车下载。", len(m.songs), m.currentPageSize())
			}
		}
		if isCollectionSearchType(m.searchType) && !(len(m.songs) == 1 && strings.HasPrefix(m.textInput.Value(), "http")) {
			m.statusMsg = fmt.Sprintf("%s解析完成，包含 %d 首歌曲（每页 %d）。空格选择，回车下载。", collectionLabel(m.searchType), len(m.songs), m.currentPageSize())
		}
		return m, nil
	case playlistResultMsg:
		m.playlists = msg
		m.songs = nil
		m.state = statePlaylistResult
		m.cursor = 0
		m.statusMsg = fmt.Sprintf("找到 %d 个歌单（每页 %d）。回车查看详情。", len(m.playlists), m.currentPageSize())
		m.statusMsg = fmt.Sprintf("找到 %d 个%s（每页 %d）。回车查看详情。", len(m.playlists), collectionLabel(m.searchType), m.currentPageSize())
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
		case "pgup":
			m.cursor = m.moveCursorByPage(m.cursor, -1, len(m.playlists))
		case "pgdown":
			m.cursor = m.moveCursorByPage(m.cursor, 1, len(m.playlists))
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
				if m.searchType == searchTypePlaylist || m.searchType == searchTypeAlbum {
					m.state = stateLoading
					m.statusMsg = fmt.Sprintf("正在获取%s [%s] 详情...", collectionLabel(m.searchType), target.Name)
					return m, tea.Batch(
						m.spinner.Tick,
						fetchCollectionSongsCmd(target.ID, target.Source, m.searchType),
					)
				}
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
		case "pgup":
			m.cursor = m.moveCursorByPage(m.cursor, -1, len(m.songs))
		case "pgdown":
			m.cursor = m.moveCursorByPage(m.cursor, 1, len(m.songs))
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
		case "r":
			if len(m.songs) == 0 || m.cursor < 0 || m.cursor >= len(m.songs) {
				return m, nil
			}

			if len(m.selected) == 0 {
				m.selected[m.cursor] = struct{}{}
			}

			m.switchQueue = m.switchQueue[:0]
			for idx := range m.selected {
				if idx >= 0 && idx < len(m.songs) {
					m.switchQueue = append(m.switchQueue, idx)
				}
			}
			if len(m.switchQueue) == 0 {
				return m, nil
			}

			m.switchTotal = len(m.switchQueue)
			m.switched = 0
			m.state = stateSwitching
			m.statusMsg = fmt.Sprintf("正在换源... 0/%d", m.switchTotal)

			firstIdx := m.switchQueue[0]
			return m, tea.Batch(
				m.spinner.Tick,
				m.progress.SetPercent(0),
				switchSourceCmd(firstIdx, m.songs[firstIdx]),
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

type switchSourceResultMsg struct {
	index int
	song  model.Song
	err   error
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

		resultStr := fmt.Sprintf("已完成: %s - %s", msg.song.Name, msg.song.Artist)
		if msg.err != nil {
			resultStr = fmt.Sprintf("❌ 失败: %s - %s (%v)", msg.song.Name, msg.song.Artist, msg.err)
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

// --- 4.5 换源状态逻辑 ---
func (m modelState) updateSwitching(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	case switchSourceResultMsg:
		m.switched++
		if msg.err == nil && msg.index >= 0 && msg.index < len(m.songs) {
			m.songs[msg.index] = msg.song
		}

		pct := float64(m.switched) / float64(m.switchTotal)
		if m.switched >= m.switchTotal {
			m.state = stateList
			m.statusMsg = fmt.Sprintf("换源完成: %d/%d", m.switched, m.switchTotal)
			m.selected = make(map[int]struct{})
			m.switchQueue = nil
			return m, m.progress.SetPercent(1)
		}

		m.statusMsg = fmt.Sprintf("正在换源... %d/%d", m.switched, m.switchTotal)
		if len(m.switchQueue) > 0 {
			m.switchQueue = m.switchQueue[1:]
		}
		if len(m.switchQueue) == 0 {
			m.state = stateList
			m.statusMsg = fmt.Sprintf("换源完成: %d/%d", m.switched, m.switchTotal)
			m.selected = make(map[int]struct{})
			return m, m.progress.SetPercent(1)
		}

		nextIdx := m.switchQueue[0]
		return m, tea.Batch(
			m.progress.SetPercent(pct),
			switchSourceCmd(nextIdx, m.songs[nextIdx]),
		)
	}
	return m, nil
}

// --- 辅助命令 ---

// 核心改进：探测歌曲详情（填充大小和码率）
func probeSongDetails(song *model.Song) {
	dlFunc := getDownloadFunc(song.Source)
	if dlFunc == nil {
		song.IsInvalid = true
		return
	}

	urlStr, err := dlFunc(song)
	if err != nil || urlStr == "" {
		song.IsInvalid = true
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
		song.IsInvalid = true
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		song.IsInvalid = true
		return
	}

	var size int64
	// 优先从 Content-Range 获取总大小
	cr := resp.Header.Get("Content-Range")
	if parts := strings.Split(cr, "/"); len(parts) == 2 {
		fmt.Sscanf(parts[1], "%d", &size)
	}
	// 降级使用 Content-Length
	if size == 0 {
		size = resp.ContentLength
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

			parseAlbumFn := getParseAlbumFunc(src)
			if parseAlbumFn != nil {
				if _, songs, err := parseAlbumFn(keyword); err == nil && len(songs) > 0 {
					probeSongsBatch(songs)
					return searchResultMsg(songs)
				}
			}

			return searchErrorMsg(fmt.Errorf("解析失败: 暂不支持 %s 平台的此链接类型或解析出错", src))
		}

		// 2. 关键词搜索模式
		targetSources := sources
		if len(targetSources) == 0 {
			targetSources = defaultSourcesForSearchType(searchType)
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
		if searchType == searchTypeAlbum {
			var allAlbums []model.Playlist
			for _, src := range targetSources {
				fn := getAlbumSearchFunc(src)
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
						allAlbums = append(allAlbums, res...)
						mu.Unlock()
					}
				}(src, fn)
			}
			wg.Wait()
			if len(allAlbums) == 0 {
				return searchErrorMsg(fmt.Errorf("未找到专辑"))
			}
			return playlistResultMsg(allAlbums)
		}

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

func recommendPlaylistsCmd(sources []string) tea.Cmd {
	return func() tea.Msg {
		targetSources := sources
		if len(targetSources) == 0 {
			targetSources = []string{"netease", "qq", "kugou", "kuwo"}
		}

		var wg sync.WaitGroup
		var mu sync.Mutex
		var allPlaylists []model.Playlist

		for _, src := range targetSources {
			fn := getRecommendFunc(src)
			if fn == nil {
				continue
			}
			wg.Add(1)
			go func(s string) {
				defer wg.Done()
				if res, err := fn(); err == nil && len(res) > 0 {
					for i := range res {
						res[i].Source = s
					}
					mu.Lock()
					allPlaylists = append(allPlaylists, res...)
					mu.Unlock()
				}
			}(src)
		}
		wg.Wait()

		if len(allPlaylists) == 0 {
			return searchErrorMsg(fmt.Errorf("未找到推荐歌单"))
		}
		return playlistResultMsg(allPlaylists)
	}
}

func fetchCollectionSongsCmd(id, source, searchType string) tea.Cmd {
	return func() tea.Msg {
		var fn func(string) ([]model.Song, error)
		switch searchType {
		case searchTypeAlbum:
			fn = getAlbumDetailFunc(source)
		default:
			fn = getPlaylistDetailFunc(source)
		}
		if fn == nil {
			return searchErrorMsg(fmt.Errorf("%s 源暂不支持%s详情", source, collectionLabel(searchType)))
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
func fetchPlaylistSongsCmd(id, source string) tea.Cmd {
	return fetchCollectionSongsCmd(id, source, searchTypePlaylist)
}

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

// 换源命令
func switchSourceCmd(index int, song model.Song) tea.Cmd {
	return func() tea.Msg {
		newSong, err := findBestSwitchSong(song)
		return switchSourceResultMsg{index: index, song: newSong, err: err}
	}
}

// 内部下载实现（支持 ID3 元数据内嵌）
func downloadSongWithCookie(song *model.Song, outDir string, withCover bool, withLyrics bool) error {
	result, err := core.SaveSongToFile(song, outDir, withCover, withLyrics)
	if err != nil {
		return err
	}
	if result.Warning != "" {
		fmt.Printf("Warning: %s\n", result.Warning)
	}
	return nil

	// 1. 准备目录
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	fileName := fmt.Sprintf("%s - %s", utils.SanitizeFilename(song.Name), utils.SanitizeFilename(song.Artist))

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

	// 3. 获取歌词并内嵌到 ID3（如启用）
	var lyricStr string
	if withLyrics {
		if lrcFunc := getLyricFunc(song.Source); lrcFunc != nil {
			if lrc, err := lrcFunc(song); err == nil && lrc != "" {
				lyricStr = lrc
			}
		}
	}

	// 4. 获取封面并内嵌到 ID3（如启用）
	var coverData []byte
	var coverMime string
	if withCover && song.Cover != "" {
		if data, err := utils.Get(song.Cover); err == nil && len(data) > 0 {
			coverData = data
			coverMime = http.DetectContentType(data)
			if idx := strings.Index(coverMime, ";"); idx >= 0 {
				coverMime = strings.TrimSpace(coverMime[:idx])
			}
		}
	}

	// 5. 内嵌元数据到 ID3（如有数据）
	ext := core.DetectAudioExt(finalData)

	if (ext == "mp3" || ext == "flac" || ext == "m4a" || ext == "wma") && (lyricStr != "" || len(coverData) > 0) {
		if embeddedData, err := core.EmbedSongMetadata(finalData, song, lyricStr, coverData, coverMime); err == nil {
			finalData = embeddedData
		} else if errors.Is(err, core.ErrFFmpegNotFound) {
			fmt.Printf("⚠ 未检测到 ffmpeg，已跳过歌词/封面嵌入，仍会正常下载音频\n")
		} else {
			fmt.Printf("⚠ 音频元数据嵌入失败，已使用原始音频继续保存: %v\n", err)
		}
	}

	// 6. 写入文件
	filePath := filepath.Join(outDir, fileName+"."+ext)
	if err := os.WriteFile(filePath, finalData, 0644); err != nil {
		return err
	}

	return nil
}

// --- 换源逻辑（与 Web 相同约束） ---
type switchCandidate struct {
	song    model.Song
	score   float64
	durDiff int
}

func findBestSwitchSong(current model.Song) (model.Song, error) {
	if current.Name == "" {
		return model.Song{}, fmt.Errorf("缺少歌名")
	}
	if current.Source == "" {
		return model.Song{}, fmt.Errorf("缺少来源")
	}

	keyword := current.Name
	if current.Artist != "" {
		keyword = current.Name + " " + current.Artist
	}

	sources := core.GetAllSourceNames()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var candidates []switchCandidate

	for _, src := range sources {
		if src == "" || src == current.Source {
			continue
		}
		if src == "soda" || src == "fivesing" {
			continue
		}
		fn := getSearchFunc(src)
		if fn == nil {
			continue
		}

		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			res, err := fn(keyword)
			if (err != nil || len(res) == 0) && current.Artist != "" {
				res, _ = fn(current.Name)
			}
			if len(res) == 0 {
				return
			}
			limit := len(res)
			if limit > 8 {
				limit = 8
			}

			for i := 0; i < limit; i++ {
				cand := res[i]
				cand.Source = s
				score := calcSongSimilarity(current.Name, current.Artist, cand.Name, cand.Artist)
				if score <= 0 {
					continue
				}

				durDiff := 0
				if current.Duration > 0 && cand.Duration > 0 {
					durDiff = intAbs(current.Duration - cand.Duration)
					if !isDurationClose(current.Duration, cand.Duration) {
						continue
					}
				}

				mu.Lock()
				candidates = append(candidates, switchCandidate{song: cand, score: score, durDiff: durDiff})
				mu.Unlock()
			}
		}(src)
	}

	wg.Wait()
	if len(candidates) == 0 {
		return model.Song{}, fmt.Errorf("未找到可换源结果")
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].durDiff < candidates[j].durDiff
		}
		return candidates[i].score > candidates[j].score
	})

	for _, cand := range candidates {
		if validatePlayable(&cand.song) {
			return cand.song, nil
		}
	}

	return model.Song{}, fmt.Errorf("无可播放的换源结果")
}

func validatePlayable(song *model.Song) bool {
	if song == nil || song.ID == "" || song.Source == "" {
		return false
	}
	if song.Source == "soda" || song.Source == "fivesing" {
		return false
	}

	fn := getDownloadFunc(song.Source)
	if fn == nil {
		return false
	}
	urlStr, err := fn(song)
	if err != nil || urlStr == "" {
		return false
	}

	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("Range", "bytes=0-1")
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
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200 || resp.StatusCode == 206
}

func calcSongSimilarity(name, artist, candName, candArtist string) float64 {
	nameA := normalizeText(name)
	nameB := normalizeText(candName)
	if nameA == "" || nameB == "" {
		return 0
	}
	nameSim := similarityScore(nameA, nameB)

	artistA := normalizeText(artist)
	artistB := normalizeText(candArtist)
	if artistA == "" || artistB == "" {
		return nameSim
	}

	artistSim := similarityScore(artistA, artistB)
	return nameSim*0.7 + artistSim*0.3
}

func normalizeText(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.In(r, unicode.Han) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func similarityScore(a, b string) float64 {
	if a == b {
		return 1
	}
	if a == "" || b == "" {
		return 0
	}
	la := len([]rune(a))
	lb := len([]rune(b))
	maxLen := la
	if lb > maxLen {
		maxLen = lb
	}
	if maxLen == 0 {
		return 0
	}
	dist := levenshteinDistance(a, b)
	if dist >= maxLen {
		return 0
	}
	return 1 - float64(dist)/float64(maxLen)
}

func levenshteinDistance(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	la := len(ra)
	lb := len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 0
			if ra[i-1] != rb[j-1] {
				cost = 1
			}
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			cur[j] = del
			if ins < cur[j] {
				cur[j] = ins
			}
			if sub < cur[j] {
				cur[j] = sub
			}
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

func isDurationClose(a, b int) bool {
	if a <= 0 || b <= 0 {
		return true
	}
	diff := intAbs(a - b)
	if diff <= 10 {
		return true
	}
	maxAllowed := int(float64(a) * 0.15)
	if maxAllowed < 10 {
		maxAllowed = 10
	}
	return diff <= maxAllowed
}

func intAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
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
	if m.state == stateInput {
		s.WriteString(m.renderInputView())
		return s.String()
	}
	s.WriteString(lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("\n🎵 Go Music DL TUI") + "\n\n")

	switch m.state {
	case stateInput:
		s.WriteString("请输入搜索关键字:\n")
		s.WriteString(m.textInput.View())
		modeLabel := "单曲"
		if m.searchType == "playlist" {
			modeLabel = "歌单"
		}
		s.WriteString(fmt.Sprintf("\n\n(当前源: %v)", getSourceDisplay(m.sources)))
		s.WriteString(fmt.Sprintf("\n(当前模式: %s搜索)", modeLabel))
		s.WriteString("\n(按 Enter 搜索/解析, Tab 切换搜歌/歌单, w 每日推荐, Ctrl+C 退出)")
		cookies := cm.GetAll()
		if len(cookies) > 0 {
			var loadedSources []string
			for k := range cookies {
				loadedSources = append(loadedSources, k)
			}
			cookieHint := fmt.Sprintf("\n(已加载 Cookie: %s)", strings.Join(loadedSources, ", "))
			s.WriteString(lipgloss.NewStyle().Foreground(greenColor).Render(cookieHint))
		}
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
		s.WriteString(statusStyle.Render("↑/↓: 移动 • PgUp/PgDn: 翻页 • 空格: 选择 • a: 全选/清空 • r: 换源 • Enter: 下载 • b: 返回 • q: 退出"))
	case statePlaylistResult: // 新增
		s.WriteString(m.renderCollectionTable())
		s.WriteString("\n")
		statusStyle := lipgloss.NewStyle().Foreground(subtleColor)
		s.WriteString(statusStyle.Render(m.statusMsg))
		s.WriteString("\n\n")
		s.WriteString(statusStyle.Render("↑/↓: 移动 • PgUp/PgDn: 翻页 • Enter: 查看详情 • b: 返回 • q: 退出"))
	case stateDownloading:
		s.WriteString("\n")
		s.WriteString(m.progress.View() + "\n\n")
		s.WriteString(fmt.Sprintf("%s 正在处理: %d/%d\n", m.spinner.View(), m.downloaded, m.totalToDl))
		if len(m.downloadQueue) > 0 {
			current := m.downloadQueue[0]
			s.WriteString(lipgloss.NewStyle().Foreground(yellowColor).Render(fmt.Sprintf("-> %s - %s", current.Name, current.Artist)))
		}
		s.WriteString("\n\n" + lipgloss.NewStyle().Foreground(subtleColor).Render(m.statusMsg))
	case stateSwitching:
		s.WriteString("\n")
		s.WriteString(m.progress.View() + "\n\n")
		s.WriteString(fmt.Sprintf("%s %s\n", m.spinner.View(), m.statusMsg))
	}
	return s.String()
}

func (m modelState) renderInputView() string {
	var s strings.Builder
	s.WriteString("请输入搜索关键字:\n")
	s.WriteString(m.textInput.View())
	s.WriteString(fmt.Sprintf("\n\n(当前源: %v)", getSourceDisplay(m.sources)))
	s.WriteString(fmt.Sprintf("\n(当前模式: %s搜索)", searchTypeLabel(m.searchType)))
	s.WriteString("\n(按 Enter 搜索/解析, Tab 切换单曲/歌单/专辑, w 每日推荐, Ctrl+C 退出)")

	cookies := cm.GetAll()
	if len(cookies) > 0 {
		var loadedSources []string
		for k := range cookies {
			loadedSources = append(loadedSources, k)
		}
		cookieHint := fmt.Sprintf("\n(已加载 Cookie: %s)", strings.Join(loadedSources, ", "))
		s.WriteString(lipgloss.NewStyle().Foreground(greenColor).Render(cookieHint))
	}
	if m.err != nil {
		s.WriteString(lipgloss.NewStyle().Foreground(redColor).Render(fmt.Sprintf("\n\n错误: %v", m.err)))
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
		colBit    = 11
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
	currentPage, totalPages := m.currentPageInfo(len(m.songs))
	b.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render(fmt.Sprintf("第 %d/%d 页，每页 %d 条", currentPage, totalPages, m.currentPageSize())) + "\n")
	start, end := m.calculatePagination()
	for i := start; i < end; i++ {
		song := m.songs[i]
		isCursor := (m.cursor == i)
		_, isSelected := m.selected[i]

		checkStr := "[ ]"
		if isSelected {
			checkStr = checkedStyle.Render("[✓]")
		}

		var sizeStr string
		if song.IsInvalid {
			// 仅标记文件详情为红色提示，但不影响勾选状态显示
			sizeStr = lipgloss.NewStyle().Foreground(redColor).Render("!无效")
		} else {
			sizeStr = song.FormatSize()
		}

		idxStr := fmt.Sprintf("%d", i+1)
		title := truncate(song.Name, colTitle-4)
		artist := truncate(song.Artist, colArtist-2)
		album := truncate(song.Album, colAlbum-2)
		dur := song.FormatDuration()
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
			renderCell(sizeStr, colSize, style),
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

	currentPage, totalPages := m.currentPageInfo(len(m.playlists))
	b.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render(fmt.Sprintf("第 %d/%d 页，每页 %d 条", currentPage, totalPages, m.currentPageSize())) + "\n")

	start, end := m.calculatePlaylistPagination()

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

func (m modelState) renderCollectionTable() string {
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
		headerStyle.Width(colTitle).Render(collectionLabel(m.searchType)+"名称"),
		headerStyle.Width(colCount).Render(collectionCountLabel(m.searchType)),
		headerStyle.Width(colCreator).Render(collectionCreatorLabel(m.searchType)),
		headerStyle.Width(colSrc).Render("来源"),
	)
	b.WriteString(header + "\n")

	currentPage, totalPages := m.currentPageInfo(len(m.playlists))
	b.WriteString(lipgloss.NewStyle().Foreground(subtleColor).Render(fmt.Sprintf("第 %d/%d 页，每页 %d 条", currentPage, totalPages, m.currentPageSize())) + "\n")

	start, end := m.calculatePlaylistPagination()
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
	return m.pageRangeForCursor(len(m.songs))
}

func (m modelState) calculatePlaylistPagination() (int, int) {
	return m.pageRangeForCursor(len(m.playlists))
}

func (m modelState) currentPageSize() int {
	if m.pageSize <= 0 {
		return core.DefaultCLIPageSize
	}
	pageSize := m.pageSize
	if pageSize == legacyCLIDefaultPageSize {
		if maxRows := m.maxRowsForListView(); maxRows > 0 && maxRows < pageSize {
			pageSize = maxRows
		}
	}
	return pageSize
}

func (m modelState) maxRowsForListView() int {
	if m.windowHeight <= 0 {
		return 0
	}
	available := m.windowHeight - listViewReservedRows
	if available < 1 {
		return 1
	}
	return available
}

func (m modelState) pageRangeForCursor(total int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	pageSize := m.currentPageSize()
	start := (m.cursor / pageSize) * pageSize
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return start, end
}

func (m modelState) currentPageInfo(total int) (int, int) {
	if total <= 0 {
		return 1, 1
	}
	pageSize := m.currentPageSize()
	totalPages := (total + pageSize - 1) / pageSize
	page := (m.cursor / pageSize) + 1
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	return page, totalPages
}

func (m modelState) moveCursorByPage(cursor int, delta int, total int) int {
	if total <= 0 {
		return 0
	}
	next := cursor + delta*m.currentPageSize()
	if next < 0 {
		next = 0
	}
	if next >= total {
		next = total - 1
	}
	return next
}
