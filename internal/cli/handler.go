package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter" // 引入第三方库
	"github.com/olekukonko/tablewriter/tw"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/go-music-dl/pkg/models"
)

func RunInteractive() {
	reader := bufio.NewReader(os.Stdin)

	// 1. 获取默认源
	defaultSources := core.GetDefaultSourceNames()
	fmt.Printf(">> 命令行模式已启动\n>> 当前启用源: %v (已排除 bilibili, joox, jamendo, fivesing)\n", defaultSources)

	// 显示所有支持的源及其描述
	fmt.Println("\n支持的音乐源简介:")
	allSources := core.GetAllSourceNames()
	for _, source := range allSources {
		desc := core.GetSourceDescription(source)
		fmt.Printf("  • %s: %s\n", source, desc)
	}
	fmt.Println()

	for {
		fmt.Print("\n[搜索] 请输入歌名或歌手 (输入 q 退出): ")
		input, _ := reader.ReadString('\n')
		keyword := strings.TrimSpace(input)

		if keyword == "q" {
			fmt.Println("再见!")
			break
		}
		if keyword == "" {
			continue
		}

		// 2. 搜索
		fmt.Println("正在搜索...")
		songs, err := core.SearchAndFilter(keyword, defaultSources)
		if err != nil {
			fmt.Println("搜索出错:", err)
			continue
		}

		if len(songs) == 0 {
			fmt.Println("未找到结果。")
			continue
		}

		// 3. 打印表格 (适配 tablewriter v1.1.3)
		fmt.Printf("\n找到 %d 条结果:\n", len(songs))

		// 使用 NewTable 并传入配置选项
		rendition := tw.Rendition{
			Borders: tw.BorderNone,
			Symbols: tw.NewSymbols(tw.StyleNone),
			Settings: tw.Settings{
				Separators: tw.SeparatorsNone,
				Lines:      tw.LinesNone,
			},
		}
		table := tablewriter.NewTable(os.Stdout,
			tablewriter.WithHeader([]string{"序号", "歌名", "歌手", "来源", "时长/大小"}),
			tablewriter.WithHeaderAlignment(tw.AlignLeft),
			tablewriter.WithRowAlignment(tw.AlignLeft),
			tablewriter.WithPadding(tw.PaddingNone),
			tablewriter.WithHeaderAutoFormat(tw.Off),
			tablewriter.WithRowAutoFormat(tw.Off),
			tablewriter.WithHeaderAutoWrap(tw.WrapTruncate),
			tablewriter.WithRowAutoWrap(tw.WrapTruncate),
			tablewriter.WithTrimSpace(tw.On),
			tablewriter.WithRendition(rendition),
		)

		for i, s := range songs {
			info := models.FormatDurationSeconds(s.Duration)
			if s.Size > 0 {
				info += fmt.Sprintf(" (%.2fMB)", float64(s.Size)/1024/1024)
			}

			row := []string{
				fmt.Sprintf("[%d]", i+1),
				s.Name,
				s.Artist,
				s.Source,
				info,
			}
			table.Append(row)
		}

		table.Render()

		// ... (省略后面的下载代码，保持不变) ...
		// 4. 选择下载
		fmt.Print("\n[下载] 请输入序号 (例如: 1-3 5 7): ")
		selectionInput, _ := reader.ReadString('\n')
		selectionInput = strings.TrimSpace(selectionInput)

		if selectionInput == "" {
			continue
		}

		selectedIndices := parseSelection(selectionInput, len(songs))
		if len(selectedIndices) == 0 {
			fmt.Println("无效的选择。")
			continue
		}

		for i, index := range selectedIndices {
			targetSong := songs[index-1]
			fmt.Printf("\n--> 正在下载 [%d/%d]: %s - %s ...\n", i+1, len(selectedIndices), targetSong.Artist, targetSong.Name)

			err := core.DownloadSong(&targetSong)

			if err != nil {
				fmt.Printf("    ❌ 失败: %v\n", err)
			} else {
				fmt.Printf("    ✅ 成功\n")
			}
		}
		fmt.Println("\n本轮任务结束。")
	}
}

// ... 辅助函数 parseSelection 等保持不变 ...
func parseSelection(input string, maxIndex int) []int {
	var result []int
	input = strings.ReplaceAll(input, ",", " ")
	input = strings.ReplaceAll(input, "，", " ")
	parts := strings.Fields(input)

	for _, part := range parts {
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
				end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
				if err1 == nil && err2 == nil && start > 0 && end > 0 && start <= end {
					if start > maxIndex {
						start = maxIndex
					}
					if end > maxIndex {
						end = maxIndex
					}
					if start > end {
						continue
					}
					for i := start; i <= end; i++ {
						result = append(result, i)
					}
					continue
				}
			}
		}
		index, err := strconv.Atoi(part)
		if err == nil && index > 0 && index <= maxIndex {
			result = append(result, index)
		}
	}
	return removeDuplicates(result)
}

func removeDuplicates(nums []int) []int {
	seen := make(map[int]bool)
	var result []int
	for _, num := range nums {
		if !seen[num] {
			seen[num] = true
			result = append(result, num)
		}
	}
	return result
}
