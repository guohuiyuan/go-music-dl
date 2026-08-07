package jamendo

import (
	"errors"
	"github.com/guohuiyuan/music-lib/model"
)

func GetDownloadURL(s *model.Song) (string, error) { return defaultJamendo.GetDownloadURL(s) }

func (j *Jamendo) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "jamendo" {
		return "", errors.New("source mismatch")
	}
	if s.URL != "" {
		return s.URL, nil
	}

	trackID := s.ID
	if s.Extra != nil && s.Extra["track_id"] != "" {
		trackID = s.Extra["track_id"]
	}
	if trackID == "" {
		return "", errors.New("id missing")
	}

	info, err := j.getTrackByID(trackID, jamendoTrackMeta{
		ArtistName: s.Artist,
		AlbumName:  s.Album,
		AlbumID:    s.AlbumID,
	})
	if err != nil {
		return "", err
	}
	return info.URL, nil
}
