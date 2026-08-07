package bilibili

import (
	"github.com/guohuiyuan/music-lib/model"
)

func GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return defaultBilibili.GetUserPlaylists(page, limit)
}

func (b *Bilibili) GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return nil, model.ErrUserPlaylistsUnsupported
}
