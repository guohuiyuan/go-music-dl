package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
)

const (
	aiPlayerCommandBuffer    = 16
	aiPlayerRequestLimit     = 64 << 10
	aiPlayerMaxClients       = 64
	aiPlayerClientIDMaxRunes = 128
	aiPlayerDetachedTTL      = 10 * time.Minute
)

// 多个播放器同时在线时，没有指定 client_id 的命令会被拒绝，避免一呼百应。
var (
	errAIPlayerNoClient        = errors.New("player not connected")
	errAIPlayerUnknownClient   = errors.New("client is not connected")
	errAIPlayerAmbiguousClient = errors.New("multiple players connected")
	errAIPlayerBusy            = errors.New("player command queue is full")
	errAIPlayerCapacity        = errors.New("too many player clients")
	errAIPlayerInvalidClientID = errors.New("invalid player client id")
)

type aiPlayerSources []string

func (s *aiPlayerSources) UnmarshalJSON(data []byte) error {
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		*s = list
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("sources must be a string or string array")
	}
	*s = []string{value}
	return nil
}

type aiPlayerCommand struct {
	ID       string   `json:"id"`
	Action   string   `json:"action"`
	ClientID string   `json:"client_id,omitempty"`
	Song     *aiSong  `json:"song,omitempty"`
	Position *float64 `json:"position,omitempty"`
	Volume   *float64 `json:"volume,omitempty"`
	Rate     *float64 `json:"rate,omitempty"`
	IssuedAt string   `json:"issued_at"`
}

type aiPlayerState struct {
	ClientID      string  `json:"client_id,omitempty"`
	Ready         bool    `json:"ready"`
	Playing       bool    `json:"playing"`
	Current       *aiSong `json:"current,omitempty"`
	Position      float64 `json:"position"`
	Duration      float64 `json:"duration"`
	Volume        float64 `json:"volume"`
	PlaybackRate  float64 `json:"playback_rate"`
	QueueLength   int     `json:"queue_length"`
	QueueIndex    int     `json:"queue_index"`
	PageVisible   bool    `json:"page_visible"`
	PlaybackError string  `json:"last_error,omitempty"`
	UpdatedAt     string  `json:"updated_at"`
}

type aiPlayerSummary struct {
	ClientID    string  `json:"client_id"`
	Connected   bool    `json:"connected"`
	Ready       bool    `json:"ready"`
	Playing     bool    `json:"playing"`
	Current     *aiSong `json:"current,omitempty"`
	Position    float64 `json:"position"`
	Duration    float64 `json:"duration"`
	QueueLength int     `json:"queue_length"`
	PageVisible bool    `json:"page_visible"`
	UserAgent   string  `json:"user_agent,omitempty"`
	RemoteAddr  string  `json:"remote_addr,omitempty"`
	UpdatedAt   string  `json:"updated_at,omitempty"`
}

type aiPlayerSnapshot struct {
	Available        bool              `json:"available"`
	ConnectedClients int               `json:"connected_clients"`
	SelectedClientID string            `json:"selected_client_id,omitempty"`
	Player           *aiPlayerState    `json:"player"`
	Players          []aiPlayerState   `json:"players,omitempty"`
	Clients          []aiPlayerSummary `json:"clients,omitempty"`
	LastCommand      *aiPlayerCommand  `json:"last_command,omitempty"`
}

// aiPlayerClient 既可以是已经接了 SSE 的在线播放器，也可以只是上报过状态的记录。
type aiPlayerClient struct {
	id          string
	ch          chan aiPlayerCommand
	hasState    bool
	state       aiPlayerState
	seq         uint64
	connectedAt time.Time
	detachedAt  time.Time
	lastCommand aiPlayerCommand
	hasCommand  bool
	userAgent   string
	remoteAddr  string
}

func (c *aiPlayerClient) summary() aiPlayerSummary {
	summary := aiPlayerSummary{
		ClientID:   c.id,
		Connected:  c.ch != nil,
		UserAgent:  c.userAgent,
		RemoteAddr: c.remoteAddr,
	}
	if c.hasState {
		summary.Ready = c.state.Ready
		summary.Playing = c.state.Playing
		summary.Current = c.state.Current
		summary.Position = c.state.Position
		summary.Duration = c.state.Duration
		summary.QueueLength = c.state.QueueLength
		summary.PageVisible = c.state.PageVisible
		summary.UpdatedAt = c.state.UpdatedAt
	}
	return summary
}

type aiPlayerHub struct {
	mu      sync.Mutex
	clients map[string]*aiPlayerClient
	counter atomic.Uint64
	anon    atomic.Uint64
	seq     atomic.Uint64
}

func newAIPlayerHub() *aiPlayerHub {
	return &aiPlayerHub{clients: make(map[string]*aiPlayerClient)}
}

// subscribe 用 client_id 建立连接身份；不带 id 时退化成 anon-N。
func (h *aiPlayerHub) subscribe(clientID string) (<-chan aiPlayerCommand, string, func(), error) {
	normalizedID, err := normalizeAIPlayerClientID(clientID, true)
	if err != nil {
		return nil, "", nil, err
	}
	ch := make(chan aiPlayerCommand, aiPlayerCommandBuffer)

	h.mu.Lock()
	now := time.Now()
	h.pruneLocked(now)
	id := normalizedID
	if id == "" {
		id = h.nextAnonymousIDLocked()
	}
	client := h.clients[id]
	if (client == nil || client.ch == nil) && h.connectedCountLocked() >= aiPlayerMaxClients {
		h.mu.Unlock()
		return nil, "", nil, errAIPlayerCapacity
	}
	if client == nil {
		client = &aiPlayerClient{id: id}
		h.clients[id] = client
	}
	if client.ch != nil {
		// 同一个 client_id 重连：旧连接让位给新连接。
		close(client.ch)
	}
	client.ch = ch
	client.connectedAt = now
	client.detachedAt = time.Time{}
	h.evictLocked()
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		current := h.clients[id]
		if current == nil || current.ch != ch {
			return
		}
		current.ch = nil
		current.detachedAt = time.Now()
		if !current.hasState {
			delete(h.clients, id)
		}
	}
	return ch, id, unsubscribe, nil
}

func (h *aiPlayerHub) nextAnonymousIDLocked() string {
	for {
		id := fmt.Sprintf("anon-%d", h.anon.Add(1))
		if _, exists := h.clients[id]; !exists {
			return id
		}
	}
}

func (h *aiPlayerHub) setClientMeta(clientID, userAgent, remoteAddr string) {
	if clientID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if client := h.clients[clientID]; client != nil {
		client.userAgent = truncate(userAgent, 200)
		client.remoteAddr = truncate(remoteAddr, 80)
	}
}

func (h *aiPlayerHub) clientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruneLocked(time.Now())
	return h.connectedCountLocked()
}

func (h *aiPlayerHub) connectedCountLocked() int {
	count := 0
	for _, client := range h.clients {
		if client.ch != nil {
			count++
		}
	}
	return count
}

// dispatch 定向投递；target 为空且只有一台在线时才隐式定向。
func (h *aiPlayerHub) dispatch(command aiPlayerCommand, target string) (aiPlayerCommand, int, error) {
	command.ID = fmt.Sprintf("cmd-%d-%d", time.Now().UnixNano(), h.counter.Add(1))
	command.IssuedAt = time.Now().UTC().Format(time.RFC3339Nano)
	target = strings.TrimSpace(target)

	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruneLocked(time.Now())

	var client *aiPlayerClient
	if target != "" {
		client = h.clients[target]
		if client == nil || client.ch == nil {
			return aiPlayerCommand{}, 0, errAIPlayerUnknownClient
		}
		command.ClientID = target
	} else {
		var connected []*aiPlayerClient
		for _, client := range h.clients {
			if client.ch != nil {
				connected = append(connected, client)
			}
		}
		switch len(connected) {
		case 0:
			return aiPlayerCommand{}, 0, errAIPlayerNoClient
		case 1:
			client = connected[0]
			command.ClientID = client.id
		default:
			return aiPlayerCommand{}, 0, errAIPlayerAmbiguousClient
		}
	}

	select {
	case client.ch <- command:
		client.lastCommand = command
		client.hasCommand = true
		return command, 1, nil
	default:
		return aiPlayerCommand{}, 0, errAIPlayerBusy
	}
}

func (h *aiPlayerHub) updateState(state aiPlayerState) {
	now := time.Now()
	state.UpdatedAt = now.UTC().Format(time.RFC3339Nano)
	clientID := strings.TrimSpace(state.ClientID)
	if clientID == "" {
		clientID = "unknown"
	}
	state.ClientID = clientID

	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruneLocked(now)

	client := h.clients[clientID]
	if client == nil {
		// 老页面先上报状态、后带 client_id 建 SSE 时，把匿名连接认领过来。
		client = h.adoptAnonymousLocked(clientID)
	}
	if client == nil {
		client = &aiPlayerClient{id: clientID, detachedAt: now}
		h.clients[clientID] = client
	}
	client.state = state
	client.hasState = true
	client.seq = h.seq.Add(1)
	if client.ch == nil {
		client.detachedAt = now
	}
	h.evictLocked()
}

func (h *aiPlayerHub) adoptAnonymousLocked(clientID string) *aiPlayerClient {
	if clientID == "" {
		return nil
	}
	var candidate *aiPlayerClient
	for id, client := range h.clients {
		if !strings.HasPrefix(id, "anon-") || client.ch == nil || client.hasState {
			continue
		}
		if candidate != nil {
			return nil
		}
		candidate = client
	}
	if candidate == nil {
		return nil
	}
	delete(h.clients, candidate.id)
	candidate.id = clientID
	h.clients[clientID] = candidate
	return candidate
}

func (h *aiPlayerHub) evictLocked() {
	if len(h.clients) <= aiPlayerMaxClients {
		return
	}
	detached := make([]*aiPlayerClient, 0, len(h.clients))
	for _, client := range h.clients {
		if client.ch == nil {
			detached = append(detached, client)
		}
	}
	sort.Slice(detached, func(i, j int) bool {
		return detached[i].detachedAt.Before(detached[j].detachedAt)
	})
	for _, client := range detached {
		if len(h.clients) <= aiPlayerMaxClients {
			break
		}
		delete(h.clients, client.id)
	}
}

func (h *aiPlayerHub) pruneLocked(now time.Time) {
	for id, client := range h.clients {
		if client.ch != nil || client.detachedAt.IsZero() {
			continue
		}
		if now.Sub(client.detachedAt) >= aiPlayerDetachedTTL {
			delete(h.clients, id)
		}
	}
}

func (h *aiPlayerHub) snapshot(selector string) aiPlayerSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruneLocked(time.Now())

	clients := make([]*aiPlayerClient, 0, len(h.clients))
	for _, client := range h.clients {
		clients = append(clients, client)
	}
	sort.SliceStable(clients, func(i, j int) bool {
		return preferredAIPlayerClient(clients[i], clients[j])
	})

	snapshot := aiPlayerSnapshot{}
	for _, client := range clients {
		if client.ch != nil {
			snapshot.ConnectedClients++
		}
		snapshot.Clients = append(snapshot.Clients, client.summary())
		if client.hasState {
			snapshot.Players = append(snapshot.Players, client.state)
		}
	}
	snapshot.Available = snapshot.ConnectedClients > 0

	selector = strings.TrimSpace(selector)
	if selector != "" {
		if client := h.clients[selector]; client != nil {
			snapshot.SelectedClientID = selector
			if client.hasState {
				state := client.state
				snapshot.Player = &state
			}
		}
	} else {
		for _, client := range clients {
			if !client.hasState {
				continue
			}
			state := client.state
			snapshot.Player = &state
			snapshot.SelectedClientID = state.ClientID
			break
		}
	}
	if snapshot.SelectedClientID != "" {
		if client := h.clients[snapshot.SelectedClientID]; client != nil && client.hasCommand {
			command := client.lastCommand
			snapshot.LastCommand = &command
		}
	}
	return snapshot
}

func preferredAIPlayerClient(a, b *aiPlayerClient) bool {
	rankA := aiPlayerClientRank(a)
	rankB := aiPlayerClientRank(b)
	if rankA != rankB {
		return rankA > rankB
	}
	if a.ch != nil && b.ch != nil {
		if a.hasState != b.hasState {
			return a.hasState
		}
		if !a.connectedAt.Equal(b.connectedAt) {
			return a.connectedAt.Before(b.connectedAt)
		}
		return a.id < b.id
	}
	if a.hasState != b.hasState {
		return a.hasState
	}
	if a.seq != b.seq {
		return a.seq > b.seq
	}
	return a.id < b.id
}

func aiPlayerClientRank(client *aiPlayerClient) int {
	rank := 0
	if client.ch != nil {
		rank += 4
	}
	if client.hasState && client.state.Playing {
		rank += 2
	}
	if client.hasState && client.state.PageVisible {
		rank++
	}
	return rank
}

func (h *aiPlayerHub) summaries() []aiPlayerSummary {
	snapshot := h.snapshot("")
	return snapshot.Clients
}

func registerAIPlayerRoutes(api *gin.RouterGroup, providers aiSearchProviders) *aiPlayerHub {
	hub := newAIPlayerHub()
	api.GET("/api/ai/player/state", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		selector, err := normalizeAIPlayerClientID(c.Query("client_id"), true)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, hub.snapshot(selector))
	})
	api.GET("/api/ai/player/events", aiPlayerEventsHandler(hub))
	api.POST("/api/ai/player/play", aiPlayerPlayHandler(hub, providers))
	api.POST("/api/ai/player/command", aiPlayerCommandHandler(hub))
	api.POST("/api/ai/player/state", aiPlayerStateHandler(hub))
	return hub
}

func aiPlayerEventsHandler(hub *aiPlayerHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		commands, clientID, unsubscribe, err := hub.subscribe(c.Query("client_id"))
		if err != nil {
			switch {
			case errors.Is(err, errAIPlayerCapacity):
				c.Header("Retry-After", "1")
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"error":   "player_capacity_reached",
					"message": "too many player clients are connected",
				})
			default:
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "invalid_client_id",
					"message": err.Error(),
				})
			}
			return
		}
		defer unsubscribe()
		hub.setClientMeta(clientID, c.Request.UserAgent(), c.ClientIP())

		c.Header("Cache-Control", "no-cache, no-store, no-transform")
		c.Header("Connection", "keep-alive")
		c.Header("Content-Type", "text/event-stream; charset=utf-8")
		c.Header("X-Accel-Buffering", "no")
		c.Header("X-Player-Client-Id", clientID)
		c.Status(http.StatusOK)
		c.Writer.Flush()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-c.Request.Context().Done():
				return
			case command, ok := <-commands:
				if !ok {
					return
				}
				data, err := json.Marshal(command)
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(c.Writer, "id: %s\nevent: player_command\ndata: %s\n\n", command.ID, data); err != nil {
					return
				}
				c.Writer.Flush()
			case <-ticker.C:
				if _, err := c.Writer.WriteString(": heartbeat\n\n"); err != nil {
					return
				}
				c.Writer.Flush()
			}
		}
	}
}

type aiPlayerPlayRequest struct {
	ClientID       string            `json:"client_id"`
	TargetClientID string            `json:"target_client_id"`
	Query          string            `json:"query"`
	ID             string            `json:"id"`
	Source         string            `json:"source"`
	Name           string            `json:"name"`
	Artist         string            `json:"artist"`
	Album          string            `json:"album"`
	Cover          string            `json:"cover"`
	Duration       int               `json:"duration"`
	Extra          map[string]string `json:"extra"`
	Sources        aiPlayerSources   `json:"sources"`
}

func aiPlayerPlayHandler(hub *aiPlayerHub, providers aiSearchProviders) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !allowAIPlayerWrite(c, hub) {
			return
		}

		var request aiPlayerPlayRequest
		if !bindAIPlayerJSON(c, &request) {
			return
		}
		target, err := resolveAIPlayerTarget(request.ClientID, request.TargetClientID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id", "message": err.Error()})
			return
		}

		var song aiSong
		id := strings.TrimSpace(request.ID)
		source := strings.TrimSpace(request.Source)
		switch {
		case id != "" || source != "":
			if id == "" || source == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_song", "message": "id and source must be provided together"})
				return
			}
			allowed := append(coreSongSources(), "local")
			normalizedSource, err := parseAISingleSource(source, allowed)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_source", "message": err.Error()})
				return
			}
			song = aiSong{
				ID:       id,
				Source:   normalizedSource,
				Name:     firstNonEmpty(request.Name, request.Query),
				Artist:   strings.TrimSpace(request.Artist),
				Album:    strings.TrimSpace(request.Album),
				Duration: request.Duration,
				Extra:    aiPlaybackExtra(request.Extra),
			}
		default:
			query := strings.TrimSpace(request.Query)
			if query == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing_query", "message": "provide query or both id and source"})
				return
			}
			sources, err := parseAIPlayerSources(request.Sources)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_sources", "message": err.Error()})
				return
			}
			result := executeAISongSearch(query, sources, strings.TrimSpace(request.Artist), 1, providers)
			if len(result.Songs) == 0 {
				c.JSON(http.StatusNotFound, gin.H{
					"error":   "song_not_found",
					"message": "no playable search result was found",
					"sources": sources,
					"errors":  result.Errors,
				})
				return
			}
			song = result.Songs[0]
		}

		command, delivered, err := hub.dispatch(aiPlayerCommand{Action: "play_song", Song: &song}, target)
		if err != nil {
			writeAIPlayerDispatchError(c, hub, err)
			return
		}
		c.JSON(http.StatusAccepted, gin.H{
			"status":            "accepted",
			"client_id":         command.ClientID,
			"command":           command,
			"delivered_clients": delivered,
		})
	}
}

func resolveAIPlayerTarget(clientID, targetClientID string) (string, error) {
	directTarget, err := normalizeAIPlayerClientID(clientID, true)
	if err != nil {
		return "", err
	}
	compatTarget, err := normalizeAIPlayerClientID(targetClientID, true)
	if err != nil {
		return "", err
	}
	if directTarget != "" && compatTarget != "" && directTarget != compatTarget {
		return "", fmt.Errorf("%w: client_id and target_client_id must match", errAIPlayerInvalidClientID)
	}
	return firstNonEmpty(directTarget, compatTarget), nil
}

type aiPlayerControlRequest struct {
	ClientID       string   `json:"client_id"`
	TargetClientID string   `json:"target_client_id"`
	Action         string   `json:"action"`
	Position       *float64 `json:"position"`
	Volume         *float64 `json:"volume"`
	Rate           *float64 `json:"rate"`
}

func aiPlayerCommandHandler(hub *aiPlayerHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !allowAIPlayerWrite(c, hub) {
			return
		}

		var request aiPlayerControlRequest
		if !bindAIPlayerJSON(c, &request) {
			return
		}
		target, err := resolveAIPlayerTarget(request.ClientID, request.TargetClientID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id", "message": err.Error()})
			return
		}

		action := strings.ToLower(strings.TrimSpace(request.Action))
		command := aiPlayerCommand{Action: action}
		switch action {
		case "play", "pause", "toggle", "stop", "next", "previous":
		case "seek":
			if request.Position == nil || *request.Position < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_position", "message": "position must be zero or greater"})
				return
			}
			command.Position = request.Position
		case "volume":
			if request.Volume == nil || *request.Volume < 0 || *request.Volume > 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_volume", "message": "volume must be between 0 and 1"})
				return
			}
			command.Volume = request.Volume
		case "rate":
			if request.Rate == nil || !validAIPlayerRate(*request.Rate) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_rate", "message": "rate must be one of 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.25, 2.5, 2.75, or 3"})
				return
			}
			command.Rate = request.Rate
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_action",
				"message": "action must be play, pause, toggle, stop, next, previous, seek, volume, or rate",
			})
			return
		}

		command, delivered, err := hub.dispatch(command, target)
		if err != nil {
			writeAIPlayerDispatchError(c, hub, err)
			return
		}
		c.JSON(http.StatusAccepted, gin.H{
			"status":            "accepted",
			"client_id":         command.ClientID,
			"command":           command,
			"delivered_clients": delivered,
		})
	}
}

func writeAIPlayerDispatchError(c *gin.Context, hub *aiPlayerHub, err error) {
	switch {
	case errors.Is(err, errAIPlayerBusy):
		c.Header("Retry-After", "1")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "player_busy",
			"message": "the selected player is not consuming commands fast enough; retry shortly",
		})
	case errors.Is(err, errAIPlayerAmbiguousClient):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "ambiguous_client",
			"message": "multiple players are connected; pass client_id to choose one",
			"clients": hub.summaries(),
		})
	case errors.Is(err, errAIPlayerUnknownClient):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "client_not_connected",
			"message": "the requested client_id is not connected",
			"clients": hub.summaries(),
		})
	default:
		c.JSON(http.StatusConflict, gin.H{
			"error":   "player_not_connected",
			"message": "open the web or desktop player before sending playback commands",
		})
	}
}

type aiPlayerStateRequest struct {
	ClientID      string  `json:"client_id"`
	Ready         bool    `json:"ready"`
	Playing       bool    `json:"playing"`
	Current       *aiSong `json:"current"`
	Position      float64 `json:"position"`
	Duration      float64 `json:"duration"`
	Volume        float64 `json:"volume"`
	PlaybackRate  float64 `json:"playback_rate"`
	QueueLength   int     `json:"queue_length"`
	QueueIndex    int     `json:"queue_index"`
	PageVisible   bool    `json:"page_visible"`
	PlaybackError string  `json:"last_error"`
}

func aiPlayerStateHandler(hub *aiPlayerHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !allowSameOriginWrite(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		var request aiPlayerStateRequest
		if !bindAIPlayerJSON(c, &request) {
			return
		}
		if request.Position < 0 || request.Duration < 0 || request.Volume < 0 || request.Volume > 1 ||
			request.QueueLength < 0 || request.QueueIndex < -1 || request.PlaybackRate < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_player_state"})
			return
		}
		normalizedClientID, err := normalizeAIPlayerClientID(request.ClientID, true)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id", "message": err.Error()})
			return
		}
		request.ClientID = normalizedClientID
		if len(request.PlaybackError) > 500 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_player_state"})
			return
		}
		if request.Current != nil {
			request.Current.Extra = aiPlaybackExtra(request.Current.Extra)
		}

		hub.updateState(aiPlayerState{
			ClientID:      request.ClientID,
			Ready:         request.Ready,
			Playing:       request.Playing,
			Current:       request.Current,
			Position:      request.Position,
			Duration:      request.Duration,
			Volume:        request.Volume,
			PlaybackRate:  request.PlaybackRate,
			QueueLength:   request.QueueLength,
			QueueIndex:    request.QueueIndex,
			PageVisible:   request.PageVisible,
			PlaybackError: request.PlaybackError,
		})
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func allowAIPlayerWrite(c *gin.Context, hub *aiPlayerHub) bool {
	if !allowSameOriginWrite(c) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	if hub.clientCount() == 0 {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error":   "player_not_connected",
			"message": "open the web or desktop player before sending playback commands",
		})
		return false
	}
	return true
}

func bindAIPlayerJSON(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, aiPlayerRequestLimit)
	if err := c.ShouldBindJSON(target); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid_json", "message": err.Error()})
		return false
	}
	return true
}

func coreSongSources() []string {
	return aiPlayableSources()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func normalizeAIPlayerClientID(value string, allowEmpty bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if allowEmpty {
			return "", nil
		}
		return "", fmt.Errorf("%w: client_id is required", errAIPlayerInvalidClientID)
	}
	runes := []rune(value)
	if len(runes) > aiPlayerClientIDMaxRunes {
		return "", fmt.Errorf("%w: client_id must be at most %d characters", errAIPlayerInvalidClientID, aiPlayerClientIDMaxRunes)
	}
	for _, r := range runes {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", fmt.Errorf("%w: client_id must not contain whitespace or control characters", errAIPlayerInvalidClientID)
		}
	}
	return value, nil
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func validAIPlayerRate(rate float64) bool {
	switch rate {
	case 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.25, 2.5, 2.75, 3:
		return true
	default:
		return false
	}
}
