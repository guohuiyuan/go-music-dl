package cli

import (
	"fmt"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	// 引入核心包 (用于下载)
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/go-music-dl/pkg/models"

	// 引入数据模型
	"github.com/guohuiyuan/music-lib/model"

	// 引入所有支持的音乐源
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
)

var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}

	listHeader = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).MarginBottom(1)
	itemStyle  = lipgloss.NewStyle().PaddingLeft(1)
	selected   = lipgloss.NewStyle().PaddingLeft(1).Foreground(highlight).Bold(true)
)

func Run(keyword string, sources []string, outDir string, number int) {
	fmt.Printf("🔍 正在搜索: %s ...\n", keyword)

	// --- 1. 默认源设置逻辑 ---
	// 如果用户没有指定源，默认使用所有支持的音乐源 (显式排除 bilibili)
	if len(sources) == 0 {
		sources = []string{
			"netease",  // 网易云
			"qq",       // QQ音乐
			"kugou",    // 酷狗
			"kuwo",     // 酷我
			"migu",     // 咪咕
			"fivesing", // 5sing
			"jamendo",  // Jamendo
			"joox",     // Joox
			"qianqian", // 千千音乐
			"soda",     // Soda
		}
	}

	var wg sync.WaitGroup
	var allSongs []model.Song
	var mu sync.Mutex

	for _, src := range sources {
		// 双重保险：在循环中再次强制排除 bilibili
		if src == "bilibili" {
			continue
		}

		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			var res []model.Song
			var err error

			// 调用各个源的 Search 方法
			// 注意：确保 music-lib 的各包中 Search 函数签名一致
			switch s {
			case "kugou":
				res, err = kugou.Search(keyword)
			case "netease":
				res, err = netease.Search(keyword)
			case "qq":
				res, err = qq.Search(keyword)
			case "kuwo":
				res, err = kuwo.Search(keyword)
			case "migu":
				res, err = migu.Search(keyword)
			case "fivesing":
				res, err = fivesing.Search(keyword)
			case "jamendo":
				res, err = jamendo.Search(keyword)
			case "joox":
				res, err = joox.Search(keyword)
			case "qianqian":
				res, err = qianqian.Search(keyword)
			case "soda":
				res, err = soda.Search(keyword)
			}

			if err != nil {
				// 某个源搜索失败不影响整体，直接返回
				return
			}

			// 截断结果，避免单个源返回过多数据
			if len(res) > number {
				res = res[:number]
			}

			mu.Lock()
			allSongs = append(allSongs, res...)
			mu.Unlock()
		}(src)
	}
	wg.Wait()

	if len(allSongs) == 0 {
		fmt.Println("❌ 未找到相关结果。")
		return
	}

	// 启动 TUI 界面
	p := tea.NewProgram(modelState{songs: allSongs, outDir: outDir})
	if _, err := p.Run(); err != nil {
		fmt.Println("运行错误:", err)
	}
}

type modelState struct {
	songs    []model.Song
	cursor   int
	outDir   string
	quitting bool
}

func (m modelState) Init() tea.Cmd { return nil }

func (m modelState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.songs)-1 {
				m.cursor++
			}
		case "enter":
			selectedSong := m.songs[m.cursor]
			// 选中歌曲后调用下载函数
			return m, func() tea.Msg {
				downloadCLI(&selectedSong, m.outDir)
				return tea.Quit()
			}
		}
	}
	return m, nil
}

func (m modelState) View() string {
	if m.quitting {
		return "再见!\n"
	}

	// 表头中文化及对齐
	s := listHeader.Render(fmt.Sprintf("%-4s %-20s %-15s %-20s %-8s %-8s %-8s", "序号", "歌名", "歌手", "专辑", "时长", "大小", "来源")) + "\n"

	start := 0
	end := len(m.songs)
	// 简单的分页逻辑
	if m.cursor > 10 {
		start = m.cursor - 10
	}
	if end > start+20 {
		end = start + 20
	}

	for i := start; i < end; i++ {
		song := m.songs[i]
		idx := fmt.Sprintf("%d", i+1)

		album := song.Album
		// 使用 pkg/models 中的辅助函数格式化时长
		dur := models.FormatDurationSeconds(song.Duration)
		size := formatSize(song.Size)

		// 简单的字符串截断，防止界面错位
		songName := song.Name
		songArtist := song.Artist
		if len(songName) > 20 {
			songName = songName[:17] + "..."
		}
		if len(songArtist) > 15 {
			songArtist = songArtist[:12] + "..."
		}
		if len(album) > 20 {
			album = album[:17] + "..."
		}

		line := fmt.Sprintf("%-4s %-20s %-15s %-20s %-8s %-8s %-8s", idx, songName, songArtist, album, dur, size, song.Source)

		if m.cursor == i {
			s += selected.Render(">" + line) + "\n"
		} else {
			s += itemStyle.Render(" " + line) + "\n"
		}
	}
	s += "\n" + lipgloss.NewStyle().Foreground(subtle).Render("j/k: 上下选择 • enter: 下载 • q: 退出")
	return s
}

// downloadCLI 使用 Core 包进行下载
// 这样可以确保复用 Headers 伪装、防盗链处理等逻辑，避免“假下载”
func downloadCLI(s *model.Song, dir string) {
	fmt.Printf("\n🚀 正在通过核心下载器下载: %s - %s ...\n", s.Artist, s.Name)

	// 调用 Core 包的 DownloadSong 方法
	// 注意：Core 包内部应处理文件保存路径，或者你可以修改 Core 接受 outputDir 参数
	// 这里假设 Core 默认下载到当前目录的 downloads 文件夹，或者你可以在 Core 中完善路径逻辑
	err := core.DownloadSong(s)

	if err != nil {
		fmt.Printf("❌ 下载失败: %v\n", err)
	} else {
		fmt.Println("✅ 下载成功!")
	}
}

func formatSize(size int64) string {
	if size == 0 {
		return "-"
	}
	mb := float64(size) / 1024 / 1024
	return fmt.Sprintf("%.2f MB", mb)
}