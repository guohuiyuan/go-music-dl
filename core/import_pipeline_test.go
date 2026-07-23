package core

import (
	"fmt"
	"testing"
)

func TestFullImportPipeline(t *testing.T) {
	// 使用小型内联 fixture，不依赖外部文件
	data := `D:/music/artist1 - song1.mp3
D:/music/artist2 - song2.m4a
D:/music/artist3 - song3.flac
D:/music/176杨玉莹-摇太阳.mp3
D:/music/A-Lin-天若有情.m4a
D:/music/Beyond - 光辉岁月.mp3
D:/music/2Someone-Star Unkind(Lanfranchi & Farina Remix).m4a`

	// 第一次导入
	result1, err := ImportDirectoryListingFromContent(data)
	if err != nil {
		t.Fatalf("第一次导入失败: %v", err)
	}
	fmt.Printf("=== 第一次导入 ===\n")
	fmt.Printf("总行数: %d\n", result1.Total)
	fmt.Printf("已导入: %d\n", result1.Imported)
	fmt.Printf("已跳过: %d\n", result1.Skipped)
	if result1.Imported+result1.Skipped != result1.Total {
		t.Fatalf("合计 %d != 总行数 %d", result1.Imported+result1.Skipped, result1.Total)
	}
	if result1.Imported == 0 {
		t.Fatal("期望有成功解析的条目")
	}

	// 第二次导入（全部文件.txt 已有第一次导入的条目）
	result2, err := ImportDirectoryListingFromContent(data)
	if err != nil {
		t.Fatalf("第二次导入失败: %v", err)
	}
	fmt.Printf("\n=== 第二次导入（已有 %d 条）===\n", result1.Imported)
	fmt.Printf("总行数: %d\n", result2.Total)
	fmt.Printf("已导入: %d\n", result2.Imported)
	fmt.Printf("已跳过: %d\n", result2.Skipped)
	if result2.Imported+result2.Skipped != result2.Total {
		t.Fatalf("合计 %d != 总行数 %d", result2.Imported+result2.Skipped, result2.Total)
	}
}
