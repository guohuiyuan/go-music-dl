package core

import (
	"fmt"
	"os"
	"testing"
)

func TestFullImportPipeline(t *testing.T) {
	data, err := os.ReadFile("../test_import.txt")
	if err != nil {
		t.Fatal(err)
	}

	// 模拟第一次导入：全部文件.txt 不存在的情况
	result1, err := ImportDirectoryListingFromContent(string(data))
	if err != nil {
		t.Fatalf("第一次导入失败: %v", err)
	}
	fmt.Printf("=== 第一次导入（全部文件.txt 不存在）===\n")
	fmt.Printf("总行数: %d\n", result1.Total)
	fmt.Printf("已导入: %d\n", result1.Imported)
	fmt.Printf("已跳过: %d\n", result1.Skipped)
	if result1.Imported+result1.Skipped != result1.Total {
		fmt.Printf("⚠️ 合计 %d != 总行数 %d\n", result1.Imported+result1.Skipped, result1.Total)
	}

	// 模拟第二次导入：全部文件.txt 已有第一次导入的条目
	result2, err := ImportDirectoryListingFromContent(string(data))
	if err != nil {
		t.Fatalf("第二次导入失败: %v", err)
	}
	fmt.Printf("\n=== 第二次导入（全部文件.txt 已有 %d 条）===\n", result1.Imported)
	fmt.Printf("总行数: %d\n", result2.Total)
	fmt.Printf("已导入: %d\n", result2.Imported)
	fmt.Printf("已跳过: %d\n", result2.Skipped)
	if result2.Imported+result2.Skipped != result2.Total {
		fmt.Printf("⚠️ 合计 %d != 总行数 %d\n", result2.Imported+result2.Skipped, result2.Total)
	}
}
