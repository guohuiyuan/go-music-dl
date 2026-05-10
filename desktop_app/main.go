package main

import (
	"log"
	"net/url"
	"os"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/gioui-plugins/gio-plugins/hyperlink/giohyperlink"
	"github.com/gioui-plugins/gio-plugins/plugin/gioplugins"
	"github.com/gioui-plugins/gio-plugins/webviewer/giowebview"
	"github.com/guohuiyuan/go-music-dl/internal/web"

	_ "gioui.org/app/permission/storage"
)

type webTag struct{}

type desktopApp struct {
	window *app.Window
	ops    op.Ops
	tag    webTag

	// 这些状态用于在多个 frame 之间协调 WebView 初始化流程，
	// 因为 Gio 插件命令是在 frame 处理中下发的。
	bridgeInstalled       bool
	callbackRegistered    bool
	pendingInitialNav     bool
	pendingHistoryBack    bool
	pendingExternalOpenTo *url.URL
}

const (
	initialURL         = "http://localhost:37777/music/"
	downloadCallback   = "musicDlOpenDownload"
	preferredBrowserPK = ""
)

// 注入的桥接脚本把桌面端行为放在壳层处理：
// 返回操作继续走原生外壳，普通链接下载模式下的下载则回传给 Go 处理。
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

  document.addEventListener("click", function (event) {
    if (event.defaultPrevented) {
      return;
    }
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
      return;
    }

    var link = event.target && event.target.closest ? event.target.closest(".btn-download") : null;
    if (!link) {
      return;
    }

    var localToggle = document.getElementById("setting-download-to-local");
    if (localToggle && localToggle.checked) {
      return;
    }

    var embedToggle = document.getElementById("setting-embed-download");
    if (embedToggle && embedToggle.checked) {
      return;
    }

    var href = String(link.href || "").trim();
    if (!href) {
      return;
    }

    event.preventDefault();
    event.stopPropagation();

    if (globalThis.callback && typeof globalThis.callback.musicDlOpenDownload === "function") {
      globalThis.callback.musicDlOpenDownload(href);
    } else {
      window.location.href = href;
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
		window := new(app.Window)
		window.Option(app.Title("music-dl"))
		if err := newDesktopApp(window).run(); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func newDesktopApp(window *app.Window) *desktopApp {
	return &desktopApp{
		window: window,
	}
}

func (a *desktopApp) run() error {
	for {
		switch evt := gioplugins.Hijack(a.window).(type) {
		case app.DestroyEvent:
			return evt.Err
		case app.FrameEvent:
			a.handleFrame(evt)
		}
	}
}

func (a *desktopApp) handleFrame(evt app.FrameEvent) {
	gtx := app.NewContext(&a.ops, evt)

	a.pendingHistoryBack = a.pendingHistoryBack || consumeBackShortcuts(gtx)
	a.consumeWebViewEvents(gtx)
	a.layoutWebView(gtx)
	evt.Frame(gtx.Ops)

	a.ensureBridge(gtx)
	a.handlePendingNavigation(gtx)
	a.handlePendingHistoryBack(gtx)
	a.handlePendingExternalOpen(gtx)
}

func (a *desktopApp) layoutWebView(gtx layout.Context) {
	size := gtx.Constraints.Max
	stack := giowebview.WebViewOp{Tag: &a.tag}.Push(gtx.Ops)
	giowebview.RectOp{Size: f32.Point{X: float32(size.X), Y: float32(size.Y)}}.Add(gtx.Ops)
	stack.Pop(gtx.Ops)
}

func (a *desktopApp) ensureBridge(gtx layout.Context) {
	if !a.bridgeInstalled {
		gioplugins.Execute(gtx, giowebview.InstallJavascriptCmd{
			View:   &a.tag,
			Script: bridgeScript,
		})
		a.bridgeInstalled = true
	}

	if a.bridgeInstalled && !a.callbackRegistered {
		// 只注册一次 JS 回调，这样下载链接会交回桌面壳层处理，
		// 而不是在内嵌 WebView 里继续跳转。
		gioplugins.Execute(gtx, giowebview.MessageReceiverCmd{
			View: &a.tag,
			Tag:  &a.tag,
			Name: downloadCallback,
		})
		a.callbackRegistered = true
		a.pendingInitialNav = true
		a.window.Invalidate()
	}
}

func (a *desktopApp) handlePendingNavigation(gtx layout.Context) {
	if !a.pendingInitialNav {
		return
	}

	// 首次跳转延后到桥接回调准备完成之后，避免页面过早加载。
	gioplugins.Execute(gtx, giowebview.NavigateCmd{
		URL:  initialURL,
		View: &a.tag,
	})
	a.pendingInitialNav = false
}

func (a *desktopApp) handlePendingHistoryBack(gtx layout.Context) {
	if !a.pendingHistoryBack || !a.bridgeInstalled || a.pendingInitialNav {
		return
	}

	gioplugins.Execute(gtx, giowebview.ExecuteJavascriptCmd{
		View:   &a.tag,
		Script: historyBackScript,
	})
	a.pendingHistoryBack = false
}

func (a *desktopApp) handlePendingExternalOpen(gtx layout.Context) {
	if a.pendingExternalOpenTo == nil {
		return
	}

	// 通过系统浏览器打开链接，可以直接复用服务端现有的 /download 行为，
	// 不需要继续扩展 WebView 插件本身。
	gioplugins.Execute(gtx, giohyperlink.OpenCmd{
		Tag:              &a.tag,
		URI:              a.pendingExternalOpenTo,
		PreferredPackage: preferredBrowserPK,
	})
	log.Printf("opened download in external browser: %s", a.pendingExternalOpenTo.String())
	a.pendingExternalOpenTo = nil
}

func (a *desktopApp) consumeWebViewEvents(gtx layout.Context) {
	for {
		evt, ok := gioplugins.Event(gtx, giowebview.Filter{Target: &a.tag})
		if !ok {
			return
		}

		switch evt := evt.(type) {
		case giowebview.MessageEvent:
			a.handleWebViewMessage(evt.Message)
		}
	}
}

func (a *desktopApp) handleWebViewMessage(raw string) {
	u, err := url.Parse(raw)
	if err != nil {
		log.Printf("invalid download url from webview: %q (%v)", raw, err)
		return
	}

	a.pendingExternalOpenTo = u
	log.Printf("received download url from webview: %s", u.String())
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
