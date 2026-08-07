package kuwo

import "github.com/guohuiyuan/music-lib/model"

func GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return defaultKuwo.GetUserPlaylists(page, limit)
}

func (p *Kuwo) GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return nil, model.ErrUserPlaylistsUnsupported
}
