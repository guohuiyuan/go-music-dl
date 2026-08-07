package soda

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

func GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return defaultSoda.GetUserPlaylists(page, limit)
}

type sodaPCMeResponse struct {
	StatusCode int               `json:"status_code"`
	StatusInfo sodaAPIStatusInfo `json:"status_info"`
	MyInfo     struct {
		ID         string    `json:"id"`
		Nickname   string    `json:"nickname"`
		PublicName string    `json:"public_name"`
		Avatar     sodaImage `json:"larger_avatar_url"`
	} `json:"my_info"`
}

type sodaUserPlaylistResponse struct {
	StatusCode int                    `json:"status_code"`
	StatusInfo sodaAPIStatusInfo      `json:"status_info"`
	NextCursor string                 `json:"next_cursor"`
	HasMore    bool                   `json:"has_more"`
	Playlists  []sodaUserPlaylistItem `json:"playlists"`
}

func (s *Soda) GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	if strings.TrimSpace(s.cookie) == "" {
		return nil, errors.New("soda user playlists require cookie")
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	me, err := s.fetchPCMe()
	if err != nil {
		return nil, err
	}
	userID := strings.TrimSpace(me.MyInfo.ID)
	if userID == "" {
		return nil, errors.New("soda user playlists require logged-in user id")
	}

	targetCount := page * limit
	requestCount := targetCount
	if requestCount < 50 {
		requestCount = 50
	}
	if requestCount > 100 {
		requestCount = 100
	}

	cursor := ""
	seenCursors := make(map[string]bool)
	seenPlaylists := make(map[string]bool)
	playlists := make([]model.Playlist, 0, targetCount)
	for attempts := 0; attempts < 20 && len(playlists) < targetCount; attempts++ {
		resp, err := s.fetchUserPlaylistPage(userID, cursor, requestCount)
		if err != nil {
			return nil, err
		}
		for _, item := range resp.Playlists {
			pl := sodaBuildPlaylistFromUserItem(item, userID, me.MyInfo.Nickname)
			if pl.ID == "" || seenPlaylists[pl.ID] {
				continue
			}
			seenPlaylists[pl.ID] = true
			playlists = append(playlists, pl)
		}

		nextCursor := strings.TrimSpace(resp.NextCursor)
		if nextCursor == "" || nextCursor == cursor || seenCursors[nextCursor] {
			break
		}
		if !resp.HasMore && len(resp.Playlists) < requestCount {
			break
		}
		seenCursors[nextCursor] = true
		cursor = nextCursor
	}

	start := (page - 1) * limit
	if start >= len(playlists) {
		return []model.Playlist{}, nil
	}
	end := start + limit
	if end > len(playlists) {
		end = len(playlists)
	}
	return playlists[start:end], nil
}

func (s *Soda) fetchPCMe() (*sodaPCMeResponse, error) {
	body, err := utils.Get(sodaPCMeURL(), s.pcRequestOptions()...)
	if err != nil {
		return nil, err
	}

	var resp sodaPCMeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda me json parse error: %w", err)
	}
	if resp.StatusCode != 0 {
		msg := strings.TrimSpace(resp.StatusInfo.StatusMsg)
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("soda me api error: status_code=%d status_msg=%s", resp.StatusCode, msg)
	}
	return &resp, nil
}

func (s *Soda) fetchUserPlaylistPage(userID, cursor string, count int) (*sodaUserPlaylistResponse, error) {
	body, err := utils.Get(sodaPCUserPlaylistURL(userID, cursor, count), s.pcRequestOptions()...)
	if err != nil {
		return nil, err
	}

	var resp sodaUserPlaylistResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda user playlist json parse error: %w", err)
	}
	if resp.StatusCode != 0 {
		msg := strings.TrimSpace(resp.StatusInfo.StatusMsg)
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("soda user playlist api error: status_code=%d status_msg=%s", resp.StatusCode, msg)
	}
	return &resp, nil
}

func sodaBuildPlaylistFromUserItem(item sodaUserPlaylistItem, currentUserID, currentNickname string) model.Playlist {
	playlistID := strings.TrimSpace(item.ID)
	if playlistID == "" {
		return model.Playlist{}
	}

	name := sodaFirstNonEmpty(item.Title, item.PublicTitle, playlistID)
	creator := sodaFirstNonEmpty(item.Owner.PublicName, item.Owner.Nickname, currentNickname, currentUserID)
	trackCount := item.CountTracks
	if trackCount == 0 {
		trackCount = item.ResourceCnt.TrackCnt
	}
	playCount := item.PlayCount
	if playCount == 0 {
		playCount = item.Stats.CountPlayed
	}

	extra := map[string]string{
		"user_id": currentUserID,
		"type":    strconv.Itoa(item.Type),
	}
	if ownerID := strings.TrimSpace(item.Owner.ID); ownerID != "" {
		extra["owner_id"] = ownerID
	}
	if publicTitle := strings.TrimSpace(item.PublicTitle); publicTitle != "" {
		extra["public_title"] = publicTitle
	}
	if reviewStatus := strings.TrimSpace(item.ReviewStatus); reviewStatus != "" {
		extra["review_status"] = reviewStatus
	}
	if item.Stats.CountCollected > 0 {
		extra["collect_count"] = strconv.Itoa(item.Stats.CountCollected)
	}

	return model.Playlist{
		Source:      "soda",
		ID:          playlistID,
		Name:        name,
		Cover:       sodaBuildImageURL(item.URLCover, "~c5_300x300.jpg"),
		TrackCount:  trackCount,
		PlayCount:   playCount,
		Creator:     creator,
		Description: strings.TrimSpace(item.Desc),
		Link:        fmt.Sprintf("https://www.qishui.com/playlist/%s", playlistID),
		Extra:       extra,
	}
}

func sodaFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
