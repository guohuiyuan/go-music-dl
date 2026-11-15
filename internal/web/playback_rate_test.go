package web

import (
	"strings"
	"testing"
)

func TestPlaybackRateControlsUseSharedState(t *testing.T) {
	appContent, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}
	videoContent, err := templateFS.ReadFile("templates/static/js/videogen.js")
	if err != nil {
		t.Fatalf("ReadFile(videogen.js): %v", err)
	}

	appJS := string(appContent)
	for _, want := range []string{
		"window.PLAYER_SPEEDS = PLAYER_SPEEDS;",
		"const PLAYER_SPEED_MARKS = Object.freeze([0.5, 1, 1.5, 2, 2.5, 3]);",
		"window.setPlayerPlaybackRate = setPlayerPlaybackRate;",
		"window.applyPlayerPlaybackRate = applyPlayerPlaybackRate;",
		"window.playerSpeedScaleMarkup = playerSpeedScaleMarkup;",
		"slider.style.setProperty(\"--speed-progress\"",
		`playerSpeedScaleMarkup("player-speed-scale")`,
		`ap.audio.addEventListener("emptied", () => {`,
		"applyPlayerPlaybackRate(playerSpeed);",
	} {
		if !strings.Contains(appJS, want) {
			t.Fatalf("app.js missing playback rate token %q", want)
		}
	}

	videoJS := string(videoContent)
	for _, want := range []string{
		"window.PLAYER_SPEEDS",
		"window.setPlayerPlaybackRate(normalized);",
		"this.getCurrentPlaybackRate()",
		"window.playerSpeedScaleMarkup",
		"[0.5, 1, 1.5, 2, 2.5, 3]",
		"scaleMarkup",
		"2.25",
		"2.75",
		"3",
	} {
		if !strings.Contains(videoJS, want) {
			t.Fatalf("videogen.js missing shared playback rate token %q", want)
		}
	}
	if strings.Contains(videoJS, "[0.5, 1, 1.25, 1.5, 2]") {
		t.Fatal("videogen.js still uses the old limited playback rate list")
	}
	if strings.Contains(appJS, `<span>0.5x</span><span>1x</span><span>3x</span>`) ||
		strings.Contains(videoJS, `<span>0.5x</span><span>1x</span><span>3x</span>`) {
		t.Fatal("playback speed scale still uses evenly spaced hard-coded labels")
	}

	styleContent, err := templateFS.ReadFile("templates/static/css/style.css")
	if err != nil {
		t.Fatalf("ReadFile(style.css): %v", err)
	}
	styleCSS := string(styleContent)
	for _, want := range []string{
		"width: 260px;",
		"--speed-progress: 20%;",
		"left: var(--speed-position, 0%);",
		"height: 2px;",
		"margin-top: -5px;",
	} {
		if !strings.Contains(styleCSS, want) {
			t.Fatalf("style.css missing playback speed layout token %q", want)
		}
	}
}

func TestTaskbarPlaybackControlSupportsNativeAndroidActions(t *testing.T) {
	appContent, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}
	appJS := string(appContent)
	for _, want := range []string{
		`window.musicDlTaskbarControl = function (action, value)`,
		`case "play":`,
		`case "pause":`,
		`case "stop":`,
		`case "seek":`,
		`const positionSeconds = (Number(value) || 0) / 1000;`,
	} {
		if !strings.Contains(appJS, want) {
			t.Fatalf("app.js missing native playback control token %q", want)
		}
	}
}
