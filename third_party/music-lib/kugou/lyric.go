package kugou

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/lyrics"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"strings"
)

func GetLyrics(s *model.Song) (string, error) { return defaultKugou.GetLyrics(s) }

// GetLyrics 获得歌词
func (k *Kugou) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "kugou" {
		return "", errors.New("source mismatch")
	}

	hash := s.ID
	if s.Extra != nil && s.Extra["hash"] != "" {
		hash = s.Extra["hash"]
	}

	searchURL := fmt.Sprintf("http://krcs.kugou.com/search?ver=1&client=mobi&duration=%d&hash=%s&album_audio_id=", s.Duration*1000, hash)

	body, err := utils.Get(searchURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return "", err
	}

	var searchResp struct {
		Status     int `json:"status"`
		Candidates []struct {
			ID        interface{} `json:"id"`
			AccessKey string      `json:"accesskey"`
			Song      string      `json:"song"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &searchResp); err != nil {
		return "", fmt.Errorf("search lyrics json parse error: %w", err)
	}

	if len(searchResp.Candidates) == 0 {
		return "", errors.New("lyrics not found")
	}

	candidate := searchResp.Candidates[0]
	downloadURL := fmt.Sprintf("http://lyrics.kugou.com/download?ver=1&client=pc&id=%v&accesskey=%s&fmt=krc&charset=utf8", candidate.ID, candidate.AccessKey)

	lrcBody, err := utils.Get(downloadURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return "", err
	}

	var downloadResp struct {
		Status      int    `json:"status"`
		Content     string `json:"content"`
		Fmt         string `json:"fmt"`
		ContentType int    `json:"contenttype"`
	}
	if err := json.Unmarshal(lrcBody, &downloadResp); err != nil {
		return "", fmt.Errorf("download lyrics json parse error: %w", err)
	}
	if downloadResp.Content == "" {
		return "", errors.New("lyrics content is empty")
	}

	tags := map[string]string{
		"ti": s.Name,
		"ar": s.Artist,
		"al": s.Album,
	}
	var data lyrics.MultiData
	if downloadResp.ContentType == 2 || downloadResp.Fmt == "lrc" {
		decodedBytes, err := base64.StdEncoding.DecodeString(downloadResp.Content)
		if err != nil {
			return "", fmt.Errorf("base64 decode error: %w", err)
		}
		lrcTags, lrcData := lyrics.ParseLRC(string(decodedBytes))
		for k, v := range lrcTags {
			if strings.TrimSpace(tags[k]) == "" {
				tags[k] = v
			}
		}
		data = lyrics.MultiData{"orig": lrcData}
	} else {
		krc, err := lyrics.DecodeKRCBase64(downloadResp.Content)
		if err != nil {
			return "", fmt.Errorf("krc decode error: %w", err)
		}
		krcTags, krcData := lyrics.ParseKRC(krc)
		for k, v := range krcTags {
			if strings.TrimSpace(tags[k]) == "" {
				tags[k] = v
			}
		}
		data = krcData
	}
	if len(data["orig"]) == 0 {
		return "", errors.New("lyrics content is empty")
	}
	return lyrics.ConvertVerbatimLRC(tags, data, lyrics.DefaultDisplayOrder()), nil
}
