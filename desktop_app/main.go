package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/gioui-plugins/gio-plugins/plugin/gioplugins"
	"github.com/gioui-plugins/gio-plugins/webviewer/giowebview"
	"github.com/guohuiyuan/go-music-dl/internal/web"
)

type webTag struct{}

const (
	initialURL = "http://localhost:37777/music/"
)

const bridgeScript = `(function () {
  if (window.__musicDlDesktopBridgeInstalled) {
    return;
  }
  window.__musicDlDesktopBridgeInstalled = true;

  document.addEventListener("keydown", function (event) {
    if (event.defaultPrevented || event.isComposing) {
      return;
    }
    if (event.key === "BrowserBack") {
      event.preventDefault();
      window.history.back();
      return;
    }
    if (event.altKey && !event.ctrlKey && !event.metaKey && !event.shiftKey && event.key === "ArrowLeft") {
      event.preventDefault();
      window.history.back();
    }
  }, true);
})();`

const historyBackScript = `if (window.history.length > 1) { window.history.back(); }`

func main() {

	path, err := app.DataDir()
	if err != nil {
		log.Fatal(err)
	}
	os.Setenv("MUSIC_DL_CONFIG_DB", path+"/settings.db")
	os.Setenv("MUSIC_DL_COOKIE_FILE", path+"/cookies.json")

	go web.Start("37777", false)

	go func() {
		w := new(app.Window)
		w.Option(app.Title("music-dl"))
		if err := run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(w *app.Window) error {
	ops := new(op.Ops)
	tag := new(webTag)
	bridgeInstalled := false
	pendingInitialNavigate := false
	pendingHistoryBack := false
	for {
		e := gioplugins.Hijack(w)

		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(ops, e)
			pendingHistoryBack = consumeBackShortcuts(gtx) || pendingHistoryBack

			size := gtx.Constraints.Max
			stack := giowebview.WebViewOp{Tag: tag}.Push(gtx.Ops)

			giowebview.RectOp{Size: f32.Point{X: float32(size.X), Y: float32(size.Y)}}.Add(gtx.Ops)
			stack.Pop(gtx.Ops)
			e.Frame(gtx.Ops)

			if !bridgeInstalled {
				gioplugins.Execute(gtx, giowebview.InstallJavascriptCmd{
					View:   tag,
					Script: bridgeScript,
				})
				bridgeInstalled = true
				pendingInitialNavigate = true
				w.Invalidate()
			} else if pendingInitialNavigate {
				gioplugins.Execute(gtx, giowebview.NavigateCmd{
					URL:  initialURL,
					View: tag,
				})
				pendingInitialNavigate = false
			}

			if pendingHistoryBack && bridgeInstalled && !pendingInitialNavigate {
				gioplugins.Execute(gtx, giowebview.ExecuteJavascriptCmd{
					View:   tag,
					Script: historyBackScript,
				})
				pendingHistoryBack = false
			}
		}
	}
}

func consumeBackShortcuts(gtx layout.Context) bool {
	handled := false
	for {
		evt, ok := gtx.Event(
			key.Filter{Name: key.NameBack},
			key.Filter{Name: key.NameLeftArrow, Required: key.ModAlt},
		)
		if !ok {
			return handled
		}

		ke, ok := evt.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		handled = true
	}
}
