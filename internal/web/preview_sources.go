package web

import (
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

// previewEnricher 为某个音源补充“是否只能试听”的元数据。
//
// 通用过滤本身不依赖音源：filterSongsForDisplay 只读取
// preview_only / only_vip_playable 和时长。Spotify、汽水等音源已经由
// music-lib 在歌曲上写好这些标记，无需适配；某个音源如果只返回完整时长、
// 不返回播放权限，就注册一个 enricher 在这里补数据。
type previewEnricher func([]model.Song) []model.Song

var previewEnrichers = map[string]previewEnricher{
	"netease": markNeteasePreviewSongs,
}

func markPreviewSongs(songs []model.Song) []model.Song {
	if len(songs) == 0 {
		return songs
	}

	indexesBySource := make(map[string][]int, len(songs))
	for i := range songs {
		source := strings.ToLower(strings.TrimSpace(songs[i].Source))
		if source == "" {
			continue
		}
		indexesBySource[source] = append(indexesBySource[source], i)
	}

	for source, indexes := range indexesBySource {
		enrich := previewEnrichers[source]
		if enrich == nil {
			continue
		}
		subset := make([]model.Song, 0, len(indexes))
		for _, index := range indexes {
			subset = append(subset, songs[index])
		}
		enriched := enrich(subset)
		if len(enriched) != len(indexes) {
			continue
		}
		for offset, index := range indexes {
			songs[index] = enriched[offset]
		}
	}
	return songs
}
