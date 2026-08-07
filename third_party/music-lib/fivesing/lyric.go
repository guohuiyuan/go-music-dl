package fivesing

import (
	"encoding/json"
	"errors"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"strings"
)

func GetLyrics(s *model.Song) (string, error) { return defaultFivesing.GetLyrics(s) }

func (f *Fivesing) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "fivesing" {
		return "", errors.New("source mismatch")
	}

	var songID, songType string
	if s.Extra != nil {
		songID = s.Extra["songid"]
		songType = s.Extra["songtype"]
	} else {
		parts := strings.Split(s.ID, "|")
		if len(parts) == 2 {
			songID = parts[0]
			songType = parts[1]
		}
	}

	if songID == "" {
		return "", errors.New("invalid id")
	}

	params := url.Values{}
	params.Set("songid", songID)
	params.Set("songtype", songType)
	apiURL := "http://mobileapi.5sing.kugou.com/song/newget?" + params.Encode()

	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", f.cookie))
	if err != nil {
		return "", err
	}

	var resp struct {
		Data struct {
			DynamicWords string `json:"dynamicWords"`
		} `json:"data"`
	}
	json.Unmarshal(body, &resp)
	if resp.Data.DynamicWords == "" {
		return "", errors.New("lyrics not found")
	}
	return resp.Data.DynamicWords, nil
}
