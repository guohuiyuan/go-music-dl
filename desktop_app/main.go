package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/op"

	"github.com/gioui-plugins/gio-plugins/plugin/gioplugins"
	"github.com/gioui-plugins/gio-plugins/webviewer/giowebview"
	"github.com/guohuiyuan/go-music-dl/internal/web"
)

type webTag struct{}

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
	ok := false
	for {
		e := gioplugins.Hijack(w)

		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(ops, e)

			size := gtx.Constraints.Max
			stack := giowebview.WebViewOp{Tag: tag}.Push(gtx.Ops)

			giowebview.RectOp{Size: f32.Point{X: float32(size.X), Y: float32(size.Y)}}.Add(gtx.Ops)
			stack.Pop(gtx.Ops)
			e.Frame(gtx.Ops)
			if !ok {
				gioplugins.Execute(gtx, giowebview.NavigateCmd{
					URL:  "http://localhost:37777/music/",
					View: tag,
				})
				ok = true
			}
		}
	}
}
