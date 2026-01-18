package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/go-music-dl/pkg/models"
)

func RunInteractive() {
	reader := bufio.NewReader(os.Stdin)

	// 1. 获取所有源，并排除 bilibili
	allSources := core.GetAllSourceNames()
	var defaultSources []string
	for _, s := range allSources {
		if s != "bilibili" {
			defaultSources = append(defaultSources, s)
		}
	}
	fmt.Printf(">> 命令行模式已启动\n>> 当前启用源: %v (已排除 bilibili)\n", defaultSources)

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

		// 3. 打印表格
		fmt.Printf("\n找到 %d 条结果:\n", len(songs))
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		// 表头中文化
		fmt.Fprintln(w, "序号\t歌名\t歌手\t来源\t时长/大小")
		fmt.Fprintln(w, "----\t----\t----\t----\t---------")
		for i, s := range songs {
			info := models.FormatDurationSeconds(s.Duration)
			if s.Size > 0 {
				info += fmt.Sprintf(" (%.2fMB)", float64(s.Size)/1024/1024)
			}
			fmt.Fprintf(w, "[%d]\t%s\t%s\t%s\t%s\n", i+1, s.Name, s.Artist, s.Source, info)
		}
		w.Flush()

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
			// 获取歌曲对象
			targetSong := songs[index-1]
			
			fmt.Printf("\n--> 正在下载 [%d/%d]: %s - %s ...\n", i+1, len(selectedIndices), targetSong.Artist, targetSong.Name)
			
			// 调用 Core 下载
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

// 辅助函数保持不变
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
					if start > maxIndex { start = maxIndex }
					if end > maxIndex { end = maxIndex }
					if start > end { continue }
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