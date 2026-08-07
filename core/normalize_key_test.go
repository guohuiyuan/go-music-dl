package core

import (
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func init() {
	SetSkipRules(DefaultSkipRules)
}

func TestNormalizeKeyDedup(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool // true = 应视为重复（key 相等）
	}{
		// 分隔符不同
		{"顿号 vs &", "卢巧音、王力宏 - 好心分手", "卢巧音&王力宏 - 好心分手", true},
		{"& vs 顺序不同", "王力宏&卢巧音 - 好心分手", "卢巧音&王力宏 - 好心分手", true},
		{"逗号 vs &", "蔡依林, 周杰伦 - 骑士精神", "蔡依林&周杰伦 - 骑士精神", true},
		{"顿号 vs 逗号", "孙楠、韩红 - 美丽的神话", "孙楠,韩红 - 美丽的神话", true},
		{"斜杠 vs 下划线", "海鸣威 / 吴琼 - 老人与海", "海鸣威 _ 吴琼 - 老人与海", true},
		{"多空格 vs 斜杠", "海鸣威  吴琼 - 老人与海", "海鸣威 / 吴琼 - 老人与海", true},
		// 大小写
		{"大小写", "Beyond - 喜欢你", "BEYOND - 喜欢你", true},
		{"英文歌名大小写", "Monty Python - Always Look On The Bright Side Of Life", "monty python - always look on the bright side of life", true},
		// 繁简
		{"繁简歌手", "叶倩文 - 伤逝", "叶蒨文 - 伤逝", true},
		{"繁简歌名", "黎明 - 今夜你会不会來", "黎明 - 今夜你会不会来", true},
		// 歌名标点
		{"歌名点号", "彭家丽 - 何故.何苦.何必", "彭家丽 - 何故 何苦 何必", true},
		{"歌名波浪号", "张宇 - 用心良苦～", "张宇 - 用心良苦", true},
		// 全半角
		{"全角英数", "Beyond - 喜欢你", "ＢＥＹＯＮＤ - 喜欢你", true},
		{"全角空格", "Beyond - 喜欢你", "Beyond　-　喜欢你", true},
		// 平凡相同
		{"完全相同", "王心凌 - 当你", "王心凌 - 当你", true},
		// 同音字无法自动处理（预期不同）
		{"同音字", "许哲佩 - 气球", "许哲珮 - 气球", false},
	}
	for _, tc := range cases {
		got := NormalizeStoredKey(tc.a) == NormalizeStoredKey(tc.b)
		if got != tc.want {
			t.Errorf("%s: NormalizeStoredKey(%q)=%q vs NormalizeStoredKey(%q)=%q, equal=%v want %v",
				tc.name, tc.a, NormalizeStoredKey(tc.a), tc.b, NormalizeStoredKey(tc.b), got, tc.want)
		}
	}
}

func TestNormalizeArtistKeepsEnglishNames(t *testing.T) {
	// 英文歌手名含空格不应被拆分
	got := NormalizeStoredKey("Michael Jackson - Beat It")
	if got != "michael jackson - beat it" {
		t.Errorf("英文歌手名被误拆分: %q", got)
	}
	// 下划线在名字中不被替换（无空格包围）
	got = NormalizeStoredKey("相依为命_ - _R")
	if got != "相依为命_ - _r" {
		t.Errorf("下划线被误处理: %q", got)
	}
}

func TestNormalizeRulesToggle(t *testing.T) {
	// 关闭小写折叠后，大小写不同的 key 不再匹配
	SetSkipRules("ws,sep,cjk,sort,tc,punct,nfkc") // 去掉 lower
	if NormalizeStoredKey("Beyond - 喜欢你") == NormalizeStoredKey("BEYOND - 喜欢你") {
		t.Errorf("关闭 lower 后仍匹配大小写")
	}
	// 恢复默认
	SetSkipRules(DefaultSkipRules)
	if NormalizeStoredKey("Beyond - 喜欢你") != NormalizeStoredKey("BEYOND - 喜欢你") {
		t.Errorf("恢复默认后大小写不匹配")
	}
}

func TestIsSongDownloadedContainRule(t *testing.T) {
	// 精确命中（含 & 排序归一）
	exact := NormalizeStoredKey("卢巧音&王力宏 - 好心分手")
	set := map[string]struct{}{exact: {}}
	song := &model.Song{Artist: "卢巧音、王力宏", Name: "好心分手"}
	if !IsSongDownloaded(song, set) {
		t.Errorf("精确归一化匹配应命中")
	}

	// 歌手包含匹配：下载 Alicia Keys&Usher，曲库 Alicia Keys
	set = map[string]struct{}{NormalizeStoredKey("Alicia Keys - If I Ain't Got You"): {}}
	song = &model.Song{Artist: "Alicia Keys&Usher", Name: "If I Ain't Got You"}
	if !IsSongDownloaded(song, set) {
		t.Errorf("歌手包含匹配应命中: 下载方含曲库方歌手")
	}

	// 反向包含：下载 Alicia Keys，曲库 Alicia Keys&Usher
	song = &model.Song{Artist: "Alicia Keys", Name: "If I Ain't Got You"}
	if !IsSongDownloaded(song, set) {
		t.Errorf("反向包含匹配应命中")
	}

	// 歌手完全不重叠（同名歌不同歌手）不命中
	set = map[string]struct{}{NormalizeStoredKey("BEYOND - 喜欢你"): {}}
	song = &model.Song{Artist: "G.E.M. 邓紫棋", Name: "喜欢你"}
	if IsSongDownloaded(song, set) {
		t.Errorf("不同歌手同名歌不应命中")
	}

	// 歌名不同不命中
	set = map[string]struct{}{NormalizeStoredKey("Alicia Keys - If I Ain't Got You"): {}}
	song = &model.Song{Artist: "Alicia Keys&Usher", Name: "Fallin'"}
	if IsSongDownloaded(song, set) {
		t.Errorf("歌名不同不应命中")
	}

	// 关闭 contain 规则后不命中
	SetSkipRules("ws,sep,cjk,sort,lower,tc,punct,nfkc") // 去掉 contain
	set = map[string]struct{}{NormalizeStoredKey("Alicia Keys - If I Ain't Got You"): {}}
	song = &model.Song{Artist: "Alicia Keys&Usher", Name: "If I Ain't Got You"}
	if IsSongDownloaded(song, set) {
		t.Errorf("关闭 contain 后不应命中")
	}
	SetSkipRules(DefaultSkipRules)
}

func TestIsSongDownloadedHomophoneRule(t *testing.T) {	SetSkipRules(DefaultSkipRules + ",homophone")

	// 场景1：黄大伟 vs 黄大炜
	set := map[string]struct{}{
		NormalizeStoredKey("黄大炜 - 你把我灌醉"): {},
	}
	buildPinyinSet(set)
	song := &model.Song{Artist: "黄大伟", Name: "你把我灌醉"}
	if !IsSongDownloaded(song, set) {
		t.Errorf("同音字匹配应命中: 黄大伟 vs 黄大炜")
	}

	// 场景2：许哲佩 vs 许哲珮
	set = map[string]struct{}{
		NormalizeStoredKey("许哲珮 - 气球"): {},
	}
	buildPinyinSet(set)
	song = &model.Song{Artist: "许哲佩", Name: "气球"}
	if !IsSongDownloaded(song, set) {
		t.Errorf("同音字匹配应命中: 许哲佩 vs 许哲珮")
	}

	// 歌名不同不应命中
	set = map[string]struct{}{
		NormalizeStoredKey("黄大炜 - 你把我灌醉"): {},
	}
	buildPinyinSet(set)
	song = &model.Song{Artist: "黄大伟", Name: "冷漠花心"}
	if IsSongDownloaded(song, set) {
		t.Errorf("歌名不同不应命中")
	}

	// 默认规则（无 homophone）不应命中
	SetSkipRules(DefaultSkipRules) // 不含 homophone
	clearPinyinSet()
	song = &model.Song{Artist: "黄大伟", Name: "你把我灌醉"}
	if IsSongDownloaded(song, set) {
		t.Errorf("默认规则下同音字不应命中")
	}
}

func TestNormalizeNewRules(t *testing.T) {
	// 全部开启（含 paren/heyu）验证各规则效果
	SetSkipRules(DefaultSkipRules + ",paren,heyu")
	defer SetSkipRules(DefaultSkipRules)
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		// semi：分号分隔符
		{"分号", "钟镇涛、彭健新 - 一段情", "钟镇涛;彭健新 - 一段情", true},
		{"全角分号", "钟镇涛；彭健新 - 一段情", "钟镇涛、彭健新 - 一段情", true},
		// bang：感叹问号
		{"感叹问号", "谭咏麟 - 再见亦是泪!", "谭咏麟 - 再见亦是泪!?", true},
		{"全角问号", "谭咏麟 - 再见亦是泪！？", "谭咏麟 - 再见亦是泪!", true},
		// trailunder：末尾下划线
		{"末尾下划线", "谭咏麟 - 再见亦是泪!_", "谭咏麟 - 再见亦是泪!", true},
		// cjkspace：全汉字歌名删空格
		{"全汉字歌名空格", "谭咏麟 - 爱多一次 痛多一次", "谭咏麟 - 爱多一次痛多一次", true},
		// 英文歌名空格不受影响
		{"英文歌名保留空格", "Michael Jackson - Beat It", "Michael Jackson - Beat It", true},
		// ndash：长横线
		{"长横线", "陈奕迅 – 十年", "陈奕迅 - 十年", true},
		// feat
		{"feat", "Alicia Keys feat. Usher - If I Ain't Got You", "Alicia Keys&Usher - If I Ain't Got You", true},
		{"ft", "A ft. B - Song", "A&B - Song", true},
		// seq：序号剥离
		{"序号", "陈奕迅 - 01 - 十年", "陈奕迅 - 十年", true},
		{"序号点", "陈奕迅 - 01.十年", "陈奕迅 - 十年", true},
		// heyu：和/与（默认关，需显式开启）
		{"和", "孙楠和韩红 - 美丽的神话", "孙楠&韩红 - 美丽的神话", true},
		// paren：括号剥离（现在默认开）
		{"括号", "阿YueYue - 不负人间 (伴奏)", "阿YueYue - 不负人间", true},
		{"全角括号", "就是南方凯 - 离别开出花（DJ版）", "就是南方凯 - 离别开出花", true},
		{"方括号", "蓝又时 - 孤单心事【男版】", "蓝又时 - 孤单心事", true},
		{"国语版标记", "彭羚 - 囚鸟(国)", "彭羚 - 囚鸟", true},
		// underscore：汉字_汉字下划线（默认开）
		{"下划线分隔", "张智霖_许秋怡 - 现代爱情故事", "张智霖&许秋怡 - 现代爱情故事", true},
		// 末尾下划线不受影响（艺术家场景）
		{"艺术家末尾下划线不受影响", "相依为命_ - _R", "相依为命 - R", false},
		// boundary：字母-汉字边界（默认开，配合 contain 生效）
		{"字母汉字边界", "G.E.M.邓紫棋 - 喜欢你", "G.E.M. 邓紫棋 - 喜欢你", true},
		{"中英拼接歌手", "阿YueYue - 不负人间", "阿 YueYue - 不负人间", true},
	}
	for _, tc := range cases {
		got := NormalizeStoredKey(tc.a) == NormalizeStoredKey(tc.b)
		if got != tc.want {
			t.Errorf("%s: %q vs %q -> %q vs %q, equal=%v want %v",
				tc.name, tc.a, tc.b, NormalizeStoredKey(tc.a), NormalizeStoredKey(tc.b), got, tc.want)
		}
	}
}

func TestNormalizeDefaultClosedRules(t *testing.T) {
	// paren 和 heyu 默认关闭：不开启时应不匹配
	SetSkipRules("ws,sep,cjk,sort,lower,tc,punct,nfkc,contain,semi,bang,cjkspace,trailunder,ndash,feat,seq")
	if NormalizeStoredKey("阿YueYue - 不负人间 (伴奏)") == NormalizeStoredKey("阿YueYue - 不负人间") {
		t.Errorf("paren 未开启时不应匹配括号差异")
	}
	if NormalizeStoredKey("孙楠和韩红 - 美丽的神话") == NormalizeStoredKey("孙楠&韩红 - 美丽的神话") {
		t.Errorf("heyu 未开启时不应匹配")
	}
	// 开启后应匹配
	SetSkipRules(DefaultSkipRules + ",paren,heyu")
	if NormalizeStoredKey("阿YueYue - 不负人间 (伴奏)") != NormalizeStoredKey("阿YueYue - 不负人间") {
		t.Errorf("paren 开启后应匹配")
	}
	if NormalizeStoredKey("孙楠和韩红 - 美丽的神话") != NormalizeStoredKey("孙楠&韩红 - 美丽的神话") {
		t.Errorf("heyu 开启后应匹配")
	}
	SetSkipRules(DefaultSkipRules)
}

func TestBoundaryBandRules(t *testing.T) {
	// boundary 依赖 contain 规则匹配（歌手包含）
	SetSkipRules(DefaultSkipRules)
	set := map[string]struct{}{NormalizeStoredKey("F.I.R. - 我们的爱"): {}}
	song := &model.Song{Artist: "F.I.R.飞儿乐团", Name: "我们的爱"}
	if !IsSongDownloaded(song, set) {
		t.Errorf("boundary+contain 应命中: F.I.R.飞儿乐团 vs F.I.R.")
	}

	// band 默认不勾：不命中
	SetSkipRules(strings.ReplaceAll(DefaultSkipRules, "boundary,", "") + ",boundary") // 保持默认
	set = map[string]struct{}{NormalizeStoredKey("八三夭 - 想见你想见你想见你"): {}}
	song = &model.Song{Artist: "八三夭乐团", Name: "想见你想见你想见你"}
	if IsSongDownloaded(song, set) {
		t.Errorf("band 未开启时不应命中")
	}
	// band 开启后命中
	SetSkipRules(DefaultSkipRules + ",band")
	if !IsSongDownloaded(song, set) {
		t.Errorf("band 开启后应命中: 八三夭乐团 vs 八三夭")
	}
	SetSkipRules(DefaultSkipRules)
}

func TestNoLiveRule(t *testing.T) {	SetSkipRules(DefaultSkipRules) // 含 nolive
	// 原版已有：live 版应跳过
	set := map[string]struct{}{NormalizeStoredKey("张学友 - 她来听我的演唱会"): {}}
	song := &model.Song{Artist: "张学友", Name: "她来听我的演唱会 live"}
	if !IsSongDownloaded(song, set) {
		t.Errorf("nolive 应命中: 原版已有时 live 版跳过")
	}
	// 括号形式 live
	song = &model.Song{Artist: "张学友", Name: "她来听我的演唱会 (live)"}
	if !IsSongDownloaded(song, set) {
		t.Errorf("nolive 应命中: 括号 live 形式")
	}
	// 原版没有：live 版应下载
	set = map[string]struct{}{NormalizeStoredKey("王菲 - 红豆"): {}}
	song = &model.Song{Artist: "张学友", Name: "她来听我的演唱会 live"}
	if IsSongDownloaded(song, set) {
		t.Errorf("原版没有时 live 版不应跳过")
	}
	// 关闭 nolive 后不跳过
	SetSkipRules(strings.ReplaceAll(DefaultSkipRules, ",nolive", ""))
	set = map[string]struct{}{NormalizeStoredKey("张学友 - 她来听我的演唱会"): {}}
	song = &model.Song{Artist: "张学友", Name: "她来听我的演唱会 live"}
	if IsSongDownloaded(song, set) {
		t.Errorf("关闭 nolive 后不应跳过")
	}
	SetSkipRules(DefaultSkipRules)
}

func TestLongFilenameSkip(t *testing.T) {
	SetSkipRules(DefaultSkipRules) // 含 longname
	// 保存设置：阈值 90（默认）
	ws := GetWebSettings()
	ws.MaxFilenameLen = 90
	if err := SaveWebSettings(ws); err != nil {
		t.Fatalf("SaveWebSettings: %v", err)
	}

	// 超长文件名（>90 字节）：跳过
	longName := "一首非常非常非常非常非常非常非常非常非常非常非常非常非常非常长的歌曲名字测试超长跳过规则"
	song := &model.Song{Artist: "测试歌手", Name: longName}
	result, err := DownloadWithDedupCheckWithTemplate(song, t.TempDir(), false, false, DefaultDownloadFilenameTemplate, map[string]struct{}{})
	if err != nil {
		t.Fatalf("超长应跳过而非报错: %v", err)
	}
	if result == nil || !result.Skipped {
		t.Errorf("超长文件名应跳过")
	}

	// 短文件名：正常下载（会尝试网络请求 → 这里应返回失败但非 skipped）
	short := &model.Song{Artist: "测试歌手", Name: "短歌"}
	result2, _ := DownloadWithDedupCheckWithTemplate(short, t.TempDir(), false, false, DefaultDownloadFilenameTemplate, map[string]struct{}{})
	if result2 != nil && result2.Skipped {
		t.Errorf("短文件名不应因超长跳过")
	}

	// 关闭 longname 规则：超长不跳过（走下载流程）
	SetSkipRules(strings.ReplaceAll(DefaultSkipRules, ",longname", ""))
	result3, _ := DownloadWithDedupCheckWithTemplate(song, t.TempDir(), false, false, DefaultDownloadFilenameTemplate, map[string]struct{}{})
	if result3 != nil && result3.Skipped {
		t.Errorf("关闭 longname 后不应因超长跳过")
	}
	SetSkipRules(DefaultSkipRules)
}
