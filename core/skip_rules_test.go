package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkipRulesPersistRoundTrip(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "data", "settings.db"))
	t.Setenv("MUSIC_DL_COOKIE_FILE", filepath.Join(baseDir, "data", "cookies.json"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)
	if err := os.MkdirAll(filepath.Join(baseDir, "data"), 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	// 默认值应为全选
	got := GetWebSettings()
	if got.SkipRules != DefaultSkipRules {
		t.Fatalf("默认 SkipRules = %q, want %q", got.SkipRules, DefaultSkipRules)
	}

	// 保存部分规则
	got.SkipRules = "ws,sep,cjk"
	if err := SaveWebSettings(got); err != nil {
		t.Fatalf("SaveWebSettings: %v", err)
	}

	// 读取应返回保存值
	back := GetWebSettings()
	if back.SkipRules != "ws,sep,cjk" {
		t.Fatalf("回读 SkipRules = %q, want %q", back.SkipRules, "ws,sep,cjk")
	}

	// SetSkipRules 缓存应已同步（SaveWebSettings 内调用）
	if !ruleOn("ws") || !ruleOn("sep") || !ruleOn("cjk") {
		t.Fatalf("保存后规则缓存未同步")
	}
	if ruleOn("lower") || ruleOn("tc") || ruleOn("punct") || ruleOn("nfkc") || ruleOn("sort") {
		t.Fatalf("未勾选规则不应启用")
	}

	// 保存空规则（用户全不选）时 normalize 应回退默认
	got.SkipRules = ""
	if err := SaveWebSettings(got); err != nil {
		t.Fatalf("SaveWebSettings(空): %v", err)
	}
	back = GetWebSettings()
	if back.SkipRules != DefaultSkipRules {
		t.Fatalf("空规则回退后 = %q, want %q", back.SkipRules, DefaultSkipRules)
	}
}
