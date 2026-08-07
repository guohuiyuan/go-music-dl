package jamendo

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"regexp"
)

func Search(keyword string) ([]model.Song, error) { return defaultJamendo.Search(keyword) }

func Parse(link string) (*model.Song, error) { return defaultJamendo.Parse(link) }

func (j *Jamendo) Search(keyword string) ([]model.Song, error) {
	body, err := j.searchByType(keyword, "track")
	if err != nil {
		return nil, err
	}

	var results []jamendoTrackItem
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("jamendo json parse error: %w", err)
	}

	songs := make([]model.Song, 0, len(results))
	for _, item := range results {
		song := buildSong(item, jamendoTrackMeta{})
		if song == nil {
			continue
		}
		songs = append(songs, *song)
	}
	return songs, nil
}

func (j *Jamendo) Parse(link string) (*model.Song, error) {
	re := regexp.MustCompile(`jamendo\.com/track/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid jamendo link")
	}

	return j.getTrackByID(matches[1], jamendoTrackMeta{})
}
