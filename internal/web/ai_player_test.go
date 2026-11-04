package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/music-lib/model"
)

func newAIPlayerTestRouter(providers aiSearchProviders) (*gin.Engine, *aiPlayerHub) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	hub := registerAIPlayerRoutes(router.Group("/music"), providers)
	return router, hub
}

func postAIPlayerJSON(t *testing.T, router http.Handler, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func subscribeAIPlayerTest(t *testing.T, hub *aiPlayerHub, clientID string) (<-chan aiPlayerCommand, string, func()) {
	t.Helper()
	commands, id, unsubscribe, err := hub.subscribe(clientID)
	if err != nil {
		t.Fatalf("subscribe %q: %v", clientID, err)
	}
	return commands, id, unsubscribe
}

func receiveAIPlayerCommand(t *testing.T, commands <-chan aiPlayerCommand) aiPlayerCommand {
	t.Helper()
	select {
	case command := <-commands:
		return command
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for player command")
		return aiPlayerCommand{}
	}
}

func TestAIPlayerSearchPlayDispatchesSong(t *testing.T) {
	providers := aiSearchProviders{
		SearchSongs: func(source string) func(string) ([]model.Song, error) {
			return func(query string) ([]model.Song, error) {
				return []model.Song{{
					ID:       "qq-1",
					Name:     query,
					Artist:   "周杰伦",
					Album:    "叶惠美",
					Duration: 269,
					Extra: map[string]string{
						"songmid": "mid-1",
						"noise":   "drop",
					},
				}}, nil
			}
		},
	}
	router, hub := newAIPlayerTestRouter(providers)
	commands, _, unsubscribe := subscribeAIPlayerTest(t, hub, "")
	defer unsubscribe()

	response := postAIPlayerJSON(t, router, "/music/api/ai/player/play",
		`{"query":"晴天","artist":"周杰伦","sources":"qq"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	command := receiveAIPlayerCommand(t, commands)
	if command.Action != "play_song" || command.Song == nil {
		t.Fatalf("unexpected command: %+v", command)
	}
	if command.Song.ID != "qq-1" || command.Song.Source != "qq" || command.Song.Name != "晴天" {
		t.Fatalf("unexpected song: %+v", command.Song)
	}
	if command.Song.Extra["songmid"] != "mid-1" {
		t.Fatalf("missing playback extra: %+v", command.Song.Extra)
	}
	if _, ok := command.Song.Extra["noise"]; ok {
		t.Fatalf("noise field was not removed: %+v", command.Song.Extra)
	}
}

func TestAIPlayerExplicitSongAndControlCommands(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})
	commands, _, unsubscribe := subscribeAIPlayerTest(t, hub, "")
	defer unsubscribe()

	playResponse := postAIPlayerJSON(t, router, "/music/api/ai/player/play",
		`{"id":"song-1","source":"netease","name":"Song","artist":"Artist"}`)
	if playResponse.Code != http.StatusAccepted {
		t.Fatalf("play status=%d body=%s", playResponse.Code, playResponse.Body.String())
	}
	playCommand := receiveAIPlayerCommand(t, commands)
	if playCommand.Action != "play_song" || playCommand.Song == nil || playCommand.Song.ID != "song-1" {
		t.Fatalf("unexpected play command: %+v", playCommand)
	}

	tests := []struct {
		body     string
		action   string
		position float64
		volume   float64
		rate     float64
	}{
		{body: `{"action":"pause"}`, action: "pause"},
		{body: `{"action":"next"}`, action: "next"},
		{body: `{"action":"seek","position":42.5}`, action: "seek", position: 42.5},
		{body: `{"action":"volume","volume":0.35}`, action: "volume", volume: 0.35},
		{body: `{"action":"rate","rate":1.25}`, action: "rate", rate: 1.25},
	}
	for _, test := range tests {
		response := postAIPlayerJSON(t, router, "/music/api/ai/player/command", test.body)
		if response.Code != http.StatusAccepted {
			t.Fatalf("%s status=%d body=%s", test.action, response.Code, response.Body.String())
		}
		command := receiveAIPlayerCommand(t, commands)
		if command.Action != test.action {
			t.Fatalf("action=%q, want %q", command.Action, test.action)
		}
		if test.action == "seek" && (command.Position == nil || *command.Position != test.position) {
			t.Fatalf("seek position=%v, want %v", command.Position, test.position)
		}
		if test.action == "volume" && (command.Volume == nil || *command.Volume != test.volume) {
			t.Fatalf("volume=%v, want %v", command.Volume, test.volume)
		}
		if test.action == "rate" && (command.Rate == nil || *command.Rate != test.rate) {
			t.Fatalf("rate=%v, want %v", command.Rate, test.rate)
		}
	}
}

func TestAIPlayerStateRoundTrip(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})
	_, _, unsubscribe := subscribeAIPlayerTest(t, hub, "")
	defer unsubscribe()

	body := `{
		"client_id":"tab-1",
		"ready":true,
		"playing":true,
		"current":{"id":"song-1","source":"qq","name":"Song","artist":"Artist","extra":{"songmid":"mid","noise":"drop"}},
		"position":31.5,
		"duration":180,
		"volume":0.65,
		"playback_rate":1.25,
		"queue_length":4,
		"queue_index":1,
		"page_visible":true
	}`
	response := postAIPlayerJSON(t, router, "/music/api/ai/player/state", body)
	if response.Code != http.StatusOK {
		t.Fatalf("state update status=%d body=%s", response.Code, response.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/music/api/ai/player/state", nil)
	stateResponse := httptest.NewRecorder()
	router.ServeHTTP(stateResponse, request)
	if stateResponse.Code != http.StatusOK {
		t.Fatalf("state status=%d body=%s", stateResponse.Code, stateResponse.Body.String())
	}
	var snapshot aiPlayerSnapshot
	if err := json.Unmarshal(stateResponse.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if !snapshot.Available || snapshot.ConnectedClients != 1 || snapshot.Player == nil {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.Player.ClientID != "tab-1" || !snapshot.Player.Playing ||
		snapshot.Player.Position != 31.5 || snapshot.Player.Current == nil ||
		snapshot.Player.Current.ID != "song-1" {
		t.Fatalf("unexpected player state: %+v", snapshot.Player)
	}
	if snapshot.Player.Current.Extra["songmid"] != "mid" {
		t.Fatalf("missing player extra: %+v", snapshot.Player.Current.Extra)
	}
	if _, ok := snapshot.Player.Current.Extra["noise"]; ok {
		t.Fatalf("noise player extra was not removed: %+v", snapshot.Player.Current.Extra)
	}
}

func TestAIPlayerRequiresConnectedClientAndWriteHeader(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})

	response := postAIPlayerJSON(t, router, "/music/api/ai/player/command", `{"action":"pause"}`)
	if response.Code != http.StatusConflict {
		t.Fatalf("no client status=%d body=%s", response.Code, response.Body.String())
	}

	_, _, unsubscribe := subscribeAIPlayerTest(t, hub, "")
	defer unsubscribe()
	request := httptest.NewRequest(http.MethodPost, "/music/api/ai/player/command", bytes.NewBufferString(`{"action":"pause"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://attacker.example")
	forbidden := httptest.NewRecorder()
	router.ServeHTTP(forbidden, request)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("forbidden status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
}

func TestAIPlayerRejectsInvalidControlInput(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})
	_, _, unsubscribe := subscribeAIPlayerTest(t, hub, "")
	defer unsubscribe()

	tests := []struct {
		path string
		body string
		code string
	}{
		{path: "/music/api/ai/player/play", body: `{}`, code: "missing_query"},
		{path: "/music/api/ai/player/play", body: `{"id":"song"}`, code: "invalid_song"},
		{path: "/music/api/ai/player/command", body: `{"action":"unknown"}`, code: "invalid_action"},
		{path: "/music/api/ai/player/command", body: `{"action":"seek","position":-1}`, code: "invalid_position"},
		{path: "/music/api/ai/player/command", body: `{"action":"volume","volume":2}`, code: "invalid_volume"},
		{path: "/music/api/ai/player/command", body: `{"action":"rate","rate":1.1}`, code: "invalid_rate"},
	}
	for _, test := range tests {
		response := postAIPlayerJSON(t, router, test.path, test.body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", test.body, response.Code, response.Body.String())
		}
		var payload struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Error != test.code {
			t.Fatalf("%s error=%q, want %q", test.body, payload.Error, test.code)
		}
	}
}

func expectNoAIPlayerCommand(t *testing.T, commands <-chan aiPlayerCommand) {
	t.Helper()
	select {
	case command := <-commands:
		t.Fatalf("unexpected command for this client: %+v", command)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestAIPlayerTargetsSingleClient(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})

	commandsA, _, unsubscribeA := subscribeAIPlayerTest(t, hub, "tab-a")
	defer unsubscribeA()
	commandsB, _, unsubscribeB := subscribeAIPlayerTest(t, hub, "tab-b")
	defer unsubscribeB()

	response := postAIPlayerJSON(t, router, "/music/api/ai/player/play",
		`{"client_id":"tab-a","id":"song-1","source":"netease","name":"Song","artist":"Artist"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	command := receiveAIPlayerCommand(t, commandsA)
	if command.ClientID != "tab-a" || command.Action != "play_song" || command.Song == nil {
		t.Fatalf("unexpected targeted command: %+v", command)
	}
	expectNoAIPlayerCommand(t, commandsB)

	control := postAIPlayerJSON(t, router, "/music/api/ai/player/command",
		`{"client_id":"tab-b","action":"pause"}`)
	if control.Code != http.StatusAccepted {
		t.Fatalf("control status=%d body=%s", control.Code, control.Body.String())
	}
	pauseCommand := receiveAIPlayerCommand(t, commandsB)
	if pauseCommand.ClientID != "tab-b" || pauseCommand.Action != "pause" {
		t.Fatalf("unexpected control command: %+v", pauseCommand)
	}
	expectNoAIPlayerCommand(t, commandsA)
}

func TestAIPlayerRejectsUntargetedCommandWithMultipleClients(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})
	_, _, unsubscribeA := subscribeAIPlayerTest(t, hub, "tab-a")
	defer unsubscribeA()
	_, _, unsubscribeB := subscribeAIPlayerTest(t, hub, "tab-b")
	defer unsubscribeB()

	response := postAIPlayerJSON(t, router, "/music/api/ai/player/command", `{"action":"pause"}`)
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Error   string            `json:"error"`
		Clients []aiPlayerSummary `json:"clients"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error != "ambiguous_client" || len(payload.Clients) != 2 {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	unknown := postAIPlayerJSON(t, router, "/music/api/ai/player/command",
		`{"client_id":"tab-missing","action":"pause"}`)
	if unknown.Code != http.StatusConflict {
		t.Fatalf("unknown client status=%d body=%s", unknown.Code, unknown.Body.String())
	}
	if err := json.Unmarshal(unknown.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error != "client_not_connected" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestAIPlayerKeepsStatePerClient(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})
	_, _, unsubscribeA := subscribeAIPlayerTest(t, hub, "tab-a")
	defer unsubscribeA()
	_, _, unsubscribeB := subscribeAIPlayerTest(t, hub, "tab-b")
	defer unsubscribeB()

	for _, body := range []string{
		`{"client_id":"tab-a","ready":true,"playing":true,"current":{"id":"song-a","source":"qq","name":"A"},"position":12,"duration":200,"volume":0.5}`,
		`{"client_id":"tab-b","ready":true,"playing":false,"current":{"id":"song-b","source":"netease","name":"B"},"position":40,"duration":300,"volume":0.8}`,
	} {
		response := postAIPlayerJSON(t, router, "/music/api/ai/player/state", body)
		if response.Code != http.StatusOK {
			t.Fatalf("state status=%d body=%s", response.Code, response.Body.String())
		}
	}

	readState := func(query string) aiPlayerSnapshot {
		request := httptest.NewRequest(http.MethodGet, "/music/api/ai/player/state"+query, nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("state read status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		var snapshot aiPlayerSnapshot
		if err := json.Unmarshal(recorder.Body.Bytes(), &snapshot); err != nil {
			t.Fatal(err)
		}
		return snapshot
	}

	all := readState("")
	if all.ConnectedClients != 2 || len(all.Players) != 2 || len(all.Clients) != 2 {
		t.Fatalf("unexpected snapshot: %+v", all)
	}
	// A 正在播放，默认优先选择活跃播放器，而不是后上报的暂停客户端 B。
	if all.SelectedClientID != "tab-a" || all.Player == nil || all.Player.Current == nil || all.Player.Current.ID != "song-a" {
		t.Fatalf("unexpected default player: %+v", all.Player)
	}

	onlyA := readState("?client_id=tab-a")
	if onlyA.Player == nil || onlyA.Player.Current == nil || onlyA.Player.Current.ID != "song-a" || onlyA.Player.Position != 12 {
		t.Fatalf("unexpected tab-a state: %+v", onlyA.Player)
	}

	// A 的定时状态上报不应把默认选择切到暂停的 B，且 B 的状态仍然保留。
	if response := postAIPlayerJSON(t, router, "/music/api/ai/player/state",
		`{"client_id":"tab-a","ready":true,"playing":true,"current":{"id":"song-a","source":"qq","name":"A"},"position":13,"duration":200,"volume":0.5}`); response.Code != http.StatusOK {
		t.Fatalf("state status=%d body=%s", response.Code, response.Body.String())
	}
	afterA := readState("")
	if afterA.SelectedClientID != "tab-a" || afterA.Player.Position != 13 {
		t.Fatalf("unexpected player after refresh: %+v", afterA.Player)
	}
	onlyB := readState("?client_id=tab-b")
	if onlyB.Player == nil || onlyB.Player.Current == nil || onlyB.Player.Current.ID != "song-b" || onlyB.Player.Position != 40 {
		t.Fatalf("tab-b state was lost: %+v", onlyB.Player)
	}
}

func TestAIPlayerAdoptsAnonymousConnectionOnStateReport(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})
	commands, anonID, unsubscribe := subscribeAIPlayerTest(t, hub, "")
	defer unsubscribe()
	if !strings.HasPrefix(anonID, "anon-") {
		t.Fatalf("expected anonymous client id, got %q", anonID)
	}

	if response := postAIPlayerJSON(t, router, "/music/api/ai/player/state",
		`{"client_id":"tab-real","ready":true,"playing":false}`); response.Code != http.StatusOK {
		t.Fatalf("state status=%d body=%s", response.Code, response.Body.String())
	}

	response := postAIPlayerJSON(t, router, "/music/api/ai/player/command",
		`{"client_id":"tab-real","action":"play"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	command := receiveAIPlayerCommand(t, commands)
	if command.ClientID != "tab-real" {
		t.Fatalf("anonymous connection was not adopted: %+v", command)
	}
}

func TestAIPlayerReconnectReplacesOldStream(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})
	oldCommands, oldID, unsubscribeOld := subscribeAIPlayerTest(t, hub, "tab-a")
	newCommands, newID, unsubscribeNew := subscribeAIPlayerTest(t, hub, "tab-a")
	defer unsubscribeOld()
	defer unsubscribeNew()
	if oldID != newID {
		t.Fatalf("reconnect changed client id: old=%q new=%q", oldID, newID)
	}

	select {
	case _, ok := <-oldCommands:
		if ok {
			t.Fatal("old stream received a command after replacement")
		}
	case <-time.After(time.Second):
		t.Fatal("old stream was not closed on reconnect")
	}

	response := postAIPlayerJSON(t, router, "/music/api/ai/player/command",
		`{"client_id":"tab-a","action":"pause"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	command := receiveAIPlayerCommand(t, newCommands)
	if command.ClientID != "tab-a" || command.Action != "pause" {
		t.Fatalf("unexpected command on replacement stream: %+v", command)
	}
}

func TestAIPlayerEnforcesConnectedClientLimit(t *testing.T) {
	hub := newAIPlayerHub()
	unsubscribers := make([]func(), 0, aiPlayerMaxClients)
	for i := 0; i < aiPlayerMaxClients; i++ {
		_, _, unsubscribe := subscribeAIPlayerTest(t, hub, fmt.Sprintf("tab-%d", i))
		unsubscribers = append(unsubscribers, unsubscribe)
	}
	defer func() {
		for _, unsubscribe := range unsubscribers {
			unsubscribe()
		}
	}()

	_, _, _, err := hub.subscribe("tab-overflow")
	if !errors.Is(err, errAIPlayerCapacity) {
		t.Fatalf("subscribe error = %v, want capacity error", err)
	}
}

func TestAIPlayerExpiresDetachedClients(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})
	_, clientID, unsubscribe := subscribeAIPlayerTest(t, hub, "tab-a")
	if response := postAIPlayerJSON(t, router, "/music/api/ai/player/state",
		`{"client_id":"tab-a","ready":true}`); response.Code != http.StatusOK {
		t.Fatalf("state status=%d body=%s", response.Code, response.Body.String())
	}
	unsubscribe()

	hub.mu.Lock()
	client := hub.clients[clientID]
	if client == nil {
		hub.mu.Unlock()
		t.Fatal("detached client was removed before expiry")
	}
	client.detachedAt = time.Now().Add(-aiPlayerDetachedTTL - time.Second)
	hub.mu.Unlock()

	snapshot := hub.snapshot("")
	if len(snapshot.Clients) != 0 || snapshot.ConnectedClients != 0 || snapshot.Player != nil {
		t.Fatalf("expired client remains visible: %+v", snapshot)
	}
}

func TestAIPlayerReturnsBusyWhenCommandBufferIsFull(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})
	_, clientID, unsubscribe := subscribeAIPlayerTest(t, hub, "tab-a")
	defer unsubscribe()

	for i := 0; i < aiPlayerCommandBuffer; i++ {
		_, delivered, err := hub.dispatch(aiPlayerCommand{Action: "pause"}, clientID)
		if err != nil || delivered != 1 {
			t.Fatalf("fill command %d: delivered=%d err=%v", i, delivered, err)
		}
	}

	response := postAIPlayerJSON(t, router, "/music/api/ai/player/command",
		`{"client_id":"tab-a","action":"pause"}`)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error != "player_busy" || response.Header().Get("Retry-After") != "1" {
		t.Fatalf("unexpected busy response: error=%q retry=%q", payload.Error, response.Header().Get("Retry-After"))
	}
}

func TestAIPlayerKeepsLastCommandPerClient(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})
	commandsA, _, unsubscribeA := subscribeAIPlayerTest(t, hub, "tab-a")
	commandsB, _, unsubscribeB := subscribeAIPlayerTest(t, hub, "tab-b")
	defer unsubscribeA()
	defer unsubscribeB()

	response := postAIPlayerJSON(t, router, "/music/api/ai/player/command",
		`{"client_id":"tab-b","action":"pause"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	command := receiveAIPlayerCommand(t, commandsB)
	if command.Action != "pause" || command.ClientID != "tab-b" {
		t.Fatalf("unexpected tab-b command: %+v", command)
	}
	expectNoAIPlayerCommand(t, commandsA)

	if snapshot := hub.snapshot("tab-a"); snapshot.LastCommand != nil {
		t.Fatalf("tab-a inherited tab-b last command: %+v", snapshot.LastCommand)
	}
	snapshot := hub.snapshot("tab-b")
	if snapshot.LastCommand == nil || snapshot.LastCommand.Action != "pause" ||
		snapshot.LastCommand.ClientID != "tab-b" {
		t.Fatalf("unexpected tab-b last command: %+v", snapshot.LastCommand)
	}
}

func TestAIPlayerValidatesClientIDsAndTargets(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})
	if _, _, _, err := hub.subscribe("bad client"); !errors.Is(err, errAIPlayerInvalidClientID) {
		t.Fatalf("subscribe error = %v, want invalid client id", err)
	}

	_, _, unsubscribe := subscribeAIPlayerTest(t, hub, "tab-a")
	defer unsubscribe()
	for _, body := range []string{
		`{"client_id":"tab-a","target_client_id":"tab-b","action":"pause"}`,
		`{"client_id":"bad client","action":"pause"}`,
	} {
		response := postAIPlayerJSON(t, router, "/music/api/ai/player/command", body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d response=%s", body, response.Code, response.Body.String())
		}
	}

	response := postAIPlayerJSON(t, router, "/music/api/ai/player/state",
		`{"client_id":"bad client","ready":true}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid state client id status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAIPlayerDefaultSelectionIsStableAndPrefersActivePlayers(t *testing.T) {
	router, hub := newAIPlayerTestRouter(aiSearchProviders{})
	_, _, unsubscribeA := subscribeAIPlayerTest(t, hub, "tab-a")
	_, _, unsubscribeB := subscribeAIPlayerTest(t, hub, "tab-b")
	defer unsubscribeA()
	defer unsubscribeB()

	postState := func(body string) {
		t.Helper()
		if response := postAIPlayerJSON(t, router, "/music/api/ai/player/state", body); response.Code != http.StatusOK {
			t.Fatalf("state status=%d body=%s", response.Code, response.Body.String())
		}
	}
	selected := func() string {
		t.Helper()
		return hub.snapshot("").SelectedClientID
	}

	postState(`{"client_id":"tab-a","ready":true,"playing":false,"page_visible":true}`)
	postState(`{"client_id":"tab-b","ready":true,"playing":false,"page_visible":true}`)
	if got := selected(); got != "tab-a" {
		t.Fatalf("paused visible clients should stay stable on oldest connection, got %q", got)
	}

	postState(`{"client_id":"tab-b","ready":true,"playing":true,"page_visible":true}`)
	if got := selected(); got != "tab-b" {
		t.Fatalf("playing client should be preferred, got %q", got)
	}

	postState(`{"client_id":"tab-a","ready":true,"playing":true,"page_visible":false}`)
	if got := selected(); got != "tab-b" {
		t.Fatalf("visible playing client should be preferred, got %q", got)
	}

	postState(`{"client_id":"tab-a","ready":true,"playing":true,"page_visible":true}`)
	if got := selected(); got != "tab-a" {
		t.Fatalf("equal active clients should use stable connection order, got %q", got)
	}
}
