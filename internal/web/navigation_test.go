package web

import (
	"encoding/json"
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/music-lib/model"
)

func newTestTemplate(t *testing.T) *template.Template {
	t.Helper()

	return template.Must(template.New("").Funcs(template.FuncMap{
		"artistTokens":       splitArtistTokens,
		"albumID":            songAlbumID,
		"playlistDetailURL":  playlistDetailURL,
		"playlistExtraValue": playlistExtraValue,
		"tojson": func(v interface{}) string {
			if v == nil {
				return ""
			}
			b, err := json.Marshal(v)
			if err != nil {
				return ""
			}
			return string(b)
		},
	}).ParseFS(templateFS,
		"templates/pages/*.html",
		"templates/partials/*.html",
	))
}

func TestRenderIndexPlaylistCardsUseAjaxNavigation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.SetHTMLTemplate(newTestTemplate(t))
	router.GET(RoutePrefix, func(c *gin.Context) {
		renderIndex(c, nil, []model.Playlist{
			{
				ID:         "123",
				Name:       "Top Hits",
				TrackCount: 18,
				Creator:    "Tester",
				Source:     "qq",
				Cover:      "https://example.com/cover.jpg",
			},
		}, "", []string{"qq"}, "", collectionContentPlaylist, "", "", "", false, "", nil)
	})

	req := httptest.NewRequest("GET", RoutePrefix, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.Contains(body, `onclick="location.href=`) {
		t.Fatalf("rendered html still uses location.href navigation: %s", body)
	}
	if !strings.Contains(body, `onclick="navigateTo('`) {
		t.Fatalf("rendered html missing navigateTo playlist navigation: %s", body)
	}
}

func TestAppJSIncludesAjaxNavigationEntryPoints(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}

	js := string(content)
	if !strings.Contains(js, "async function navigateTo(url, options = {})") {
		t.Fatal("app.js missing navigateTo function")
	}
	if !strings.Contains(js, "function bindPageNavigationEvents()") {
		t.Fatal("app.js missing bindPageNavigationEvents function")
	}
	if !strings.Contains(js, "function handlePaginationShortcut(event)") {
		t.Fatal("app.js missing pagination shortcut handler")
	}
	if !strings.Contains(js, "document.addEventListener('keydown', handlePaginationShortcut);") {
		t.Fatal("app.js missing pagination shortcut binding")
	}
	if !strings.Contains(js, "initializePageContent(document);") {
		t.Fatal("app.js missing initializePageContent bootstrap")
	}
	if !strings.Contains(js, "function refreshDownloadLinks(root = document)") {
		t.Fatal("app.js missing scoped refreshDownloadLinks helper")
	}
	if !strings.Contains(js, "refreshDownloadLinks(root);") {
		t.Fatal("app.js missing download link refresh during page initialization")
	}
	if !strings.Contains(js, "function maybeAutoCheckUpdate()") {
		t.Fatal("app.js missing auto update check")
	}
	if !strings.Contains(js, "function checkAppUpdate(options = {})") {
		t.Fatal("app.js missing GitHub update check")
	}
	if !strings.Contains(js, "async function openAboutAppModal()") {
		t.Fatal("app.js missing openAboutAppModal entry point")
	}
	if !strings.Contains(js, "async function openLatestUpdatePage(target = 'download')") {
		t.Fatal("app.js missing openLatestUpdatePage helper")
	}
	if !strings.Contains(js, "function openClientExternalURL(url, popup = null)") {
		t.Fatal("app.js missing client-side external opener")
	}
	if strings.Contains(js, "/app_update/open?url=") {
		t.Fatal("app.js should not ask the server to open release pages")
	}
	if strings.Contains(js, "/app_update/install_link") {
		t.Fatal("app.js should no longer reference removed install_link API")
	}
	for _, removed := range []string{
		"function syncGithubProxyInputs",
		"function currentGithubProxyURL",
		"function updateProxyCustomState",
		"async function testGithubProxy",
		"function syncUpdateProxyInputs",
		"function currentUpdateProxyURL",
		"function updateUpdateProxyCustomState",
		"async function testUpdateGithubProxy",
		"function applyUpdateProxyVisibility",
		"async function refreshUpdateModalCheck",
	} {
		if strings.Contains(js, removed) {
			t.Fatalf("app.js should not redeclare deprecated helper %q", removed)
		}
	}
	if !strings.Contains(js, "function initializeLocalMusicPage(root = document)") {
		t.Fatal("app.js missing async local music page initializer")
	}
	if !strings.Contains(js, "offset: String(offset)") {
		t.Fatal("app.js missing paged local music API request")
	}
}

func TestAppJSPlaybackURLIgnoresEmbedDownloadSetting(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}

	js := string(content)
	if !strings.Contains(js, "function buildStreamURL(id, source, name, artist, album, cover, extra)") {
		t.Fatal("app.js missing buildStreamURL")
	}
	if !strings.Contains(js, "stream: true") {
		t.Fatal("buildStreamURL should force stream=1")
	}
	if strings.Contains(js, "function buildStreamURL(id, source, name, artist, album, cover, extra) {\n    return buildDownloadRequestURL(id, source, name, artist, album, cover, extra, {\n        embed: webSettings.embedDownload") {
		t.Fatal("buildStreamURL must not follow embedDownload; playback should always use plain streaming")
	}
	if !strings.Contains(js, "preload: 'metadata'") {
		t.Fatal("APlayer should preload metadata instead of full audio")
	}
}

func TestDownloadURLsCarryAlbumForMetadataEmbedding(t *testing.T) {
	jsContent, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}
	js := string(jsContent)
	for _, want := range []string{
		"function buildDownloadRequestURL(id, source, name, artist, album, cover, extra, options = {})",
		"function buildBrowserDownloadURL(id, source, name, artist, album, cover, extra)",
		"params.set('album', albumValue);",
		"buildDownloadURL(ds.id, ds.source, ds.name, ds.artist, ds.album || ''",
		"buildDownloadURL(song.id, song.source, song.name, song.artist, song.album || ''",
		"buildBrowserDownloadURL(ds.id, ds.source, ds.name, ds.artist, ds.album || ''",
		"buildBrowserDownloadURL(song.id, song.source, song.name, song.artist, song.album || ''",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js missing album download URL token %q", want)
		}
	}

	htmlContent, err := templateFS.ReadFile("templates/partials/song_list.html")
	if err != nil {
		t.Fatalf("ReadFile(song_list.html): %v", err)
	}
	if !strings.Contains(string(htmlContent), `&album={{urlquery .Album}}`) {
		t.Fatal("song_list.html download link should include album query parameter")
	}
	if !strings.Contains(string(htmlContent), `class="btn-circle btn-dl btn-browser-download"`) {
		t.Fatal("song_list.html should render a dedicated browser download button")
	}
	if !strings.Contains(string(htmlContent), `title="浏览器下载"`) {
		t.Fatal("browser download button should be labeled separately from save-to-local download")
	}
}

func TestSongListIncludesSortControls(t *testing.T) {
	jsContent, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}
	js := string(jsContent)
	for _, want := range []string{
		"function setSongSortMode(mode)",
		"function toggleSongSortDirection()",
		"function applySongSort(root = document)",
		"card.dataset.sortSize = String(parseSizeToBytes(data.size));",
		"card.dataset.sortBitrate = String(parseBitrateToKbps(data.bitrate));",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js missing song sort token %q", want)
		}
	}

	htmlContent, err := templateFS.ReadFile("templates/partials/song_list.html")
	if err != nil {
		t.Fatalf("ReadFile(song_list.html): %v", err)
	}
	html := string(htmlContent)
	for _, want := range []string{
		`id="song-sort-select"`,
		`value="quality">按音质</option>`,
		`value="size">按大小</option>`,
		`value="duration">按时长</option>`,
		`data-sort-size=`,
		`data-sort-bitrate=`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("song_list.html missing sort token %q", want)
		}
	}
}

func TestSettingsModalIncludesDownloadDirPresets(t *testing.T) {
	content, err := templateFS.ReadFile("templates/partials/modals.html")
	if err != nil {
		t.Fatalf("ReadFile(modals.html): %v", err)
	}

	html := string(content)
	for _, want := range []string{
		`id="setting-download-dir-preset"`,
		`id="setting-download-filename-template"`,
		`id="setting-floating-lyrics"`,
		`id="setting-auto-switch-invalid-sources"`,
		`onclick="openAboutAppModal()"`,
		`关于 go-music-dl`,
		`class="cookie-item setting-item setting-link-row"`,
		`setting-link-icon`,
		`setting-link-title`,
		`setting-link-chevron`,
		`{album}`,
		`{source}`,
		`{ext}`,
		`PC 默认：data/downloads`,
		`PC 示例：D:/Music/Downloads`,
		`Android 默认：/sdcard/Music`,
		`Android 兼容：/storage/emulated/0/Music`,
		`自定义目录...`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("settings modal missing token %q", want)
		}
	}
	for _, unwanted := range []string{
		`id="setting-update-repo-url"`,
		`id="setting-github-proxy-enabled"`,
		`id="setting-github-proxy-disabled"`,
		`name="setting-github-proxy-url"`,
		`onclick="testGithubProxy()"`,
		`onclick="checkAppUpdate({ showNoUpdate: true })"`,
		`id="setting-auto-check-update"`,
		`id="updateRepoUrlInput"`,
		`id="updateGithubProxyEnabled"`,
		`name="update-github-proxy-url"`,
		`id="updateGithubProxyCustom"`,
		`onchange="applyUpdateProxyVisibility()"`,
		`onclick="testUpdateGithubProxy()"`,
		`btn-pill btn-pill-primary" onclick="openAboutAppModal()"`,
		`setting-action-row`,
	} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("modals.html should not contain removed update control %q", unwanted)
		}
	}
}

func TestDefaultPageSizePrompts(t *testing.T) {
	jsContent, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}
	for _, want := range []string{
		`const DEFAULT_WEB_PAGE_SIZE = 30;`,
		`const DEFAULT_CLI_PAGE_SIZE = 20;`,
	} {
		if !strings.Contains(string(jsContent), want) {
			t.Fatalf("app.js missing default page size token %q", want)
		}
	}

	modalContent, err := templateFS.ReadFile("templates/partials/modals.html")
	if err != nil {
		t.Fatalf("ReadFile(modals.html): %v", err)
	}
	html := string(modalContent)
	for _, want := range []string{
		`id="setting-web-page-size" min="1" max="200" step="1" placeholder="默认 30"`,
		`用于网页分页显示，默认 30。`,
		`id="setting-cli-page-size" min="1" max="200" step="1" placeholder="默认 20"`,
		`用于 TUI 分页显示，默认 20。`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("modals.html missing page size prompt %q", want)
		}
	}
}

func TestAppUpdateModalIsAboutOnly(t *testing.T) {
	content, err := templateFS.ReadFile("templates/partials/modals.html")
	if err != nil {
		t.Fatalf("ReadFile(modals.html): %v", err)
	}
	html := string(content)
	for _, want := range []string{
		`id="appUpdateModal"`,
		`id="appUpdateTitle"`,
		`id="appUpdateSummary"`,
		`id="updateCheckStatus"`,
		`onclick="closeUpdateModal()"`,
		`onclick="openLatestUpdatePage('release')"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("app update modal missing %q", want)
		}
	}
	for _, unwanted := range []string{
		`id="updateRepoUrlInput"`,
		`id="updateGithubProxyEnabled"`,
		`id="updateGithubProxyDisabled"`,
		`id="updateGithubProxyCustom"`,
		`id="updateGithubProxyCustomRadio"`,
		`name="update-github-proxy-url"`,
		`name="update-github-proxy-mode"`,
		`name="update-install-mode"`,
		`id="updatePackageFile"`,
		`id="appUpdateNotes"`,
		`id="updateProxyTestStatus"`,
		`onclick="testUpdateGithubProxy()"`,
		`onclick="refreshUpdateModalCheck()"`,
		`onclick="openLatestUpdatePage('download')"`,
		`onchange="applyUpdateProxyVisibility()"`,
		`installUpdateFromModal`,
		`appUpdateInstallBtn`,
		`重新检查`,
		`前往 GitHub 下载`,
		`从文件安装`,
		`https://edgeone.gh-proxy.com`,
		`https://gh.llkk.cc`,
	} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("app update modal should be minimal but contains %q", unwanted)
		}
	}
}

func TestSearchSourceSelectorSupportsDesktopAndMobileCollapse(t *testing.T) {
	htmlContent, err := templateFS.ReadFile("templates/partials/search_box.html")
	if err != nil {
		t.Fatalf("ReadFile(search_box.html): %v", err)
	}
	html := string(htmlContent)
	for _, want := range []string{
		`id="source-selector"`,
		`class="source-collapse-btn"`,
		`onclick="toggleSourceSelector()"`,
		`aria-controls="source-grid"`,
		`fa-chevron-up source-collapse-icon`,
		`id="source-grid"`,
		`class="source-actions"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("search_box.html missing collapse token %q", want)
		}
	}

	cssContent, err := templateFS.ReadFile("templates/static/css/style.css")
	if err != nil {
		t.Fatalf("ReadFile(style.css): %v", err)
	}
	css := string(cssContent)
	for _, want := range []string{
		`.source-selector.is-collapsed .source-grid { display: none; }`,
		`.source-selector.is-collapsed .source-collapse-icon { transform: rotate(180deg); }`,
		`.source-collapse-btn { background: none; border: none; padding: 0; display: inline-flex; align-items: center; gap: 6px; cursor: pointer;`,
		`display: inline-block;`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("style.css missing collapse rule %q", want)
		}
	}

	jsContent, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}
	js := string(jsContent)
	for _, want := range []string{
		`function toggleSourceSelector(force)`,
		`function initSourceSelectorCollapse(root = document)`,
		`function isMobileSourceSelectorViewport()`,
		`function applySourceSelectorCollapsed(selector, collapsed)`,
		`'(max-width: 720px)'`,
		`sessionStorage.setItem('sourceSelectorCollapsed'`,
		`initSourceSelectorCollapse(root);`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js missing collapse helper %q", want)
		}
	}
	for _, unwanted := range []string{
		`if (!isMobileSourceSelectorViewport()) return;`,
		`if (!isMobile) {
        collapsed = false;
    }`,
	} {
		if strings.Contains(js, unwanted) {
			t.Fatalf("app.js still limits source selector collapse to mobile: %q", unwanted)
		}
	}
}

func TestAppToastFitsMobileViewport(t *testing.T) {
	cssContent, err := templateFS.ReadFile("templates/static/css/style.css")
	if err != nil {
		t.Fatalf("ReadFile(style.css): %v", err)
	}

	css := string(cssContent)
	for _, want := range []string{
		`.app-toast-container {`,
		`right: 18px;`,
		`width: min(360px, calc(100vw - 36px));`,
		`box-sizing: border-box;`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("style.css missing mobile-safe toast rule %q", want)
		}
	}
}

func TestPaginationTemplatesExposeShortcutMetadata(t *testing.T) {
	paths := []string{
		"templates/partials/song_list.html",
		"templates/partials/playlist_grid.html",
	}

	for _, path := range paths {
		content, err := templateFS.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}

		html := string(content)
		if !strings.Contains(html, `data-current-page="{{ .Page }}"`) {
			t.Fatalf("%s missing current page metadata", path)
		}
		if !strings.Contains(html, `data-total-pages="{{ .TotalPages }}"`) {
			t.Fatalf("%s missing total pages metadata", path)
		}
		if !strings.Contains(html, `data-shortcut-hint="PgUp / PgDn"`) {
			t.Fatalf("%s missing pagination shortcut hint", path)
		}
	}
}
