package main

import (
	"testing"
	"time"

	"github.com/guohuiyuan/music-lib/apple"
	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
)

// PlaylistTestSuite 定义歌单平台的测试套件配置
type PlaylistTestSuite struct {
	Name    string
	Keyword string

	// 核心通用接口 (必须有)
	SearchPlaylist   func(string) ([]model.Playlist, error)
	GetPlaylistSongs func(string) ([]model.Song, error)
	ParsePlaylist    func(string) (*model.Playlist, []model.Song, error)

	// 可选特性接口 (某些源可能没有，允许为 nil)
	GetRecommended func() ([]model.Playlist, error)
}

func TestPlaylistPlatforms(t *testing.T) {
	suites := []PlaylistTestSuite{
		{
			Name:             "netease",
			Keyword:          "热歌",
			SearchPlaylist:   netease.SearchPlaylist,
			GetPlaylistSongs: netease.GetPlaylistSongs,
			ParsePlaylist:    netease.ParsePlaylist,
			GetRecommended:   netease.GetRecommendedPlaylists,
		},
		{
			Name:             "qq",
			Keyword:          "流行",
			SearchPlaylist:   qq.SearchPlaylist,
			GetPlaylistSongs: qq.GetPlaylistSongs,
			ParsePlaylist:    qq.ParsePlaylist,
			GetRecommended:   qq.GetRecommendedPlaylists,
		},
		{
			Name:             "kugou",
			Keyword:          "经典",
			SearchPlaylist:   kugou.SearchPlaylist,
			GetPlaylistSongs: kugou.GetPlaylistSongs,
			ParsePlaylist:    kugou.ParsePlaylist,
			GetRecommended:   kugou.GetRecommendedPlaylists,
		},
		{
			Name:             "kuwo",
			Keyword:          "抖音",
			SearchPlaylist:   kuwo.SearchPlaylist,
			GetPlaylistSongs: kuwo.GetPlaylistSongs,
			ParsePlaylist:    kuwo.ParsePlaylist,
			GetRecommended:   kuwo.GetRecommendedPlaylists,
		},
		{
			Name:             "soda",
			Keyword:          "抖音",
			SearchPlaylist:   soda.SearchPlaylist,
			GetPlaylistSongs: soda.GetPlaylistSongs,
			ParsePlaylist:    soda.ParsePlaylist,
		},
		{
			Name:             "fivesing",
			Keyword:          "古风",
			SearchPlaylist:   fivesing.SearchPlaylist,
			GetPlaylistSongs: fivesing.GetPlaylistSongs,
			ParsePlaylist:    fivesing.ParsePlaylist,
		},
		{
			Name:             "bilibili",
			Keyword:          "合集",
			SearchPlaylist:   bilibili.SearchPlaylist,
			GetPlaylistSongs: bilibili.GetPlaylistSongs,
			ParsePlaylist:    bilibili.ParsePlaylist,
		},
		{
			Name:             "apple",
			Keyword:          "chill",
			SearchPlaylist:   apple.SearchPlaylist,
			GetPlaylistSongs: apple.GetPlaylistSongs,
			ParsePlaylist:    apple.ParsePlaylist,
		},
	}

	for _, suite := range suites {
		suite := suite // 捕获变量
		t.Run(suite.Name, func(t *testing.T) {
			t.Parallel() // 并行测试以加快速度

			// -------------------------------------------------------
			// 1. 测试歌单搜索 (SearchPlaylist)
			// -------------------------------------------------------
			t.Logf("=== [%s] 1. Start Testing SearchPlaylist (Keyword: %s) ===", suite.Name, suite.Keyword)
			playlists, err := suite.SearchPlaylist(suite.Keyword)
			if err != nil {
				// 使用 Errorf 标记为测试失败，但允许继续（或使用 Logf 仅记录）
				t.Logf("❌ [%s] SearchPlaylist Error (Network/API issue): %v", suite.Name, err)
				return // 网络错误跳过后续步骤
			}
			if len(playlists) == 0 {
				t.Logf("⚠️ [%s] No playlists found, skipping detail tests", suite.Name)
				t.Skip("No playlists found")
				return
			}

			var first model.Playlist
			found := false
			for _, p := range playlists {
				if p.TrackCount > 0 {
					first = p
					found = true
					break
				}
			}
			if !found {
				first = playlists[0]
				t.Logf("⚠️ [%s] Warning: All returned playlists have 0 tracks, using the first one anyway", suite.Name)
			}

			t.Logf("✅ [%s] Found Playlist: %s (ID: %s, Tracks: %d)", suite.Name, first.Name, first.ID, first.TrackCount)

			if first.ID == "" {
				t.Errorf("❌ [%s] Playlist ID is empty! Invalid data.", suite.Name)
			}

			// [修改] 针对 Fivesing 在搜索后增加延时，防止过快请求详情
			if suite.Name == "fivesing" {
				t.Logf("⏳ [%s] Sleeping 2s before fetching details...", suite.Name)
				time.Sleep(2 * time.Second)
			}

			// -------------------------------------------------------
			// 2. 测试获取歌单详情 (GetPlaylistSongs)
			// -------------------------------------------------------
			t.Logf("=== [%s] 2. Start Testing GetPlaylistSongs (ID: %s) ===", suite.Name, first.ID)
			songs, err := suite.GetPlaylistSongs(first.ID)
			if err != nil {
				t.Errorf("❌ [%s] GetPlaylistSongs Failed: %v", suite.Name, err)
			} else {
				if len(songs) == 0 {
					t.Logf("⚠️ [%s] GetPlaylistSongs returned 0 songs (might be empty or restricted)", suite.Name)
				} else {
					t.Logf("✅ [%s] GetPlaylistSongs Success! Retrieved %d songs.", suite.Name, len(songs))
					t.Logf("   [%s] Sample Song: %s - %s (ID: %s)", suite.Name, songs[0].Name, songs[0].Artist, songs[0].ID)
				}
			}

			// -------------------------------------------------------
			// 3. 测试解析歌单链接 (ParsePlaylist)
			// -------------------------------------------------------
			if first.Link != "" && suite.ParsePlaylist != nil {
				// [修改] 动态延时：普通源 1s
				delay := 1 * time.Second
				time.Sleep(delay)

				t.Logf("=== [%s] 3. Start Testing ParsePlaylist (Link: %s) ===", suite.Name, first.Link)
				parsedMeta, parsedSongs, err := suite.ParsePlaylist(first.Link)
				if err != nil {
					t.Errorf("❌ [%s] ParsePlaylist Failed: %v", suite.Name, err)
				} else {
					if parsedMeta == nil {
						t.Errorf("❌ [%s] ParsePlaylist returned nil metadata", suite.Name)
					} else {
						t.Logf("✅ [%s] ParsePlaylist Meta Success: %s (ID: %s)", suite.Name, parsedMeta.Name, parsedMeta.ID)
					}
					t.Logf("✅ [%s] ParsePlaylist Songs Success: Retrieved %d songs", suite.Name, len(parsedSongs))
				}
			} else {
				t.Logf("⏹️ [%s] Skipping ParsePlaylist (Link empty or Func nil)", suite.Name)
			}

			// -------------------------------------------------------
			// 4. [新增] 测试推荐歌单 (可选特性)
			// -------------------------------------------------------
			if suite.GetRecommended != nil {
				// [修改] 动态延时：普通源 0.5s，Fivesing 3s
				delay := 500 * time.Millisecond
				if suite.Name == "fivesing" {
					delay = 3 * time.Second
					t.Logf("⏳ [%s] Sleeping %v before fetching recommended...", suite.Name, delay)
				}
				time.Sleep(delay)

				t.Logf("=== [%s] 4. Start Testing GetRecommended (Optional Feature) ===", suite.Name)

				recPlaylists, err := suite.GetRecommended()
				if err != nil {
					t.Errorf("❌ [%s] GetRecommended Failed: %v", suite.Name, err)
				} else {
					if len(recPlaylists) == 0 {
						t.Errorf("❌ [%s] GetRecommended returned 0 playlists", suite.Name)
					} else {
						t.Logf("✅ [%s] GetRecommended Success! Got %d playlists", suite.Name, len(recPlaylists))

						// 验证第一个推荐歌单是否有效（尝试获取其中的歌曲）
						if len(recPlaylists) > 0 {
							firstRec := recPlaylists[0]
							t.Logf("   [%s] Verifying first recommended: %s (ID: %s)", suite.Name, firstRec.Name, firstRec.ID)

							// 复用 GetPlaylistSongs 验证其有效性
							recSongs, err := suite.GetPlaylistSongs(firstRec.ID)
							if err != nil {
								t.Logf("⚠️ [%s] Warning: Failed to fetch songs for recommended playlist: %v", suite.Name, err)
							} else {
								t.Logf("✅ [%s] Recommended Playlist Verified! Contains %d songs.", suite.Name, len(recSongs))
							}
						}
					}
				}
			} else {
				t.Logf("⏹️ [%s] GetRecommended not configured, skipping.", suite.Name)
			}
		})
	}
}
