package web

import (
	"strings"
	"testing"
)

func TestVideogenUsesRomajiBeforeTranslationOrder(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/js/videogen.js")
	if err != nil {
		t.Fatalf("ReadFile(videogen.js): %v", err)
	}

	js := string(content)
	if strings.Contains(js, "const [orig, trans, roma] = group.lines") {
		t.Fatal("videogen.js still assumes the old original/translation/romaji order")
	}
	for _, want := range []string{
		"function splitLyricGroupLinesWorker(lines)",
		"function splitLyricGroupLines(lines)",
		"const { orig, roma, trans } = splitLyricGroupLinesWorker(group.lines)",
		"const { orig, roma, trans } = splitLyricGroupLines(group.lines)",
		"renderKaraokeLineHTML(roma, 'vg-line-roma'",
		"renderKaraokeLineHTML(trans, 'vg-line-trans'",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("videogen.js missing %q", want)
		}
	}
}

func TestVideogenRenderUploadsBinaryFrameBatches(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/js/videogen.js")
	if err != nil {
		t.Fatalf("ReadFile(videogen.js): %v", err)
	}

	js := string(content)
	for _, want := range []string{
		"targetCanvas.toBlob",
		"const form = new FormData()",
		`form.append("frames", blob,`,
		"framesBuffer.push(await canvasToJpegBlob(canvas, 0.92))",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("videogen.js missing %q", want)
		}
	}
	if strings.Contains(js, `framesBuffer.push(canvas.toDataURL("image/jpeg", 0.92))`) {
		t.Fatal("videogen.js still uploads render frames as base64 data URLs")
	}
}

func TestVideogenKaraokeColorsMatchPlaybackLyrics(t *testing.T) {
	jsContent, err := templateFS.ReadFile("templates/static/js/videogen.js")
	if err != nil {
		t.Fatalf("ReadFile(videogen.js): %v", err)
	}
	cssContent, err := templateFS.ReadFile("templates/static/css/videogen.css")
	if err != nil {
		t.Fatalf("ReadFile(videogen.css): %v", err)
	}

	js := string(jsContent)
	for _, want := range []string{
		`const karaokeTextColor = "#ffffff"`,
		`const karaokeAccentColor = "#12bd85"`,
		`karaokeStrokeText(lineText, x, y, layout.lineHeight, baseColor, karaokeAccentColor, alpha)`,
		`const blockAlpha = isCurrent ? 1 : 0.72`,
		`karaokeTextColor`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("videogen.js missing karaoke color token %q", want)
		}
	}

	css := string(cssContent)
	for _, want := range []string{
		".vg-line-orig",
		".vg-line-roma",
		".vg-line-trans",
		"--karaoke-base-color: #ffffff",
		"color: #ffffff",
		"-1px -1px 0 #10b981",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("videogen.css missing playback-matched karaoke color %q", want)
		}
	}
}
