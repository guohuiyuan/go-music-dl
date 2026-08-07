package qianqian

import "github.com/guohuiyuan/music-lib/model"

func GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return defaultQianqian.GetUserPlaylists(page, limit)
}

func (p *Qianqian) GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return nil, model.ErrUserPlaylistsUnsupported
}
