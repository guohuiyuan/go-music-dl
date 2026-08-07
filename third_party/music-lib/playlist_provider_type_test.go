package main

import (
	"testing"

	"github.com/guohuiyuan/music-lib/apple"
	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/provider"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
)

var _ provider.PlaylistProvider = (*netease.Netease)(nil)
var _ provider.PlaylistProvider = (*qq.QQ)(nil)
var _ provider.PlaylistProvider = (*kugou.Kugou)(nil)
var _ provider.PlaylistProvider = (*kuwo.Kuwo)(nil)
var _ provider.PlaylistProvider = (*migu.Migu)(nil)
var _ provider.PlaylistProvider = (*qianqian.Qianqian)(nil)
var _ provider.PlaylistProvider = (*soda.Soda)(nil)
var _ provider.PlaylistProvider = (*fivesing.Fivesing)(nil)
var _ provider.PlaylistProvider = (*jamendo.Jamendo)(nil)
var _ provider.PlaylistProvider = (*joox.Joox)(nil)
var _ provider.PlaylistProvider = (*bilibili.Bilibili)(nil)
var _ provider.PlaylistProvider = (*apple.Apple)(nil)

var _ provider.PlaylistCategoryProvider = (*netease.Netease)(nil)
var _ provider.PlaylistCategoryProvider = (*qq.QQ)(nil)
var _ provider.PlaylistCategoryProvider = (*kugou.Kugou)(nil)
var _ provider.PlaylistCategoryProvider = (*kuwo.Kuwo)(nil)
var _ provider.PlaylistCategoryProvider = (*migu.Migu)(nil)
var _ provider.PlaylistCategoryProvider = (*qianqian.Qianqian)(nil)
var _ provider.PlaylistCategoryProvider = (*soda.Soda)(nil)
var _ provider.PlaylistCategoryProvider = (*fivesing.Fivesing)(nil)
var _ provider.PlaylistCategoryProvider = (*jamendo.Jamendo)(nil)
var _ provider.PlaylistCategoryProvider = (*joox.Joox)(nil)
var _ provider.PlaylistCategoryProvider = (*bilibili.Bilibili)(nil)

var _ provider.UserPlaylistProvider = (*netease.Netease)(nil)
var _ provider.UserPlaylistProvider = (*qq.QQ)(nil)
var _ provider.UserPlaylistProvider = (*kugou.Kugou)(nil)
var _ provider.UserPlaylistProvider = (*kuwo.Kuwo)(nil)
var _ provider.UserPlaylistProvider = (*migu.Migu)(nil)
var _ provider.UserPlaylistProvider = (*qianqian.Qianqian)(nil)
var _ provider.UserPlaylistProvider = (*soda.Soda)(nil)
var _ provider.UserPlaylistProvider = (*fivesing.Fivesing)(nil)
var _ provider.UserPlaylistProvider = (*jamendo.Jamendo)(nil)
var _ provider.UserPlaylistProvider = (*joox.Joox)(nil)
var _ provider.UserPlaylistProvider = (*bilibili.Bilibili)(nil)

var _ provider.QRLoginProvider = (*netease.Netease)(nil)
var _ provider.QRLoginProvider = (*qq.QQ)(nil)
var _ provider.QRLoginProvider = (*kugou.Kugou)(nil)
var _ provider.QRLoginProvider = (*bilibili.Bilibili)(nil)

var _ provider.RecommendedPlaylistProvider = (*netease.Netease)(nil)
var _ provider.RecommendedPlaylistProvider = (*qq.QQ)(nil)
var _ provider.RecommendedPlaylistProvider = (*kugou.Kugou)(nil)
var _ provider.RecommendedPlaylistProvider = (*kuwo.Kuwo)(nil)
var _ provider.RecommendedPlaylistProvider = (*soda.Soda)(nil)

func TestPlaylistProviderTypeContracts(t *testing.T) {
	playlistProviders := []provider.PlaylistProvider{
		netease.New(""),
		qq.New(""),
		kugou.New(""),
		kuwo.New(""),
		migu.New(""),
		qianqian.New(""),
		soda.New(""),
		fivesing.New(""),
		jamendo.New(""),
		joox.New(""),
		bilibili.New(""),
	}
	categoryProviders := []provider.PlaylistCategoryProvider{
		netease.New(""),
		qq.New(""),
		kugou.New(""),
		kuwo.New(""),
		migu.New(""),
		qianqian.New(""),
		soda.New(""),
		fivesing.New(""),
		jamendo.New(""),
		joox.New(""),
		bilibili.New(""),
	}

	if len(playlistProviders) != len(categoryProviders) {
		t.Fatalf("provider count mismatch: playlists=%d categories=%d", len(playlistProviders), len(categoryProviders))
	}
}
