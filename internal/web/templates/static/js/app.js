// templates/app.js

const API_ROOT = window.API_ROOT;
const WEB_SETTINGS_KEY = "musicdl:web_settings";
const INSPECT_REQUEST_DELAY_MS = 100;
const AUTO_SWITCH_INVALID_DELAY_MS = 500;
const DEFAULT_WEB_PAGE_SIZE = 30;
const DEFAULT_CLI_PAGE_SIZE = 20;
const LOCAL_MUSIC_SOURCE = "local";
const LEGACY_LOCAL_MUSIC_SOURCE = "local-file";
const DOWNLOAD_DIR_CUSTOM_VALUE = "__custom__";
const DOWNLOAD_DIR_PRESET_VALUES = [
  "data/downloads",
  "downloads",
  "D:/Music/Downloads",
  "/sdcard/Music",
  "/storage/emulated/0/Music",
  "/sdcard/Download",
];
const DOWNLOAD_DIR_PRESETS = new Set(DOWNLOAD_DIR_PRESET_VALUES);
const DEFAULT_UPDATE_REPO_URL = "https://github.com/guohuiyuan/go-music-dl";
const DEFAULT_GITHUB_PROXY_URL = "https://edgeone.gh-proxy.com";
const OPEN_CONFIG_QUERY = "open_config";
const GITHUB_PROXY_PRESETS = [
  "https://edgeone.gh-proxy.com",
  "https://hk.gh-proxy.com/",
  "https://gh-proxy.com/",
  "https://gh.llkk.cc",
];

function isLocalMusicSourceValue(source) {
  const value = String(source || "").trim();
  return value === LOCAL_MUSIC_SOURCE || value === LEGACY_LOCAL_MUSIC_SOURCE;
}

let webSettings = {
  embedDownload: true,
  downloadToLocal: true,
  downloadDir: "data/downloads",
  downloadFilenameTemplate: "{name} - {artist}",
  disableFloatingLyrics: false,
  webPageSize: DEFAULT_WEB_PAGE_SIZE,
  cliPageSize: DEFAULT_CLI_PAGE_SIZE,
  autoCheckUpdate: true,
  autoSwitchInvalidSources: true,
  autoCacheOnPlay: true,
  updateRepoUrl: DEFAULT_UPDATE_REPO_URL,
  githubProxyEnabled: false,
  githubProxyUrl: DEFAULT_GITHUB_PROXY_URL,
  vgChangeCover: false,
  vgChangeAudio: false,
  vgChangeLyric: false,
  vgExportVideo: false,
};

function normalizeWebSettings(raw) {
  const next = {
    embedDownload: true,
    downloadToLocal: true,
    downloadDir: "data/downloads",
    downloadFilenameTemplate: "{name} - {artist}",
    disableFloatingLyrics: false,
    webPageSize: DEFAULT_WEB_PAGE_SIZE,
    cliPageSize: DEFAULT_CLI_PAGE_SIZE,
    autoCheckUpdate: true,
    autoSwitchInvalidSources: true,
    autoCacheOnPlay: true,
    updateRepoUrl: DEFAULT_UPDATE_REPO_URL,
    githubProxyEnabled: false,
    githubProxyUrl: DEFAULT_GITHUB_PROXY_URL,
    vgChangeCover: false,
    vgChangeAudio: false,
    vgChangeLyric: false,
    vgExportVideo: false,
  };

  if (!raw || typeof raw !== "object") {
    return next;
  }

  if (typeof raw.embedDownload === "boolean") {
    next.embedDownload = raw.embedDownload;
  }
  if (typeof raw.downloadDir === "string" && raw.downloadDir.trim() !== "") {
    next.downloadDir = raw.downloadDir.trim();
  }
  if (
    typeof raw.downloadFilenameTemplate === "string" &&
    raw.downloadFilenameTemplate.trim() !== ""
  ) {
    next.downloadFilenameTemplate = raw.downloadFilenameTemplate.trim();
  }
  if (typeof raw.disableFloatingLyrics === "boolean") {
    next.disableFloatingLyrics = raw.disableFloatingLyrics;
  }
  if (Number.isInteger(raw.webPageSize) && raw.webPageSize > 0) {
    next.webPageSize = Math.min(raw.webPageSize, 500);
  }
  if (Number.isInteger(raw.cliPageSize) && raw.cliPageSize > 0) {
    next.cliPageSize = Math.min(raw.cliPageSize, 200);
  }
  if (typeof raw.autoCheckUpdate === "boolean") {
    next.autoCheckUpdate = raw.autoCheckUpdate;
  }
  if (typeof raw.autoSwitchInvalidSources === "boolean") {
    next.autoSwitchInvalidSources = raw.autoSwitchInvalidSources;
  }
  if (typeof raw.autoCacheOnPlay === "boolean") {
    next.autoCacheOnPlay = raw.autoCacheOnPlay;
  }
  if (
    typeof raw.updateRepoUrl === "string" &&
    raw.updateRepoUrl.trim() !== ""
  ) {
    next.updateRepoUrl = raw.updateRepoUrl.trim();
  }
  if (typeof raw.githubProxyEnabled === "boolean") {
    next.githubProxyEnabled = raw.githubProxyEnabled;
  }
  if (
    typeof raw.githubProxyUrl === "string" &&
    raw.githubProxyUrl.trim() !== ""
  ) {
    next.githubProxyUrl = raw.githubProxyUrl.trim();
  }
  if (typeof raw.vgChangeCover === "boolean") {
    next.vgChangeCover = raw.vgChangeCover;
  }
  if (typeof raw.vgChangeAudio === "boolean") {
    next.vgChangeAudio = raw.vgChangeAudio;
  }
  if (typeof raw.vgChangeLyric === "boolean") {
    next.vgChangeLyric = raw.vgChangeLyric;
  }
  if (typeof raw.vgExportVideo === "boolean") {
    next.vgExportVideo = raw.vgExportVideo;
  }
  return next;
}

function loadWebSettingsFromCache() {
  try {
    const raw = localStorage.getItem(WEB_SETTINGS_KEY);
    if (!raw) return webSettings;
    webSettings = normalizeWebSettings(JSON.parse(raw));
  } catch (_) {}
  return webSettings;
}

function persistWebSettingsCache() {
  try {
    localStorage.setItem(WEB_SETTINGS_KEY, JSON.stringify(webSettings));
  } catch (_) {}
}

function applyVideoGenFeatureVisibility() {
  const featureDisplayMap = {
    vgChangeCover: "vg-feature-change-cover",
    vgChangeAudio: "vg-feature-change-audio",
    vgChangeLyric: "vg-feature-change-lyric",
    vgExportVideo: "vg-feature-export-video",
  };

  Object.entries(featureDisplayMap).forEach(([key, elementId]) => {
    const element = document.getElementById(elementId);
    if (!element) return;
    element.style.display = webSettings[key] ? "flex" : "none";
  });
}

function floatingLyricsEnabled() {
  return !webSettings.disableFloatingLyrics;
}

function syncFloatingLyricsSetting() {
  document.body.classList.toggle(
    "floating-lyrics-disabled",
    !floatingLyricsEnabled(),
  );
  if (!window.KaraokeLyrics) return;
  if (!floatingLyricsEnabled()) {
    window.KaraokeLyrics.hide();
    return;
  }
  if (window.ap?.list?.audios) {
    window.ap.list.audios.forEach((audio) => {
      if (!audio || audio.raw_lrc || audio.lrc || !audio.custom_id) return;
      const lyricURLs = lyricURLsForSong({
        id: audio.custom_id,
        source: audio.source || "",
        name: audio.name || "",
        artist: audio.artist || "",
        album: audio.album || "",
        duration: audio.duration || 0,
        extra: audio.extra || "",
      });
      audio.lrc = lyricURLs.line;
      audio.raw_lrc = lyricURLs.auto;
    });
  }
  if (window.ap?.audio && !window.ap.audio.paused) {
    window.KaraokeLyrics.load(getCurrentAPlayerAudio());
  }
}

function lyricURLsForPlayback(song) {
  if (!floatingLyricsEnabled()) {
    return { line: "", auto: "", download: lyricURLsForSong(song).download };
  }
  return lyricURLsForSong(song);
}

function syncDownloadDirPresetFromInput() {
  const presetSelect = document.getElementById("setting-download-dir-preset");
  const dirInput = document.getElementById("setting-download-dir");
  if (!presetSelect || !dirInput) return;

  const value = String(dirInput.value || "").trim();
  const normalizedValue = value.replace(/\\/g, "/");
  presetSelect.value = DOWNLOAD_DIR_PRESETS.has(normalizedValue)
    ? normalizedValue
    : DOWNLOAD_DIR_CUSTOM_VALUE;
}

function bindDownloadDirPresetEvents() {
  const presetSelect = document.getElementById("setting-download-dir-preset");
  const dirInput = document.getElementById("setting-download-dir");
  if (!presetSelect || !dirInput || presetSelect.dataset.bound === "1") return;

  presetSelect.dataset.bound = "1";
  presetSelect.addEventListener("change", () => {
    if (presetSelect.value === DOWNLOAD_DIR_CUSTOM_VALUE) {
      dirInput.focus();
      return;
    }
    dirInput.value = presetSelect.value;
  });
  dirInput.addEventListener("input", syncDownloadDirPresetFromInput);
}

function applyWebSettings(settings) {
  const wasAutoSwitchInvalidSourcesEnabled =
    isAutoSwitchInvalidSourcesEnabled();
  webSettings = normalizeWebSettings(settings);
  persistWebSettingsCache();

  const embedToggle = document.getElementById("setting-embed-download");
  if (embedToggle) {
    embedToggle.checked = webSettings.embedDownload;
  }

  const dirInput = document.getElementById("setting-download-dir");
  if (dirInput) {
    dirInput.value = webSettings.downloadDir;
  }
  bindDownloadDirPresetEvents();
  syncDownloadDirPresetFromInput();

  const filenameTemplateInput = document.getElementById(
    "setting-download-filename-template",
  );
  if (filenameTemplateInput) {
    filenameTemplateInput.value = webSettings.downloadFilenameTemplate;
  }

  const floatingLyricsToggle = document.getElementById(
    "setting-floating-lyrics",
  );
  if (floatingLyricsToggle) {
    floatingLyricsToggle.checked = !webSettings.disableFloatingLyrics;
  }

  const webPageSizeInput = document.getElementById("setting-web-page-size");
  if (webPageSizeInput) {
    webPageSizeInput.value = String(
      webSettings.webPageSize || DEFAULT_WEB_PAGE_SIZE,
    );
  }

  const cliPageSizeInput = document.getElementById("setting-cli-page-size");
  if (cliPageSizeInput) {
    cliPageSizeInput.value = String(
      webSettings.cliPageSize || DEFAULT_CLI_PAGE_SIZE,
    );
  }

  const autoSwitchInvalidSourcesToggle = document.getElementById(
    "setting-auto-switch-invalid-sources",
  );
  if (autoSwitchInvalidSourcesToggle) {
    autoSwitchInvalidSourcesToggle.checked =
      webSettings.autoSwitchInvalidSources;
  }

  const autoCacheOnPlayToggle = document.getElementById(
    "setting-auto-cache-on-play",
  );
  if (autoCacheOnPlayToggle) {
    autoCacheOnPlayToggle.checked = webSettings.autoCacheOnPlay;
  }

  const vgChangeCoverToggle = document.getElementById(
    "setting-vg-change-cover",
  );
  if (vgChangeCoverToggle) {
    vgChangeCoverToggle.checked = webSettings.vgChangeCover;
  }

  const vgChangeAudioToggle = document.getElementById(
    "setting-vg-change-audio",
  );
  if (vgChangeAudioToggle) {
    vgChangeAudioToggle.checked = webSettings.vgChangeAudio;
  }

  const vgChangeLyricToggle = document.getElementById(
    "setting-vg-change-lyric",
  );
  if (vgChangeLyricToggle) {
    vgChangeLyricToggle.checked = webSettings.vgChangeLyric;
  }

  const vgExportVideoToggle = document.getElementById(
    "setting-vg-export-video",
  );
  if (vgExportVideoToggle) {
    vgExportVideoToggle.checked = webSettings.vgExportVideo;
  }

  applyVideoGenFeatureVisibility();
  syncFloatingLyricsSetting();
  refreshDownloadLinks();
  if (webSettings.autoSwitchInvalidSources) {
    if (!wasAutoSwitchInvalidSourcesEnabled) {
      autoSwitchInvalidLastKey = "";
    }
    scheduleAutoSwitchInvalidSources(0);
  } else {
    clearAutoSwitchInvalidTimer();
  }
}

async function fetchWebSettings() {
  try {
    const response = await fetch(API_ROOT + "/settings");
    if (!response.ok) return;
    const data = await response.json();
    applyWebSettings(data);
  } catch (_) {}
}

function systemConfigReturnURL() {
  const url = new URL(window.location.href);
  url.searchParams.set(OPEN_CONFIG_QUERY, "1");
  return url.pathname + url.search + url.hash;
}

function redirectToConfigAuth(setupRequired = false) {
  const next = encodeURIComponent(systemConfigReturnURL());
  window.location.href = `${API_ROOT}/${setupRequired ? "setup" : "login"}?next=${next}`;
}

function handleConfigAuthResponse(response, payload = null) {
  if (response.status !== 401) return false;
  redirectToConfigAuth(!!(payload && payload.setupRequired));
  return true;
}

function setAuthFloatLoggedIn(loggedIn) {
  const form = document.getElementById("auth-float-form");
  const icon = document.getElementById("auth-float-icon");
  if (!form || !icon) return;
  form.dataset.loggedIn = loggedIn ? "1" : "0";
  form.title = loggedIn ? "退出登录" : "登录";
  form.classList.toggle("auth-login", !loggedIn);
  form.classList.toggle("auth-logout", loggedIn);
  icon.className = loggedIn
    ? "fa-solid fa-arrow-right-from-bracket"
    : "fa-solid fa-right-to-bracket";
}

async function refreshAuthFloat() {
  try {
    // 桌面模式（127.0.0.1:37777）无认证概念，直接显示未登录
    if (window.location.port === "37777" && window.location.hostname === "127.0.0.1") {
      setAuthFloatLoggedIn(false);
      return;
    }
    const response = await fetch(API_ROOT + "/cookies", {
      method: "HEAD",
      headers: { Accept: "application/json" },
    });
    setAuthFloatLoggedIn(response.ok);
  } catch (_) {
    setAuthFloatLoggedIn(false);
  }
}

function bindAuthFloat() {
  const form = document.getElementById("auth-float-form");
  if (!form || form.dataset.bound === "1") return;
  form.dataset.bound = "1";
  form.addEventListener("submit", function (event) {
    if (form.dataset.loggedIn === "1") return;
    event.preventDefault();
    openSystemConfig();
  });
}

function buildDownloadRequestURL(
  id,
  source,
  name,
  artist,
  album,
  cover,
  extra,
  options = {},
) {
  const params = new URLSearchParams({
    id: String(id || ""),
    source: String(source || ""),
    name: String(name || ""),
    artist: String(artist || ""),
  });

  const albumValue = String(album || "");
  if (albumValue !== "") {
    params.set("album", albumValue);
  }
  const coverValue = String(cover || "");
  if (coverValue !== "") {
    params.set("cover", coverValue);
  }
  const extraValue = String(extra || "");
  if (extraValue !== "" && extraValue !== "{}" && extraValue !== "null") {
    params.set("extra", extraValue);
  }
  if (options.embed) {
    params.set("embed", "1");
  }
  if (options.saveLocal) {
    params.set("save_local", "1");
  }
  if (options.stream) {
    params.set("stream", "1");
  }

  return `${API_ROOT}/download?${params.toString()}`;
}

function buildStreamURL(id, source, name, artist, album, cover, extra) {
  return buildDownloadRequestURL(
    id,
    source,
    name,
    artist,
    album,
    cover,
    extra,
    {
      stream: true,
    },
  );
}

function buildDownloadURL(id, source, name, artist, album, cover, extra) {
  return buildDownloadRequestURL(
    id,
    source,
    name,
    artist,
    album,
    cover,
    extra,
    {
      embed: webSettings.embedDownload,
      saveLocal: true,
    },
  );
}

function buildBrowserDownloadURL(
  id,
  source,
  name,
  artist,
  album,
  cover,
  extra,
) {
  return buildDownloadRequestURL(
    id,
    source,
    name,
    artist,
    album,
    cover,
    extra,
    {
      embed: webSettings.embedDownload,
    },
  );
}

function buildLyricRequestURL(song, endpoint = "lyric", format = "auto") {
  const params = new URLSearchParams({
    id: String(song?.id || ""),
    source: String(song?.source || ""),
    name: String(song?.name || ""),
    artist: String(song?.artist || ""),
    album: String(song?.album || ""),
    duration: String(song?.duration || 0),
    format: String(format || "auto"),
  });

  const extraValue =
    typeof song?.extra === "string"
      ? song.extra
      : JSON.stringify(song?.extra || {});
  if (extraValue && extraValue !== "{}" && extraValue !== "null") {
    params.set("extra", extraValue);
  }

  return `${API_ROOT}/${endpoint}?${params.toString()}`;
}

function lyricURLsForSong(song) {
  return {
    line: buildLyricRequestURL(song, "lyric", "line"),
    auto: buildLyricRequestURL(song, "lyric", "auto"),
    download: buildLyricRequestURL(song, "download_lrc", "auto"),
  };
}

window.buildLyricRequestURL = buildLyricRequestURL;
window.lyricURLsForSong = lyricURLsForSong;

function updateDownloadButton(link) {
  if (!link) return;

  const card = link.closest(".song-card");
  if (!card) return;

  const ds = card.dataset;
  link.href = buildDownloadURL(
    ds.id,
    ds.source,
    ds.name,
    ds.artist,
    ds.album || "",
    ds.cover || "",
    ds.extra || "",
  );
  link.title = "保存到本地目录";
}

function updateBrowserDownloadButton(link) {
  if (!link) return;

  const card = link.closest(".song-card");
  if (!card) return;

  const ds = card.dataset;
  link.href = buildBrowserDownloadURL(
    ds.id,
    ds.source,
    ds.name,
    ds.artist,
    ds.album || "",
    ds.cover || "",
    ds.extra || "",
  );
  link.title = "浏览器下载";
}

function updateLyricButton(link) {
  if (!link) return;

  const card = link.closest(".song-card");
  const song = songFromCard(card);
  if (!song) return;

  link.href = lyricURLsForSong(song).download;
}

function buildCoverDownloadURL(song) {
  const source = String(song?.source || "");
  const params = new URLSearchParams();
  if (isLocalMusicSourceValue(source)) {
    params.set("id", String(song?.id || ""));
    params.set("download", "1");
    params.set("name", String(song?.name || ""));
    params.set("artist", String(song?.artist || ""));
    return `${API_ROOT}/local_music/cover?${params.toString()}`;
  }

  params.set(
    "url",
    String(song?.cover || "https://via.placeholder.com/600?text=No+Cover"),
  );
  params.set("name", String(song?.name || ""));
  params.set("artist", String(song?.artist || ""));
  return `${API_ROOT}/download_cover?${params.toString()}`;
}

function updateCoverButton(link) {
  if (!link) return;

  const card = link.closest(".song-card");
  const song = songFromCard(card);
  if (!song) return;

  link.href = buildCoverDownloadURL(song);
}

function refreshDownloadLinks(root = document) {
  root.querySelectorAll(".song-card").forEach((card) => {
    updateDownloadButton(card.querySelector(".btn-download"));
    updateBrowserDownloadButton(card.querySelector(".btn-browser-download"));
    updateLyricButton(card.querySelector(".btn-lyric"));
    updateCoverButton(card.querySelector(".btn-cover"));
  });
}

function withSaveLocalParam(url) {
  const target = new URL(String(url || ""), window.location.href);
  target.searchParams.set("save_local", "1");
  return target.toString();
}

async function requestLocalDownload(url) {
  const response = await fetch(withSaveLocalParam(url), {
    method: "POST",
    headers: {
      Accept: "application/json",
      "X-Requested-With": "XMLHttpRequest",
    },
  });
  const data = await response.json().catch(() => null);
  if (!response.ok || !data || data.error) {
    throw new Error((data && data.error) || "保存失败");
  }
  return data;
}

function formatBatchSongLabel(song) {
  const name = song && song.name ? song.name : "Unknown";
  const artist = song && song.artist ? song.artist : "Unknown";
  return `${name} - ${artist}`;
}

function buildBatchFailureMessage(failures, title) {
  if (!failures || failures.length === 0) {
    return "";
  }

  let message = `\n\n${title} ${failures.length} 首：`;
  failures.forEach((item, index) => {
    const reason = item.reason ? `：${item.reason}` : "";
    message += `\n${index + 1}. ${formatBatchSongLabel(item.song)}${reason}`;
  });
  return message;
}

function showToast(title, message = "", type = "info", duration = 0) {
  let container = document.getElementById("app-toast-container");
  if (!container) {
    container = document.createElement("div");
    container.id = "app-toast-container";
    container.className = "app-toast-container";
    document.body.appendChild(container);
  }

  const toast = document.createElement("div");
  toast.className = `app-toast app-toast-${type}`;
  toast.innerHTML = [
    '<button type="button" class="app-toast-close" aria-label="关闭提示">&times;</button>',
    `<div class="app-toast-title">${escapeHTML(title)}</div>`,
    message
      ? `<div class="app-toast-message">${escapeHTML(message)}</div>`
      : "",
  ].join("");
  container.appendChild(toast);

  const close = () => {
    toast.classList.add("app-toast-hide");
    window.setTimeout(() => toast.remove(), 220);
  };
  toast.querySelector(".app-toast-close")?.addEventListener("click", close);
  if (duration > 0) {
    window.setTimeout(close, duration);
  }
}

function inferExtFromContentType(contentType) {
  const raw = String(contentType || "")
    .toLowerCase()
    .split(";")[0]
    .trim();
  switch (raw) {
    case "audio/flac":
    case "audio/x-flac":
      return "flac";
    case "audio/ogg":
    case "application/ogg":
      return "ogg";
    case "audio/mp4":
    case "audio/x-m4a":
    case "audio/aac":
    case "audio/aacp":
      return "m4a";
    case "audio/x-ms-wma":
    case "audio/wma":
      return "wma";
    default:
      return "mp3";
  }
}

function getDownloadFilenameFromResponse(response, song) {
  const disposition = response.headers.get("Content-Disposition") || "";
  const encodedMatch = disposition.match(/filename\*\s*=\s*utf-8''([^;]+)/i);
  if (encodedMatch && encodedMatch[1]) {
    try {
      return decodeURIComponent(encodedMatch[1].trim().replace(/^"|"$/g, ""));
    } catch (_) {}
  }

  const plainMatch =
    disposition.match(/filename\s*=\s*"([^"]+)"/i) ||
    disposition.match(/filename\s*=\s*([^;]+)/i);
  if (plainMatch && plainMatch[1]) {
    return plainMatch[1].trim().replace(/^"|"$/g, "");
  }

  return `${formatBatchSongLabel(song)}.${inferExtFromContentType(response.headers.get("Content-Type"))}`;
}

async function requestBrowserDownload(song) {
  const url = String(song?.url || "").trim();
  if (!url) {
    throw new Error("下载地址为空");
  }

  let frame = document.getElementById("browser-download-frame");
  if (!frame) {
    frame = document.createElement("iframe");
    frame.id = "browser-download-frame";
    frame.name = "browser-download-frame";
    frame.style.display = "none";
    document.body.appendChild(frame);
  }

  const link = document.createElement("a");

  link.href = url;
  link.target = frame.name;
  link.rel = "noopener";
  link.style.display = "none";
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);

  await new Promise((resolve) => window.setTimeout(resolve, 250));

  return {
    triggered: true,
    warning: "",
  };
}

async function handleDownloadClick(link) {
  if (!link) {
    return false;
  }

  link.style.pointerEvents = "none";
  link.style.opacity = "0.6";
  try {
    const data = await requestLocalDownload(link.href);
    let message = data.path || webSettings.downloadDir;
    if (data.warning) {
      message += `\n提示: ${data.warning}`;
    }
    showToast("下载完成", message, data.warning ? "warning" : "success", 0);
    return true;
  } catch (error) {
    showToast("下载失败", error.message || "下载失败", "error", 0);
  } finally {
    link.style.pointerEvents = "";
    link.style.opacity = "";
  }

  return false;
}

let navigationAbortController = null;
let pageNavigationEventsBound = false;
let songSortMode = "default";
let songSortDirection = "desc";

function isAppRoute(pathname) {
  return pathname === API_ROOT || pathname.startsWith(`${API_ROOT}/`);
}

function bindSourceSelectorButtons(root = document) {
  const checkboxes = root.querySelectorAll(".source-checkbox");

  const btnAll = document.getElementById("btn-all");
  if (btnAll) {
    btnAll.onclick = () => {
      checkboxes.forEach((cb) => {
        if (!cb.disabled) cb.checked = true;
      });
    };
  }

  const btnNone = document.getElementById("btn-none");
  if (btnNone) {
    btnNone.onclick = () => {
      checkboxes.forEach((cb) => {
        if (!cb.disabled) cb.checked = false;
      });
    };
  }
}

function isMobileSourceSelectorViewport() {
  try {
    return window.matchMedia("(max-width: 720px)").matches;
  } catch (_) {
    return (window.innerWidth || 0) <= 720;
  }
}

function applySourceSelectorCollapsed(selector, collapsed) {
  if (!selector) return;
  selector.classList.toggle("is-collapsed", !!collapsed);
  const btn = selector.querySelector(".source-collapse-btn");
  if (btn) btn.setAttribute("aria-expanded", collapsed ? "false" : "true");
}

function toggleSourceSelector(force) {
  const selector = document.getElementById("source-selector");
  if (!selector) return;
  const collapsed =
    typeof force === "boolean"
      ? force
      : !selector.classList.contains("is-collapsed");
  applySourceSelectorCollapsed(selector, collapsed);
  try {
    sessionStorage.setItem("sourceSelectorCollapsed", collapsed ? "1" : "0");
  } catch (_) {}
}

function initSourceSelectorCollapse(root = document) {
  const scope =
    root && typeof root.querySelector === "function" ? root : document;
  const selector = scope.querySelector("#source-selector");
  if (!selector) return;
  if (selector.dataset.collapseInit === "1") return;
  selector.dataset.collapseInit = "1";

  const isMobile = isMobileSourceSelectorViewport();
  let collapsed;
  try {
    const stored = sessionStorage.getItem("sourceSelectorCollapsed");
    if (stored === "1" || stored === "0") {
      collapsed = stored === "1";
    }
  } catch (_) {}
  if (typeof collapsed !== "boolean") {
    collapsed = isMobile;
  }
  applySourceSelectorCollapsed(selector, collapsed);
}

function bindSearchForm(root = document) {
  const searchForm = root.querySelector("#search-form");
  if (!searchForm) return;

  searchForm.onsubmit = (event) => {
    event.preventDefault();

    const pageInput = searchForm.querySelector('input[name="page"]');
    if (pageInput) {
      pageInput.value = "1";
    }

    const targetURL = new URL(searchForm.action, window.location.href);
    const params = new URLSearchParams();
    new FormData(searchForm).forEach((value, key) => {
      params.append(key, String(value));
    });
    targetURL.search = params.toString();

    navigateTo(targetURL.toString());
  };
}

function bindSongCardCovers(root = document) {
  const cards = root.querySelectorAll(".song-card");
  cards.forEach((card, index) => {
    queueInspectSong(card, index * INSPECT_REQUEST_DELAY_MS);

    const coverWrap = card.querySelector(".cover-wrapper");
    if (!coverWrap) return;

    coverWrap.style.cursor = "pointer";
    coverWrap.title = "点击生成视频";
    coverWrap.onclick = (e) => {
      e.stopPropagation();
      if (window.VideoGen) {
        const img = coverWrap.querySelector("img");
        const currentCover = img ? img.src : card.dataset.cover || "";

        window.VideoGen.open({
          id: card.dataset.id,
          source: card.dataset.source,
          name: card.dataset.name,
          artist: card.dataset.artist,
          album: card.dataset.album || "",
          cover: currentCover,
          duration: parseInt(card.dataset.duration) || 0,
          extra: card.dataset.extra || "",
        });
      } else {
        console.error("VideoGen library not loaded.");
        alert("视频生成组件加载失败，请刷新页面重试");
      }
    };
  });
}

// === 本地音乐匹配缓存 ===
let localMusicMatchCache = {};
let batchMatchTimer = null;
let autoCachePending = {}; // 防止重复缓存请求
// This state is used by the DOM bootstrap below, so initialize it before any
// page lifecycle callback can access it.
let currentPlayingId = null;
window.currentPlayingId = null;

function isAutoCacheOnPlayEnabled() {
  return webSettings.autoCacheOnPlay !== false;
}

function scheduleAutoCache(audio) {
  if (!isAutoCacheOnPlayEnabled()) return;
  const src = audio.source || "";
  if (src === "local" || src === "local-file") return;
  const key = audio.custom_id || audio.name || "";
  if (!key || autoCachePending[key] || localMusicMatchCache[audio.custom_id])
    return;

  autoCachePending[key] = true;
  setTimeout(() => autoCacheTrack(audio, key), 2000);
}

function scheduleAutoCacheMatch(audio, key, attempt = 0) {
  const delays = [1500, 3500, 6500];
  const delay = delays[attempt];
  if (!audio.custom_id || delay === undefined) {
    delete autoCachePending[key];
    return;
  }

  setTimeout(async () => {
    await batchMatchLocalMusic();
    if (localMusicMatchCache[audio.custom_id]) {
      delete autoCachePending[key];
      return;
    }
    scheduleAutoCacheMatch(audio, key, attempt + 1);
  }, delay);
}

async function autoCacheTrack(
  audio,
  key = audio.custom_id || audio.name || "",
) {
  if (
    !isAutoCacheOnPlayEnabled() ||
    !key ||
    localMusicMatchCache[audio.custom_id]
  ) {
    delete autoCachePending[key];
    return;
  }

  try {
    const resp = await fetch(API_ROOT + "/local_music/auto_cache", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Requested-With": "XMLHttpRequest",
      },
      body: JSON.stringify({
        id: audio.custom_id || "",
        source: audio.source || "",
        name: audio.name || "",
        artist: audio.artist || "",
        album: audio.album || "",
        cover: audio.cover || "",
        extra: audio.extra || {},
      }),
    });
    if (!resp.ok) return;
    const data = await resp.json();
    if (data.status === "started" || data.status === "in_progress") {
      scheduleAutoCacheMatch(audio, key);
      return;
    }
  } catch (_) {}
  delete autoCachePending[key];
}

function scheduleBatchLocalMusicMatch() {
  // 只对搜索结果页做匹配（本地音乐页不需要）
  if (isLocalMusicPageActive()) return;
  if (batchMatchTimer) clearTimeout(batchMatchTimer);
  batchMatchTimer = setTimeout(batchMatchLocalMusic, 300);
}

async function batchMatchLocalMusic() {
  const cards = Array.from(document.querySelectorAll(".song-card"));
  const onlineCards = cards.filter((card) => {
    const src = card.dataset.source || "";
    return src !== "local" && src !== "local-file";
  });
  if (onlineCards.length === 0) return;

  const queries = onlineCards.map((card) => ({
    name: card.dataset.name || "",
    artist: card.dataset.artist || "",
  }));

  try {
    const resp = await fetch(API_ROOT + "/local_music/batch_match", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(queries),
    });
    if (!resp.ok) return;
    const data = await resp.json();
    if (!data.matches || data.matches.length === 0) return;

    for (const m of data.matches) {
      const card = onlineCards[m.qi];
      if (!card) continue;
      // 缓存匹配结果（key = 歌曲id）
      localMusicMatchCache[card.dataset.id] = m;
      // 添加 "本地已有" 标签
      addLocalMatchBadge(card, m);
    }
  } catch (_) {
    // 静默失败
  }
}

function addLocalMatchBadge(card, match) {
  // 避免重复添加
  if (card.querySelector(".tag-local-match")) return;
  const tagsEl = card.querySelector(".tags");
  if (!tagsEl) return;
  const badge = document.createElement("span");
  badge.className = "tag tag-local-match";
  badge.title = `本地已有: ${match.ext || "音频"} (${formatSizeBytes(match.size || 0)})，播放将优先使用本地文件`;
  badge.textContent = "本地已有";
  badge.style.cssText = "background:#e3f2fd;color:#3182ce;cursor:default;";
  // 插入到 source tag 后面
  const srcTag = tagsEl.querySelector(".tag-src");
  if (srcTag && srcTag.nextSibling) {
    tagsEl.insertBefore(badge, srcTag.nextSibling);
  } else {
    tagsEl.appendChild(badge);
  }
}

function formatSizeBytes(bytes) {
  if (!bytes || bytes <= 0) return "";
  if (bytes < 1024) return bytes + "B";
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + "KB";
  return (bytes / 1048576).toFixed(1) + "MB";
}

function initializePageContent(root = document) {
  resetAutoSwitchInvalidState();
  bindSourceSelectorButtons(root);
  initSourceSelectorCollapse(root);
  bindSearchForm(root);
  bindSongSortControls(root);

  const initialTypeEl = root.querySelector('input[name="type"]:checked');
  if (initialTypeEl) {
    toggleSearchType(initialTypeEl.value);
  }

  refreshDownloadLinks(root);
  bindSongCardCovers(root);
  updateBatchToolbar();
  highlightCard(currentPlayingId);
  syncAllPlayButtons();
  syncMediaSession();
  initializeLocalMusicPage(root);
  scheduleBatchLocalMusicMatch();
}

function shouldHandleInternalNavigation(link, event) {
  if (!link || event.defaultPrevented) return false;
  if (
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey
  ) {
    return false;
  }
  if (link.hasAttribute("download")) return false;

  const hrefAttr = String(link.getAttribute("href") || "").trim();
  if (
    !hrefAttr ||
    hrefAttr.startsWith("#") ||
    hrefAttr.startsWith("javascript:") ||
    hrefAttr.startsWith("mailto:") ||
    hrefAttr.startsWith("tel:")
  ) {
    return false;
  }

  const targetAttr = String(link.getAttribute("target") || "")
    .trim()
    .toLowerCase();
  if (targetAttr && targetAttr !== "_self") {
    return false;
  }

  if (
    link.classList.contains("btn-download") ||
    link.classList.contains("btn-lyric") ||
    link.classList.contains("btn-cover")
  ) {
    return false;
  }

  let targetURL;
  try {
    targetURL = new URL(hrefAttr, window.location.href);
  } catch (_) {
    return false;
  }

  return (
    targetURL.origin === window.location.origin &&
    isAppRoute(targetURL.pathname)
  );
}

async function navigateTo(url, options = {}) {
  let targetURL;
  try {
    targetURL = new URL(url, window.location.href);
  } catch (_) {
    return false;
  }

  if (
    targetURL.origin !== window.location.origin ||
    !isAppRoute(targetURL.pathname)
  ) {
    window.location.href = targetURL.toString();
    return false;
  }

  if (navigationAbortController) {
    navigationAbortController.abort();
  }

  const controller = new AbortController();
  navigationAbortController = controller;

  try {
    const response = await fetch(targetURL.toString(), {
      headers: {
        "X-Requested-With": "XMLHttpRequest",
      },
      signal: controller.signal,
    });
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }

    const html = await response.text();
    const parser = new DOMParser();
    const nextDoc = parser.parseFromString(html, "text/html");
    const nextContainer = nextDoc.querySelector(".container");
    const currentContainer = document.querySelector(".container");

    if (!nextContainer || !currentContainer) {
      throw new Error("missing container");
    }

    currentContainer.innerHTML = nextContainer.innerHTML;
    defaultDocumentTitle = nextDoc.title || defaultDocumentTitle;
    document.title = defaultDocumentTitle;

    const historyMode = options.historyMode || "push";
    if (historyMode === "replace") {
      window.history.replaceState(null, "", targetURL.toString());
    } else if (historyMode !== "none") {
      if (targetURL.toString() === window.location.href) {
        window.history.replaceState(null, "", targetURL.toString());
      } else {
        window.history.pushState(null, "", targetURL.toString());
      }
    }

    initializePageContent(currentContainer);
    updateFloatPageNav();

    if (options.scroll !== false) {
      window.scrollTo({ top: 0, behavior: "auto" });
    }

    return true;
  } catch (error) {
    if (error && error.name === "AbortError") {
      return false;
    }
    window.location.href = targetURL.toString();
    return false;
  } finally {
    if (navigationAbortController === controller) {
      navigationAbortController = null;
    }
  }
}

function refreshCurrentPageContent(options = {}) {
  if (isLocalMusicPageActive()) {
    return loadLocalMusicPage(getCurrentLocalMusicPage(), {
      force: true,
      updateHistory: false,
      scroll: options.scroll,
    });
  }
  return navigateTo(window.location.href, {
    historyMode: "replace",
    scroll: false,
    ...options,
  });
}

function isEditableElement(element) {
  if (!(element instanceof Element)) return false;
  if (element.isContentEditable) return true;
  return !!element.closest(
    '[contenteditable=""], [contenteditable="true"], input, textarea, select',
  );
}

function hasVisibleModalOverlay() {
  return Array.from(document.querySelectorAll(".modal-overlay")).some(
    (overlay) => {
      return window.getComputedStyle(overlay).display !== "none";
    },
  );
}

function getActivePaginationState() {
  const paginationBar = Array.from(
    document.querySelectorAll(
      ".pagination-bar[data-current-page][data-total-pages]",
    ),
  ).find((bar) => bar.offsetParent !== null);
  if (!paginationBar) return null;

  const currentPage = parsePositiveInt(paginationBar.dataset.currentPage, 1);
  const totalPages = parsePositiveInt(paginationBar.dataset.totalPages, 1);
  if (totalPages <= 1) return null;

  return { currentPage, totalPages };
}

function handlePaginationShortcut(event) {
  if (event.defaultPrevented || event.isComposing) return;
  if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
  if (
    isEditableElement(event.target) ||
    isEditableElement(document.activeElement)
  )
    return;
  if (hasVisibleModalOverlay()) return;

  let delta = 0;
  if (event.key === "PageUp") {
    delta = -1;
  } else if (event.key === "PageDown") {
    delta = 1;
  }
  if (delta === 0) return;

  const state = getActivePaginationState();
  if (!state) return;

  const nextPage = state.currentPage + delta;
  if (nextPage < 1 || nextPage > state.totalPages) return;

  event.preventDefault();
  goToPage(nextPage);
}

function togglePlayback() {
  if (typeof ap === "undefined" || !ap || !ap.audio) return;
  if (!ap.list || !Array.isArray(ap.list.audios) || ap.list.audios.length === 0)
    return;
  if (ap.audio.paused) {
    ap.play();
  } else {
    ap.pause();
  }
}

function handlePlaybackShortcut(event) {
  if (event.defaultPrevented || event.isComposing) return;
  if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
  if (event.code !== "Space" && event.key !== " " && event.key !== "Spacebar")
    return;
  if (
    isEditableElement(event.target) ||
    isEditableElement(document.activeElement)
  )
    return;
  if (hasVisibleModalOverlay()) return;
  if (
    typeof ap === "undefined" ||
    !ap ||
    !ap.list ||
    !ap.list.audios ||
    ap.list.audios.length === 0
  )
    return;

  event.preventDefault();
  togglePlayback();
}

function bindPageNavigationEvents() {
  if (pageNavigationEventsBound) return;
  pageNavigationEventsBound = true;

  document.addEventListener("click", async function (event) {
    const link = event.target.closest(".btn-download");
    if (!link) return;
    event.preventDefault();
    await handleDownloadClick(link);
  });

  document.addEventListener(
    "click",
    function (event) {
      const link = event.target.closest("a");
      if (!shouldHandleInternalNavigation(link, event)) return;

      event.preventDefault();
      navigateTo(link.href);
    },
    true,
  );

  document.addEventListener("keydown", handlePaginationShortcut);
  document.addEventListener("keydown", handlePlaybackShortcut);

  window.addEventListener("popstate", function () {
    navigateTo(window.location.href, {
      historyMode: "none",
      scroll: false,
    });
  });
}

document.addEventListener("DOMContentLoaded", function () {
  loadWebSettingsFromCache();
  applyWebSettings(webSettings);
  bindAuthFloat();
  refreshAuthFloat();
  fetchWebSettings().finally(() => maybeAutoCheckUpdate());
  bindPageNavigationEvents();
  initializePageContent(document);
  updateFloatPageNav();
  if (
    new URLSearchParams(window.location.search).get(OPEN_CONFIG_QUERY) === "1"
  ) {
    const url = new URL(window.location.href);
    url.searchParams.delete(OPEN_CONFIG_QUERY);
    window.history.replaceState(null, "", url.toString());
    openSystemConfig();
  }
  return;
  /*

    const cards = document.querySelectorAll('.song-card');
    cards.forEach((card, index) => {
        queueInspectSong(card, index * INSPECT_REQUEST_DELAY_MS);
    });

    cards.forEach(card => {
        const coverWrap = card.querySelector('.cover-wrapper');
        if (!coverWrap) return;
        
        coverWrap.style.cursor = 'pointer';
        coverWrap.title = '点击生成视频';
        
        coverWrap.onclick = (e) => {
            e.stopPropagation();
            if (window.VideoGen) {
                const img = coverWrap.querySelector('img');
                const currentCover = img ? img.src : (card.dataset.cover || '');

                window.VideoGen.open({
                    id: card.dataset.id,
                    source: card.dataset.source,
                    name: card.dataset.name,
                    artist: card.dataset.artist,
                    cover: currentCover,
                    duration: parseInt(card.dataset.duration) || 0
                });
            } else {
                console.error("VideoGen library not loaded.");
                alert("视频生成组件加载失败，请刷新页面重试");
            }
        };
    });

    document.addEventListener('click', async function(event) {
        const link = event.target.closest('.btn-download');
        if (!link) return;
        if (!webSettings.downloadToLocal) return;
        event.preventDefault();
        await handleDownloadClick(link);
    });

    updateBatchToolbar();

    syncAllPlayButtons();
    */
});

function toggleSearchType(type) {
  const checkboxes = document.querySelectorAll(".source-checkbox");
  const searchInput = document.getElementById("search-keyword");
  const placeholders = {
    song: "搜索歌曲、歌手，或直接粘贴分享链接",
    playlist: "搜索歌单、创建者，或直接粘贴歌单链接",
    album: "搜索专辑、歌手，或直接粘贴专辑链接",
  };

  if (searchInput && placeholders[type]) {
    searchInput.placeholder = placeholders[type];
  }

  checkboxes.forEach((cb) => {
    let isSupported = true;
    if (type === "playlist") {
      isSupported = cb.dataset.playlistSupported === "true";
    } else if (type === "album") {
      isSupported = cb.dataset.albumSupported === "true";
    }

    if (type === "playlist" || type === "album") {
      if (!isSupported) {
        cb.disabled = true;
        cb.checked = false;
      } else {
        cb.disabled = false;
      }
    } else {
      cb.disabled = false;
    }
  });
}

function goToRecommend() {
  const supported = ["netease", "qq", "kugou", "kuwo"];
  const selected = [];
  document.querySelectorAll(".source-checkbox:checked").forEach((cb) => {
    if (supported.includes(cb.value)) {
      selected.push(cb.value);
    }
  });

  if (selected.length === 0) {
    navigateTo(API_ROOT + "/recommend?sources=" + supported.join("&sources="));
  } else {
    navigateTo(API_ROOT + "/recommend?sources=" + selected.join("&sources="));
  }
}

function switchCategorySource(tab) {
  if (!tab) return;
  const panelId = tab.getAttribute("data-target");
  if (!panelId) return;
  const scope = tab.closest(".category-panel") || document;
  scope.querySelectorAll(".category-source-tab").forEach(function (t) {
    t.classList.toggle("is-active", t === tab);
  });
  scope.querySelectorAll(".category-source-panel").forEach(function (p) {
    p.classList.toggle("is-active", p.id === panelId);
  });
}

function goToPlaylistCategories() {
  const selected = [];
  document.querySelectorAll(".source-checkbox:checked").forEach((cb) => {
    if (cb.dataset.categorySupported === "true") {
      selected.push(cb.value);
    }
  });

  if (selected.length === 0) {
    navigateTo(API_ROOT + "/playlist_categories");
  } else {
    navigateTo(
      API_ROOT +
        "/playlist_categories?sources=" +
        selected.map(encodeURIComponent).join("&sources="),
    );
  }
}

function goToUserPlaylists() {
  const selected = [];
  document.querySelectorAll(".source-checkbox:checked").forEach((cb) => {
    if (cb.dataset.userPlaylistSupported === "true") {
      selected.push(cb.value);
    }
  });

  if (selected.length === 0) {
    navigateTo(API_ROOT + "/user_playlists");
  } else {
    navigateTo(
      API_ROOT +
        "/user_playlists?sources=" +
        selected.map(encodeURIComponent).join("&sources="),
    );
  }
}

function goToPage(page, pageSize) {
  const target = parseInt(page, 10);
  if (!Number.isFinite(target) || target < 1) return;
  if (isLocalMusicPageActive()) {
    loadLocalMusicPage(target, {
      updateHistory: true,
      scroll: true,
    });
    return;
  }
  const url = new URL(window.location.href);
  url.searchParams.set("page", String(target));
  const ps = parseInt(pageSize, 10);
  if (Number.isFinite(ps) && ps > 0) {
    url.searchParams.set("page_size", String(ps));
  }
  navigateTo(url.toString());
}

function changePageSize(size) {
  // 同时更新隐藏的 form 输入和下拉框
  const hiddenInput = document.getElementById("page-size-hidden");
  if (hiddenInput) hiddenInput.value = String(size);

  const dropdown = document.getElementById("page-size-select");
  if (dropdown) dropdown.value = String(size);

  // 本地音乐页：走 AJAX 分页
  if (isLocalMusicPageActive()) {
    // 更新工具栏的 pageSize 数据
    const toolbar = document.getElementById("batch-toolbar");
    if (toolbar) toolbar.dataset.pageSize = String(size);
    // 也更新 webSettings 中的值
    webSettings.webPageSize = parseInt(size, 10);
    persistWebSettingsCache();
    void loadLocalMusicPage(1, { updateHistory: true, scroll: true });
    return;
  }

  // 其他页面：完整刷新
  const url = new URL(window.location.href);
  url.searchParams.set("page_size", String(size));
  url.searchParams.set("page", "1");
  window.location.href = url.toString();
}

function parsePositiveInt(value, fallbackValue) {
  const parsed = Number.parseInt(String(value || ""), 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallbackValue;
  }
  return parsed;
}

function parseSizeToBytes(value) {
  const raw = String(value || "")
    .trim()
    .toLowerCase();
  if (!raw || raw === "-" || raw === "无效" || raw === "检测失败") return 0;
  const match = raw.match(/([\d.]+)\s*([kmgt]?i?b|[kmgt])?/i);
  if (!match) return parsePositiveInt(raw, 0);
  const number = Number.parseFloat(match[1]);
  if (!Number.isFinite(number) || number <= 0) return 0;
  const unit = String(match[2] || "b").toLowerCase();
  const multipliers = {
    b: 1,
    k: 1024,
    kb: 1024,
    kib: 1024,
    m: 1024 ** 2,
    mb: 1024 ** 2,
    mib: 1024 ** 2,
    g: 1024 ** 3,
    gb: 1024 ** 3,
    gib: 1024 ** 3,
    t: 1024 ** 4,
    tb: 1024 ** 4,
    tib: 1024 ** 4,
  };
  return Math.round(number * (multipliers[unit] || 1));
}

function parseBitrateToKbps(value) {
  const raw = String(value || "")
    .trim()
    .toLowerCase();
  if (!raw || raw === "-") return 0;
  const match = raw.match(/([\d.]+)\s*(k|kbps|kbit\/s|m|mbps|mbit\/s)?/i);
  if (!match) return parsePositiveInt(raw, 0);
  const number = Number.parseFloat(match[1]);
  if (!Number.isFinite(number) || number <= 0) return 0;
  const unit = String(match[2] || "kbps").toLowerCase();
  return unit.startsWith("m") ? Math.round(number * 1000) : Math.round(number);
}

function ensureSongSortIndexes(root = document) {
  const scope =
    root && typeof root.querySelectorAll === "function" ? root : document;
  const cards = Array.from(
    scope.matches && scope.matches(".result-list")
      ? scope.querySelectorAll(".song-card")
      : scope.querySelectorAll(".result-list .song-card"),
  );
  cards.forEach((card, index) => {
    if (!card.dataset.sortIndex) {
      card.dataset.sortIndex = String(index);
    }
    if (!card.dataset.sortDuration) {
      card.dataset.sortDuration = String(
        parsePositiveInt(card.dataset.duration, 0),
      );
    }
    if (card.dataset.sortSize) {
      card.dataset.sortSize = String(parseSizeToBytes(card.dataset.sortSize));
    }
    if (card.dataset.sortBitrate) {
      card.dataset.sortBitrate = String(
        parseBitrateToKbps(card.dataset.sortBitrate),
      );
    }
  });
}

function songSortValue(card, mode) {
  switch (mode) {
    case "quality":
      return parsePositiveInt(card.dataset.sortBitrate, 0);
    case "size":
      return parsePositiveInt(card.dataset.sortSize, 0);
    case "duration":
      return parsePositiveInt(
        card.dataset.sortDuration || card.dataset.duration,
        0,
      );
    default:
      return parsePositiveInt(card.dataset.sortIndex, 0);
  }
}

function applySongSort(root = document) {
  const scope =
    root && typeof root.querySelector === "function" ? root : document;
  const list =
    scope.matches && scope.matches(".result-list")
      ? scope
      : scope.querySelector(".result-list") ||
        document.querySelector(".result-list");
  if (!list) return;
  ensureSongSortIndexes(list);

  const cards = Array.from(list.querySelectorAll(".song-card"));
  const direction = songSortDirection === "asc" ? 1 : -1;
  cards.sort((a, b) => {
    if (songSortMode === "default") {
      return (
        parsePositiveInt(a.dataset.sortIndex, 0) -
        parsePositiveInt(b.dataset.sortIndex, 0)
      );
    }
    const delta =
      songSortValue(a, songSortMode) - songSortValue(b, songSortMode);
    if (delta !== 0) return delta * direction;
    return (
      parsePositiveInt(a.dataset.sortIndex, 0) -
      parsePositiveInt(b.dataset.sortIndex, 0)
    );
  });
  cards.forEach((card) => list.appendChild(card));
}

function syncSongSortControls(root = document) {
  const scope =
    root && typeof root.querySelector === "function" ? root : document;
  const select =
    scope.querySelector("#song-sort-select") ||
    document.getElementById("song-sort-select");
  if (select) select.value = songSortMode;

  const button =
    scope.querySelector("#song-sort-direction") ||
    document.getElementById("song-sort-direction");
  if (!button) return;
  const descending = songSortDirection !== "asc";
  button.title = descending ? "降序" : "升序";
  button.innerHTML = descending
    ? '<i class="fa-solid fa-arrow-down-wide-short"></i>'
    : '<i class="fa-solid fa-arrow-up-wide-short"></i>';
}

function setSongSortMode(mode) {
  songSortMode = ["default", "quality", "size", "duration"].includes(mode)
    ? mode
    : "default";
  syncSongSortControls();
  applySongSort();
}

function toggleSongSortDirection() {
  songSortDirection = songSortDirection === "asc" ? "desc" : "asc";
  syncSongSortControls();
  applySongSort();
}

function bindSongSortControls(root = document) {
  ensureSongSortIndexes(root);
  syncSongSortControls(root);
  applySongSort(root);
}

function isLocalMusicPageActive(root = document) {
  const scope =
    root && typeof root.querySelector === "function" ? root : document;
  return !!scope.querySelector(
    '#localMusicPageList[data-local-music-page="true"]',
  );
}

function getCurrentLocalMusicPage() {
  const bar = document.getElementById("localMusicPagePagination");
  if (bar) {
    return parsePositiveInt(bar.dataset.currentPage, 1);
  }
  try {
    return parsePositiveInt(
      new URL(window.location.href).searchParams.get("page"),
      1,
    );
  } catch (_) {
    return 1;
  }
}

function getLocalMusicPageSize() {
  return Math.min(
    parsePositiveInt(webSettings.webPageSize, DEFAULT_WEB_PAGE_SIZE),
    200,
  );
}

let queuedLocalMusicPageLoad = null;

function buildLocalMusicPageURL(params) {
  const apiRoot = String(window.API_ROOT || "").replace(/\/+$/, "");
  const url = new URL(`${apiRoot}/local_music`, window.location.origin);
  url.search = params.toString();
  return url.toString();
}

function shouldRetryLocalMusicPageRequest(error) {
  if (!error) return false;
  const message = String(error.message || "");
  return error.name === "TypeError" || /failed to fetch|network/i.test(message);
}

async function fetchLocalMusicPagePayload(params) {
  const requestURL = buildLocalMusicPageURL(params);
  let lastError = null;

  for (let attempt = 0; attempt < 2; attempt++) {
    try {
      const response = await fetch(requestURL, {
        headers: { Accept: "application/json" },
        credentials: "same-origin",
        cache: "no-store",
      });
      const payload = await response.json().catch(() => null);
      if (!response.ok || !payload || payload.error) {
        throw new Error((payload && payload.error) || "加载本地音乐失败");
      }
      return payload;
    } catch (error) {
      lastError = error;
      if (attempt === 0 && shouldRetryLocalMusicPageRequest(error)) {
        await new Promise((resolve) => setTimeout(resolve, 180));
        continue;
      }
    }
  }

  if (shouldRetryLocalMusicPageRequest(lastError)) {
    console.warn("Local music page request failed", lastError);
    throw new Error("本地音乐列表连接失败，请稍后重试");
  }
  throw lastError || new Error("加载本地音乐失败");
}

function setLocalMusicPageHint(message, isError = false) {
  const hint = document.getElementById("localMusicPageHint");
  if (!hint) return;
  hint.textContent = message || "";
  hint.classList.toggle("error", !!isError);
  hint.style.display = message ? "block" : "none";
}

function normalizeSongExtra(extra) {
  if (!extra) return {};
  if (typeof extra === "object" && !Array.isArray(extra)) return extra;
  if (typeof extra === "string") {
    try {
      const parsed = JSON.parse(extra);
      if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
        return parsed;
      }
    } catch (_) {}
  }
  return {};
}

function serializeSongExtra(extra) {
  return JSON.stringify(normalizeSongExtra(extra));
}

function localMusicSongFromTrack(track) {
  const extra = normalizeSongExtra(track?.extra);
  return {
    id: String(track?.id || ""),
    source: String(track?.source || LOCAL_MUSIC_SOURCE),
    name: String(track?.name || track?.filename || "未命名音乐"),
    artist: String(track?.artist || "未知歌手"),
    album: String(track?.album || ""),
    cover: String(track?.cover || ""),
    duration: parsePositiveInt(track?.duration, 0),
    extra,
  };
}

function renderLocalMusicPageCard(track) {
  const song = localMusicSongFromTrack(track);
  const extraJSON = serializeSongExtra(song.extra);
  const album = song.album || "";
  const cover = song.cover || "";
  const coverHTML = cover
    ? `<img src="${escapeHTML(cover)}" alt="${escapeHTML(song.name)}" loading="lazy" onerror="this.src='https://via.placeholder.com/150?text=Music'">`
    : `<div style="width:100%;height:100%;display:flex;align-items:center;justify-content:center;color:#ccc;font-size:24px;">♪</div><img src="https://via.placeholder.com/150?text=Music" style="display:none;">`;

  const lyricButton =
    song.extra && song.extra.lyric
      ? `<a href="${escapeHTML(lyricURLsForSong({ ...song, extra: extraJSON }).download)}" id="lrc-${escapeHTML(song.id)}" class="btn-circle btn-dl btn-lyric" title="下载歌词" target="_blank"><i class="fa-solid fa-file-lines"></i></a>`
      : "";
  const coverButton = cover
    ? `<a href="${escapeHTML(buildCoverDownloadURL({ ...song, extra: extraJSON }))}" class="btn-circle btn-dl btn-cover" title="下载封面" target="_blank"><i class="fa-regular fa-image"></i></a>`
    : "";

  return `
        <li class="song-card"
            data-id="${escapeHTML(song.id)}"
            data-source="${escapeHTML(song.source)}"
            data-album-id=""
            data-album="${escapeHTML(album)}"
            data-duration="${song.duration}"
            data-name="${escapeHTML(song.name)}"
            data-artist="${escapeHTML(song.artist)}"
            data-cover="${escapeHTML(cover)}"
            data-sort-size="${parsePositiveInt(track?.size, 0)}"
            data-sort-bitrate="${parsePositiveInt(song.extra?.bitrate, 0)}"
            data-extra='${escapeHTML(extraJSON)}'>
            <div class="checkbox-wrapper">
                <input type="checkbox" class="song-checkbox" onclick="event.stopPropagation(); updateBatchToolbar();">
            </div>
            <div class="cover-wrapper">${coverHTML}</div>
            <div class="song-info">
                <h3>${escapeHTML(song.name)}</h3>
                <div class="artist-line">${renderArtistLineHTML(song)}</div>
                <div class="tags">
                    <span class="tag tag-local">本地</span>
                    <span class="tag tag-duration">${formatDuration(song.duration)}</span>
                    <span class="tag tag-loading" id="size-${escapeHTML(song.id)}"><i class="fa fa-spinner fa-spin"></i></span>
                    <span class="tag tag-loading" id="bitrate-${escapeHTML(song.id)}"><i class="fa fa-circle-notch fa-spin"></i></span>
                </div>
            </div>
            <div class="actions">
                <button type="button" class="btn-circle btn-play" title="播放" onclick="playAllAndJumpTo(this)">
                    <i class="fa-solid fa-play"></i>
                </button>
                <button type="button" class="btn-circle btn-fav" title="收藏到自制歌单" onclick="openAddToCollectionModal(this)">
                    <i class="fa-regular fa-heart"></i>
                </button>
                ${lyricButton}
                ${coverButton}
                <button type="button" class="btn-circle btn-delete-local" title="删除本地音乐" onclick="deleteLocalMusicFromButton(this)">
                    <i class="fa-solid fa-trash"></i>
                </button>
            </div>
        </li>
    `;
}

function ensureLocalMusicPaginationBar() {
  let bar = document.getElementById("localMusicPagePagination");
  if (bar) return bar;
  const list = document.getElementById("localMusicPageList");
  if (!list) return null;
  bar = document.createElement("div");
  bar.id = "localMusicPagePagination";
  bar.className = "pagination-bar";
  bar.dataset.shortcutHint = "PgUp / PgDn";
  bar.title = "Shortcut: PgUp / PgDn";
  list.insertAdjacentElement("afterend", bar);
  return bar;
}

function renderLocalMusicPagePagination(page, totalPages) {
  const bar = ensureLocalMusicPaginationBar();
  if (!bar) return;
  if (totalPages <= 1) {
    bar.style.display = "none";
    bar.dataset.currentPage = String(page);
    bar.dataset.totalPages = String(totalPages);
    return;
  }

  bar.style.display = "flex";
  bar.dataset.currentPage = String(page);
  bar.dataset.totalPages = String(totalPages);
  bar.innerHTML = `
        <button type="button" class="ctrl-btn primary" onclick="goToPage(${page - 1})" ${page <= 1 ? "disabled" : ""}>
            <i class="fa-solid fa-chevron-left"></i> 上一页
        </button>
        <span class="pagination-text">第 ${page} / ${totalPages} 页</span>
        <span class="pagination-shortcut-hint">PgUp / PgDn</span>
        <button type="button" class="ctrl-btn primary" onclick="goToPage(${page + 1})" ${page >= totalPages ? "disabled" : ""}>
            下一页 <i class="fa-solid fa-chevron-right"></i>
        </button>
    `;
}

function updateLocalMusicPageURL(page, replace = false) {
  const url = new URL(window.location.href);
  url.searchParams.set("page", String(page));
  if (replace) {
    window.history.replaceState(null, "", url.toString());
  } else {
    window.history.pushState(null, "", url.toString());
  }
}

async function loadLocalMusicPage(page = 1, options = {}) {
  const list = document.getElementById("localMusicPageList");
  if (!list) return false;
  if (list.dataset.loading === "1") {
    queuedLocalMusicPageLoad = { page, options: { ...options } };
    return false;
  }

  const targetPage = Math.max(1, parsePositiveInt(page, 1));
  const pageSize = getLocalMusicPageSize();
  const offset = (targetPage - 1) * pageSize;
  const params = new URLSearchParams({
    offset: String(offset),
    limit: String(pageSize),
  });
  if (options.force) {
    params.set("refresh", "1");
  }

  list.dataset.loading = "1";
  setLocalMusicPageHint("正在加载本地音乐...");
  try {
    const payload = await fetchLocalMusicPagePayload(params);

    const total = parsePositiveInt(payload.total, 0);
    const totalPages = Math.max(1, Math.ceil(total / pageSize));
    if (total > 0 && targetPage > totalPages) {
      list.dataset.loading = "0";
      return loadLocalMusicPage(totalPages, {
        ...options,
        updateHistory: true,
        replaceHistory: true,
      });
    }

    const totalEl = document.getElementById("localMusicPageTotal");
    if (totalEl) totalEl.textContent = String(total);

    const toolbar = document.getElementById("batch-toolbar");
    if (toolbar && toolbar.dataset.localMusic === "true") {
      toolbar.dataset.currentPage = String(targetPage);
      toolbar.dataset.totalPages = String(totalPages);
      toolbar.dataset.totalCount = String(total);
      toolbar.dataset.pageSize = String(pageSize);
    }

    const tracks = Array.isArray(payload.tracks) ? payload.tracks : [];
    list.innerHTML = tracks.map(renderLocalMusicPageCard).join("");

    if (!payload.exists) {
      setLocalMusicPageHint("下载目录还不存在。上传音乐后会自动创建该目录。");
    } else if (total === 0) {
      setLocalMusicPageHint(
        "下载目录里还没有支持的音频文件，可上传 mp3、flac、m4a、ogg、wav、wma、aac。",
      );
    } else if (payload.refreshing) {
      setLocalMusicPageHint("正在后台刷新本地音乐列表，当前显示上次扫描结果。");
    } else {
      setLocalMusicPageHint("");
    }

    renderLocalMusicPagePagination(targetPage, totalPages);
    updateFloatPageNav();
    refreshDownloadLinks(list);
    bindSongSortControls(list);
    bindSongCardCovers(list);
    updateBatchToolbar();
    highlightCard(currentPlayingId);
    syncAllPlayButtons();
    syncMediaSession();

    if (options.updateHistory) {
      updateLocalMusicPageURL(targetPage, !!options.replaceHistory);
    }
    if (options.scroll) {
      window.scrollTo({ top: 0, behavior: "auto" });
    }
    return true;
  } catch (error) {
    setLocalMusicPageHint(error.message || "加载本地音乐失败", true);
    return false;
  } finally {
    list.dataset.loading = "0";
    const queued = queuedLocalMusicPageLoad;
    queuedLocalMusicPageLoad = null;
    if (queued && list.isConnected) {
      setTimeout(() => {
        void loadLocalMusicPage(queued.page, queued.options);
      }, 0);
    }
  }
}

function initializeLocalMusicPage(root = document) {
  if (!isLocalMusicPageActive(root)) return;
  const list = document.getElementById("localMusicPageList");
  if (!list || list.dataset.initialized === "1") return;
  list.dataset.initialized = "1";
  loadLocalMusicPage(getCurrentLocalMusicPage(), {
    updateHistory: false,
  });
}

async function checkDuplicateSongs() {
  // 显示 loading 弹窗
  showDuplicateModal(null, true);
  try {
    const resp = await fetch(API_ROOT + "/local_music/duplicates");
    if (!resp.ok) throw new Error("API error");
    const data = await resp.json();
    showDuplicateModal(data.groups || [], false);
  } catch (_) {
    showDuplicateModal([], false);
  }
}

async function refreshLocalMusicPageAfterMutation() {
  if (isLocalMusicPageActive()) {
    return loadLocalMusicPage(getCurrentLocalMusicPage(), {
      updateHistory: true,
      replaceHistory: true,
      scroll: false,
    });
  }
  return refreshCurrentPageContent({ scroll: false });
}

function showDuplicateModal(groups, loading) {
  // 移除旧弹窗
  const old = document.getElementById("duplicate-modal-overlay");
  if (old) old.remove();

  const overlay = document.createElement("div");
  overlay.id = "duplicate-modal-overlay";
  overlay.style.cssText =
    "position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,0.5);z-index:20000;display:flex;align-items:center;justify-content:center;padding:20px;";
  overlay.onclick = (e) => {
    if (e.target === overlay) overlay.remove();
  };

  const modal = document.createElement("div");
  modal.style.cssText =
    "background:#fff;border-radius:16px;width:100%;max-width:640px;max-height:80vh;display:flex;flex-direction:column;box-shadow:0 20px 60px rgba(0,0,0,0.3);";

  // Header
  const header = document.createElement("div");
  header.style.cssText =
    "display:flex;align-items:center;padding:20px 24px 16px;border-bottom:1px solid #eee;";
  header.innerHTML =
    '<h3 style="margin:0;font-size:17px;color:#c45500;"><i class="fa-solid fa-triangle-exclamation"></i> 重复歌曲检测</h3>';
  const closeBtn = document.createElement("button");
  closeBtn.type = "button";
  closeBtn.textContent = "✕";
  closeBtn.style.cssText =
    "margin-left:auto;font-size:18px;background:none;border:none;cursor:pointer;color:#999;padding:4px 8px;";
  closeBtn.onclick = () => overlay.remove();
  header.appendChild(closeBtn);
  modal.appendChild(header);

  // Body
  const body = document.createElement("div");
  body.style.cssText = "overflow-y:auto;padding:16px 24px;flex:1;";

  if (loading) {
    body.innerHTML =
      '<div style="text-align:center;padding:40px;color:#999;"><i class="fa-solid fa-spinner fa-spin fa-2x"></i><p style="margin-top:12px;">正在扫描...</p></div>';
  } else if (!groups || groups.length === 0) {
    body.innerHTML =
      '<div style="text-align:center;padding:40px;color:#10b981;"><i class="fa-solid fa-circle-check fa-2x"></i><p style="margin-top:12px;">未发现重复歌曲 🎉</p></div>';
  } else {
    groups.forEach((g, gi) => {
      const gp = document.createElement("div");
      gp.style.cssText =
        "margin-bottom:14px;padding:12px;background:#fff8f0;border:1px solid #f0d5a0;border-radius:8px;";
      gp.innerHTML = `<div style="font-weight:600;margin-bottom:8px;">${gi + 1}. ${escapeHTML(g.name)} — ${escapeHTML(g.artist)}</div>`;

      g.songs.forEach((s) => {
        const sizeStr = s.size ? (s.size / 1048576).toFixed(1) + "MB" : "?";
        const durStr = s.duration
          ? Math.floor(s.duration / 60) +
            ":" +
            String(s.duration % 60).padStart(2, "0")
          : "?";
        const row = document.createElement("div");
        row.style.cssText =
          "display:flex;align-items:center;gap:8px;font-size:12px;padding:4px 8px;background:#fff;border-radius:4px;margin-bottom:3px;";
        row.innerHTML = `<span style="flex:1;color:#555;">${s.ext || "?"} · ${sizeStr} · ${durStr}</span>`;

        const playBtn = document.createElement("button");
        playBtn.type = "button";
        playBtn.style.cssText =
          "font-size:11px;padding:2px 8px;background:#10b981;color:#fff;border:none;border-radius:4px;cursor:pointer;";
        playBtn.textContent = "播放";
        playBtn.onclick = () => {
          ap.list.clear();
          ap.list.add([
            {
              name: s.name,
              artist: s.artist,
              url: buildStreamURL(s.id, "local", s.name, s.artist, "", "", ""),
              cover: "",
              lrc: "",
              theme: "#10b981",
              custom_id: s.id,
              source: "local",
              duration: s.duration || 0,
            },
          ]);
          ap.play();
        };

        const delBtn = document.createElement("button");
        delBtn.type = "button";
        delBtn.style.cssText =
          "font-size:11px;padding:2px 8px;background:#e53e3e;color:#fff;border:none;border-radius:4px;cursor:pointer;";
        delBtn.textContent = "删除";
        delBtn.onclick = async () => {
          if (!confirm(`确定删除 "${s.name}"?`)) return;
          const originalHTML = delBtn.innerHTML;
          delBtn.disabled = true;
          delBtn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i>';
          try {
            await deleteLocalMusic(s.id);
            stopDeletedLocalMusicPlayback(new Set([s.id]));
            await refreshLocalMusicPageAfterMutation();
            await checkDuplicateSongs();
          } catch (error) {
            alert(error.message || "删除失败");
          } finally {
            if (delBtn.isConnected) {
              delBtn.innerHTML = originalHTML;
              delBtn.disabled = false;
            }
          }
        };

        row.appendChild(playBtn);
        row.appendChild(delBtn);
        gp.appendChild(row);
      });
      body.appendChild(gp);
    });
  }

  modal.appendChild(body);
  overlay.appendChild(modal);
  document.body.appendChild(overlay);
}

function songFromCard(card) {
  if (!card) return null;
  const ds = card.dataset;
  if (!ds.id || !ds.source) return null;

  let coverUrl = ds.cover || "";
  const imgEl = card.querySelector(".cover-wrapper img");
  if (imgEl && imgEl.src) {
    coverUrl = imgEl.src;
  }

  return {
    id: ds.id,
    source: ds.source,
    name: ds.name || "",
    artist: ds.artist || "",
    album: ds.album || "",
    duration: parsePositiveInt(ds.duration, 0),
    cover: coverUrl,
    extra: ds.extra || "",
  };
}

function inspectSong(card) {
  if (!card) return;
  const id = card.dataset.id;
  const source = card.dataset.source;
  const duration = card.dataset.duration;
  const extra = card.dataset.extra || "";

  const params = new URLSearchParams({
    id: String(id || ""),
    source: String(source || ""),
    duration: String(duration || ""),
  });
  if (extra !== "" && extra !== "{}" && extra !== "null") {
    params.set("extra", extra);
  }

  fetch(`${API_ROOT}/inspect?${params.toString()}`)
    .then((r) => r.json())
    .then((data) => {
      if (data && data.song && typeof data.song === "object") {
        updateCardWithSong(card, data.song, { inspect: false });
      }

      const currentId = card.dataset.id || id;
      const sizeTag =
        document.getElementById(`size-${currentId}`) ||
        card.querySelector('[id^="size-"]');
      const bitrateTag =
        document.getElementById(`bitrate-${currentId}`) ||
        card.querySelector('[id^="bitrate-"]');

      if (data.valid) {
        if (sizeTag) {
          sizeTag.textContent = data.size;
          sizeTag.className = "tag tag-success";
        }
        if (bitrateTag) {
          bitrateTag.textContent = data.bitrate;
          bitrateTag.className = "tag";
        }
        card.dataset.sortSize = String(parseSizeToBytes(data.size));
        card.dataset.sortBitrate = String(parseBitrateToKbps(data.bitrate));
        if (data.duration) {
          card.dataset.sortDuration = String(
            parsePositiveInt(
              data.duration,
              parsePositiveInt(card.dataset.duration, 0),
            ),
          );
        }
      } else {
        if (sizeTag) {
          sizeTag.textContent = "无效";
          sizeTag.className = "tag tag-fail";
        }
        if (bitrateTag) {
          bitrateTag.textContent = "-";
          bitrateTag.className = "tag";
        }
        card.dataset.sortSize = "0";
        card.dataset.sortBitrate = "0";
      }
      if (songSortMode !== "default") {
        applySongSort();
      }
    })
    .catch(() => {
      const el = document.getElementById(`size-${id}`);
      if (el) el.textContent = "检测失败";
    })
    .finally(() => {
      delete card.dataset.inspectPending;
      if (document.querySelector(".tag-fail")) {
        scheduleAutoSwitchInvalidSources();
      }
    });
}

function queueInspectSong(card, delay = INSPECT_REQUEST_DELAY_MS) {
  if (!card) return;
  card.dataset.inspectPending = "1";
  window.setTimeout(() => inspectSong(card), delay);
}

const QR_LOGIN_SOURCE_LABELS = {
  netease: "网易云音乐",
  qq: "QQ音乐",
  qq_wx: "QQ音乐微信",
  kugou: "酷狗音乐",
  bilibili: "Bilibili",
  soda: "汽水音乐",
};
const QR_LOGIN_COOKIE_SOURCES = {
  qq_wx: "qq",
};
const QR_LOGIN_POLL_INTERVAL_MS = {
  soda: 2000,
};

let qrLoginState = {
  source: "",
  key: "",
  baseKey: "",
  pollTimer: 0,
  pollBusy: false,
  smsBusy: false,
  sms: {
    encryptUID: "",
    verifyParams: "",
    codeSent: false,
    mode: "",
    upSMSMobile: "",
    upSMSContent: "",
  },
};

function qrLoginSourceLabel(source) {
  return QR_LOGIN_SOURCE_LABELS[source] || source;
}

function qrLoginCookieSource(source) {
  return QR_LOGIN_COOKIE_SOURCES[source] || source;
}

function clearQRLoginPoll() {
  if (qrLoginState.pollTimer) {
    window.clearInterval(qrLoginState.pollTimer);
    qrLoginState.pollTimer = 0;
  }
  qrLoginState.pollBusy = false;
}

function setQRLoginStatus(message, type = "") {
  const el = document.getElementById("qrLoginStatus");
  if (!el) return;
  el.textContent = message || "";
  el.className = "qr-login-status" + (type ? ` ${type}` : "");
}

function setQRLoginLoading(show, message = "正在生成二维码...") {
  const el = document.getElementById("qrLoginLoading");
  if (!el) return;
  el.textContent = message;
  el.style.display = show ? "flex" : "none";
}

function resetQRLoginSMSView() {
  const panel = document.getElementById("qrLoginSMSPanel");
  const input = document.getElementById("qrLoginSMSCode");
  const sendBtn = document.getElementById("qrLoginSendSMSBtn");
  const validateBtn = document.getElementById("qrLoginValidateSMSBtn");
  if (panel) panel.style.display = "none";
  if (input) {
    if (!input.dataset.qrSmsBound) {
      input.addEventListener("keydown", (event) => {
        if (event.key === "Enter") validateSodaSMSCode();
      });
      input.dataset.qrSmsBound = "true";
    }
    input.value = "";
    input.disabled = false;
    input.style.display = "";
  }
  if (sendBtn) {
    sendBtn.disabled = false;
    sendBtn.textContent = "发送验证码";
  }
  if (validateBtn) {
    validateBtn.disabled = false;
    validateBtn.style.display = "";
    validateBtn.textContent = "确认登录";
  }
}

function qrLoginResultExtra(result) {
  const extra = result?.extra || result?.Extra || {};
  return extra && typeof extra === "object" ? extra : {};
}

function qrLoginExtraFlag(extra, key) {
  return String(extra?.[key] || "").toLowerCase() === "true";
}

function resetQRLoginView(source) {
  const title = document.getElementById("qrLoginTitle");
  const canvas = document.getElementById("qrLoginCanvas");
  const image = document.getElementById("qrLoginImage");
  if (title) title.textContent = `${qrLoginSourceLabel(source)}扫码登录`;
  if (canvas) canvas.style.display = "none";
  if (image) {
    image.style.display = "none";
    image.removeAttribute("src");
  }
  setQRLoginLoading(true);
  setQRLoginStatus(
    source === "qq_wx" ? "请使用微信扫码登录" : "请使用对应音乐 App 扫码登录",
  );
  resetQRLoginSMSView();
}

function closeQRLoginModal() {
  clearQRLoginPoll();
  resetQRLoginSMSView();
  const modal = document.getElementById("qrLoginModal");
  if (modal) modal.style.display = "none";
}

function restartQRLogin() {
  if (!qrLoginState.source) return;
  startQRLogin(qrLoginState.source);
}

function startQRLogin(source) {
  source = String(source || "").trim();
  if (!QR_LOGIN_SOURCE_LABELS[source]) {
    showToast(
      "扫码登录不可用",
      `${source || "当前平台"}暂不支持扫码登录`,
      "warning",
    );
    return;
  }

  clearQRLoginPoll();
  qrLoginState = {
    source,
    key: "",
    baseKey: "",
    pollTimer: 0,
    pollBusy: false,
    smsBusy: false,
    sms: {
      encryptUID: "",
      verifyParams: "",
      codeSent: false,
      mode: "",
      upSMSMobile: "",
      upSMSContent: "",
    },
  };
  const modal = document.getElementById("qrLoginModal");
  if (modal) modal.style.display = "flex";
  resetQRLoginView(source);

  fetch(`${API_ROOT}/qr_login/${encodeURIComponent(source)}`, {
    method: "POST",
  })
    .then(async (response) => {
      const data = await response.json().catch(() => null);
      if (handleConfigAuthResponse(response, data)) {
        return null;
      }
      if (!response.ok || !data) {
        throw new Error((data && data.error) || "二维码创建失败");
      }
      return data;
    })
    .then((session) => {
      if (!session) return;
      qrLoginState.key = String(session.key || "");
      qrLoginState.baseKey = qrLoginState.key;
      renderQRLoginSession(session);
      setQRLoginStatus(
        source === "qq_wx"
          ? "二维码已生成，请打开微信扫码"
          : "二维码已生成，请打开 App 扫码",
      );
      const pollInterval = QR_LOGIN_POLL_INTERVAL_MS[source] || 2200;
      pollQRLogin();
      qrLoginState.pollTimer = window.setInterval(pollQRLogin, pollInterval);
    })
    .catch((error) => {
      setQRLoginLoading(false);
      setQRLoginStatus(error.message || "二维码创建失败", "error");
      showToast("二维码创建失败", error.message || "请稍后重试", "error");
    });
}

function renderQRLoginSession(session) {
  const canvas = document.getElementById("qrLoginCanvas");
  const image = document.getElementById("qrLoginImage");
  const imageURL = String(session.image_url || session.imageURL || "").trim();
  const loginURL = String(session.url || session.URL || "").trim();

  if (imageURL && image) {
    image.src = imageURL;
    image.style.display = "block";
    if (canvas) canvas.style.display = "none";
    setQRLoginLoading(false);
    return;
  }

  if (!loginURL || !canvas) {
    throw new Error("二维码内容为空");
  }

  drawQRCodeToCanvas(loginURL, canvas);
  canvas.style.display = "block";
  if (image) image.style.display = "none";
  setQRLoginLoading(false);
}

function cookieFromQRLoginResult(result) {
  const direct = String(result?.cookie || "").trim();
  if (direct) return direct;
  const cookies = result?.cookies;
  if (!cookies || typeof cookies !== "object") return "";
  return Object.keys(cookies)
    .filter(
      (key) => String(key || "").trim() && String(cookies[key] || "").trim(),
    )
    .sort()
    .map((key) => `${key}=${cookies[key]}`)
    .join("; ");
}

function handleQRLoginSuccess(result) {
  clearQRLoginPoll();
  const cookie = cookieFromQRLoginResult(result);
  const input = document.getElementById(
    `cookie-${qrLoginCookieSource(qrLoginState.source)}`,
  );
  if (input && cookie) input.value = cookie;
  setQRLoginStatus("登录成功，Cookie 已保存", "success");
  showToast(
    "扫码登录成功",
    `${qrLoginSourceLabel(qrLoginState.source)} Cookie 已保存`,
    "success",
  );
  window.setTimeout(closeQRLoginModal, 900);
}

function showSodaSMSLogin(result) {
  clearQRLoginPoll();
  const extra = qrLoginResultExtra(result);
  const upSMSMobile = String(extra.up_sms_mobile || "").trim();
  const upSMSContent = String(extra.up_sms_content || "").trim();
  const smsMode =
    qrLoginExtraFlag(extra, "need_user_sms") ||
    String(extra.sms_mode || "").toLowerCase() === "up"
      ? "up"
      : "";
  qrLoginState.sms = {
    encryptUID: String(extra.encrypt_uid || qrLoginState.sms.encryptUID || ""),
    verifyParams: String(
      extra.verify_params || qrLoginState.sms.verifyParams || "",
    ),
    codeSent:
      qrLoginExtraFlag(extra, "need_sms_code") || qrLoginState.sms.codeSent,
    mode: smsMode || qrLoginState.sms.mode || "",
    upSMSMobile: upSMSMobile || qrLoginState.sms.upSMSMobile || "",
    upSMSContent: upSMSContent || qrLoginState.sms.upSMSContent || "",
  };
  const panel = document.getElementById("qrLoginSMSPanel");
  const input = document.getElementById("qrLoginSMSCode");
  const sendBtn = document.getElementById("qrLoginSendSMSBtn");
  const validateBtn = document.getElementById("qrLoginValidateSMSBtn");
  if (panel) panel.style.display = "block";
  if (sendBtn) sendBtn.disabled = false;
  if (validateBtn) validateBtn.disabled = false;

  const mobile = String(extra.mobile || "").trim();
  if (qrLoginState.sms.mode === "up") {
    if (input) {
      input.value = "";
      input.disabled = true;
      input.style.display = "none";
    }
    if (validateBtn) validateBtn.style.display = "none";
    if (sendBtn) sendBtn.textContent = "我已发送";
    const target = qrLoginState.sms.upSMSMobile || "指定号码";
    const content = qrLoginState.sms.upSMSContent || "指定内容";
    const from = mobile ? `请使用绑定手机号 ${mobile} ` : "请使用绑定手机号 ";
    setQRLoginStatus(
      `${from}发送短信 ${content} 到 ${target}，发送后点击“我已发送”`,
      "warning",
    );
    return;
  }
  if (qrLoginState.sms.codeSent) {
    setQRLoginStatus(
      mobile
        ? `验证码已发送至 ${mobile}，请输入后确认`
        : "验证码已发送，请输入后确认",
      "warning",
    );
    if (input) input.focus();
    return;
  }

  setQRLoginStatus(
    mobile
      ? `扫码成功，请点击发送验证码到 ${mobile}`
      : "扫码成功，请点击发送验证码",
    "warning",
  );
}

function sodaSMSActionKey(action, code = "") {
  const token = String(qrLoginState.baseKey || qrLoginState.key || "").trim();
  const encryptUID = String(qrLoginState.sms.encryptUID || "").trim();
  const verifyParams = String(qrLoginState.sms.verifyParams || "").trim();
  if (!token || !encryptUID) return "";
  const parts = [token, action, encryptUID, verifyParams];
  if (action === "validate") parts.push(code);
  return parts.join("|");
}

function fetchQRLoginCheck(source, key) {
  return fetch(
    `${API_ROOT}/qr_login/${encodeURIComponent(source)}?key=${encodeURIComponent(key)}`,
  ).then(async (response) => {
    const data = await response.json().catch(() => null);
    if (handleConfigAuthResponse(response, data)) {
      return null;
    }
    if (!response.ok || !data) {
      throw new Error((data && data.error) || "登录状态检查失败");
    }
    return data;
  });
}

function sendSodaSMSCode() {
  if (qrLoginState.source !== "soda" || qrLoginState.smsBusy) return;
  const isUpSMS = qrLoginState.sms.mode === "up";
  const key = sodaSMSActionKey(isUpSMS ? "up_sms" : "send_code");
  if (!key) {
    setQRLoginStatus("缺少短信验证参数，请刷新二维码重试", "error");
    return;
  }

  qrLoginState.smsBusy = true;
  const sendBtn = document.getElementById("qrLoginSendSMSBtn");
  if (sendBtn) {
    sendBtn.disabled = true;
    sendBtn.textContent = isUpSMS ? "确认中..." : "发送中...";
  }
  setQRLoginStatus(isUpSMS ? "正在确认上行短信..." : "正在发送验证码...");

  fetchQRLoginCheck("soda", key)
    .then((result) => {
      if (!result) return;
      const status = String(result.status || "");
      if (status === "success") {
        handleQRLoginSuccess(result);
        return;
      }
      if (status === "failed") {
        setQRLoginStatus(
          String(result.message || "").trim() ||
            (isUpSMS ? "上行短信确认失败" : "验证码发送失败"),
          "error",
        );
        return;
      }
      showSodaSMSLogin(result);
    })
    .catch((error) => {
      setQRLoginStatus(
        error.message || (isUpSMS ? "上行短信确认失败" : "验证码发送失败"),
        "error",
      );
    })
    .finally(() => {
      qrLoginState.smsBusy = false;
      if (sendBtn) {
        sendBtn.disabled = false;
        sendBtn.textContent = isUpSMS ? "我已发送" : "重新发送";
      }
    });
}

function validateSodaSMSCode() {
  if (qrLoginState.source !== "soda" || qrLoginState.smsBusy) return;
  const input = document.getElementById("qrLoginSMSCode");
  const code = String(input?.value || "").trim();
  if (!code) {
    setQRLoginStatus("请输入验证码", "warning");
    if (input) input.focus();
    return;
  }
  const key = sodaSMSActionKey("validate", code);
  if (!key) {
    setQRLoginStatus("缺少短信验证参数，请刷新二维码重试", "error");
    return;
  }

  qrLoginState.smsBusy = true;
  const validateBtn = document.getElementById("qrLoginValidateSMSBtn");
  if (validateBtn) {
    validateBtn.disabled = true;
    validateBtn.textContent = "验证中...";
  }
  setQRLoginStatus("正在验证...");

  fetchQRLoginCheck("soda", key)
    .then((result) => {
      if (!result) return;
      const status = String(result.status || "");
      if (status === "success") {
        handleQRLoginSuccess(result);
        return;
      }
      setQRLoginStatus(
        String(result.message || "").trim() || "验证码错误，请重试",
        status === "failed" ? "error" : "warning",
      );
    })
    .catch((error) => {
      setQRLoginStatus(error.message || "验证码验证失败", "error");
    })
    .finally(() => {
      qrLoginState.smsBusy = false;
      if (validateBtn) {
        validateBtn.disabled = false;
        validateBtn.textContent = "确认登录";
      }
    });
}

function pollQRLogin() {
  if (!qrLoginState.source || !qrLoginState.key || qrLoginState.pollBusy)
    return;
  qrLoginState.pollBusy = true;

  fetchQRLoginCheck(qrLoginState.source, qrLoginState.key)
    .then((result) => {
      if (!result) return;
      const status = String(result.status || "");
      const message = String(result.message || "").trim();
      if (status === "success") {
        handleQRLoginSuccess(result);
        return;
      }
      if (status === "scanned") {
        const extra = qrLoginResultExtra(result);
        if (
          qrLoginState.source === "soda" &&
          qrLoginExtraFlag(extra, "need_sms")
        ) {
          showSodaSMSLogin(result);
          return;
        }
        setQRLoginStatus(message || "已扫码，请在手机上确认", "warning");
        return;
      }
      if (status === "waiting") {
        setQRLoginStatus(message || "等待扫码中");
        return;
      }
      if (status === "expired") {
        clearQRLoginPoll();
        setQRLoginStatus(message || "二维码已过期，请刷新", "warning");
        return;
      }
      clearQRLoginPoll();
      setQRLoginStatus(message || "登录失败，请刷新重试", "error");
    })
    .catch((error) => {
      clearQRLoginPoll();
      setQRLoginStatus(error.message || "登录状态检查失败", "error");
    })
    .finally(() => {
      qrLoginState.pollBusy = false;
    });
}

function qrUtf8Bytes(text) {
  if (window.TextEncoder) {
    return Array.from(new TextEncoder().encode(text));
  }
  return Array.from(unescape(encodeURIComponent(text)), (ch) =>
    ch.charCodeAt(0),
  );
}

function qrAppendBits(bits, value, length) {
  for (let i = length - 1; i >= 0; i--) {
    bits.push(((value >>> i) & 1) !== 0);
  }
}

const QR_VERSION_SPECS = [
  null,
  { data: 19, ecc: 7, blocks: 1, align: [] },
  { data: 34, ecc: 10, blocks: 1, align: [6, 18] },
  { data: 55, ecc: 15, blocks: 1, align: [6, 22] },
  { data: 80, ecc: 20, blocks: 1, align: [6, 26] },
  { data: 108, ecc: 26, blocks: 1, align: [6, 30] },
  { data: 136, ecc: 18, blocks: 2, align: [6, 34] },
];

function qrPickVersion(byteLength) {
  for (let version = 1; version < QR_VERSION_SPECS.length; version++) {
    if (
      byteLength <= Math.floor((QR_VERSION_SPECS[version].data * 8 - 12) / 8)
    ) {
      return version;
    }
  }
  throw new Error("二维码内容过长，请刷新重试");
}

function qrGfMul(x, y) {
  let z = 0;
  for (let i = 7; i >= 0; i--) {
    z = ((z << 1) ^ ((z >>> 7) * 0x11d)) & 0xff;
    if (((y >>> i) & 1) !== 0) z ^= x;
  }
  return z;
}

function qrRsGenerator(degree) {
  const result = new Array(degree).fill(0);
  result[degree - 1] = 1;
  let root = 1;
  for (let i = 0; i < degree; i++) {
    for (let j = 0; j < degree; j++) {
      result[j] = qrGfMul(result[j], root);
      if (j + 1 < degree) result[j] ^= result[j + 1];
    }
    root = qrGfMul(root, 2);
  }
  return result;
}

function qrRsRemainder(data, generator) {
  const result = new Array(generator.length).fill(0);
  data.forEach((value) => {
    const factor = value ^ result.shift();
    result.push(0);
    for (let i = 0; i < result.length; i++) {
      result[i] ^= qrGfMul(generator[i], factor);
    }
  });
  return result;
}

function qrCodewords(text) {
  const bytes = qrUtf8Bytes(text);
  const version = qrPickVersion(bytes.length);
  const spec = QR_VERSION_SPECS[version];
  const bits = [];
  qrAppendBits(bits, 0x4, 4);
  qrAppendBits(bits, bytes.length, 8);
  bytes.forEach((value) => qrAppendBits(bits, value, 8));
  const maxBits = spec.data * 8;
  if (bits.length > maxBits) throw new Error("二维码内容过长");
  for (let i = 0, n = Math.min(4, maxBits - bits.length); i < n; i++)
    bits.push(false);
  while (bits.length % 8 !== 0) bits.push(false);

  const data = [];
  for (let i = 0; i < bits.length; i += 8) {
    let value = 0;
    for (let j = 0; j < 8; j++) value = (value << 1) | (bits[i + j] ? 1 : 0);
    data.push(value);
  }
  for (let pad = 0xec; data.length < spec.data; pad ^= 0xfd) {
    data.push(pad);
  }

  const generator = qrRsGenerator(spec.ecc);
  const blockLen = spec.data / spec.blocks;
  const blocks = [];
  for (let i = 0; i < spec.blocks; i++) {
    const block = data.slice(i * blockLen, (i + 1) * blockLen);
    blocks.push({ data: block, ecc: qrRsRemainder(block, generator) });
  }

  const result = [];
  for (let i = 0; i < blockLen; i++) {
    blocks.forEach((block) => result.push(block.data[i]));
  }
  for (let i = 0; i < spec.ecc; i++) {
    blocks.forEach((block) => result.push(block.ecc[i]));
  }
  return { version, codewords: result };
}

function qrSetModule(qr, x, y, dark, fixed = false) {
  if (x < 0 || y < 0 || x >= qr.size || y >= qr.size) return;
  qr.modules[y][x] = !!dark;
  if (fixed) qr.fixed[y][x] = true;
}

function qrAddFinder(qr, cx, cy) {
  for (let dy = -4; dy <= 4; dy++) {
    for (let dx = -4; dx <= 4; dx++) {
      const dist = Math.max(Math.abs(dx), Math.abs(dy));
      qrSetModule(qr, cx + dx, cy + dy, dist !== 2 && dist !== 4, true);
    }
  }
}

function qrAddAlignment(qr, cx, cy) {
  for (let dy = -2; dy <= 2; dy++) {
    for (let dx = -2; dx <= 2; dx++) {
      const dist = Math.max(Math.abs(dx), Math.abs(dy));
      qrSetModule(qr, cx + dx, cy + dy, dist === 0 || dist === 2, true);
    }
  }
}

function qrFormatBits(mask) {
  const data = (1 << 3) | mask;
  let bits = data << 10;
  for (let i = 14; i >= 10; i--) {
    if (((bits >>> i) & 1) !== 0) bits ^= 0x537 << (i - 10);
  }
  return ((data << 10) | (bits & 0x3ff)) ^ 0x5412;
}

function qrPlaceFormat(qr, mask) {
  const bits = qrFormatBits(mask);
  const n = qr.size;
  for (let i = 0; i <= 5; i++)
    qrSetModule(qr, 8, i, ((bits >>> i) & 1) !== 0, true);
  qrSetModule(qr, 8, 7, ((bits >>> 6) & 1) !== 0, true);
  qrSetModule(qr, 8, 8, ((bits >>> 7) & 1) !== 0, true);
  qrSetModule(qr, 7, 8, ((bits >>> 8) & 1) !== 0, true);
  for (let i = 9; i < 15; i++)
    qrSetModule(qr, 14 - i, 8, ((bits >>> i) & 1) !== 0, true);
  for (let i = 0; i < 8; i++)
    qrSetModule(qr, n - 1 - i, 8, ((bits >>> i) & 1) !== 0, true);
  for (let i = 8; i < 15; i++)
    qrSetModule(qr, 8, n - 15 + i, ((bits >>> i) & 1) !== 0, true);
  qrSetModule(qr, 8, n - 8, true, true);
}

function qrBuildMatrix(text) {
  const { version, codewords } = qrCodewords(text);
  const size = 17 + version * 4;
  const qr = {
    size,
    modules: Array.from({ length: size }, () => new Array(size).fill(false)),
    fixed: Array.from({ length: size }, () => new Array(size).fill(false)),
  };
  qrAddFinder(qr, 3, 3);
  qrAddFinder(qr, size - 4, 3);
  qrAddFinder(qr, 3, size - 4);
  for (let i = 8; i < size - 8; i++) {
    qrSetModule(qr, i, 6, i % 2 === 0, true);
    qrSetModule(qr, 6, i, i % 2 === 0, true);
  }
  QR_VERSION_SPECS[version].align.forEach((cy) => {
    QR_VERSION_SPECS[version].align.forEach((cx) => {
      if (!qr.fixed[cy][cx]) qrAddAlignment(qr, cx, cy);
    });
  });
  for (let i = 0; i < 9; i++) {
    if (i !== 6) {
      qrSetModule(qr, 8, i, false, true);
      qrSetModule(qr, i, 8, false, true);
    }
  }
  for (let i = 0; i < 8; i++) qrSetModule(qr, size - 1 - i, 8, false, true);
  for (let i = 0; i < 7; i++) qrSetModule(qr, 8, size - 1 - i, false, true);
  qrSetModule(qr, 8, size - 8, true, true);

  const bits = [];
  codewords.forEach((value) => qrAppendBits(bits, value, 8));
  let bitIndex = 0;
  let upward = true;
  for (let right = size - 1; right >= 1; right -= 2) {
    if (right === 6) right--;
    for (let vert = 0; vert < size; vert++) {
      const y = upward ? size - 1 - vert : vert;
      for (let j = 0; j < 2; j++) {
        const x = right - j;
        if (qr.fixed[y][x]) continue;
        let dark = bitIndex < bits.length ? bits[bitIndex] : false;
        bitIndex++;
        if ((x + y) % 2 === 0) dark = !dark;
        qrSetModule(qr, x, y, dark, false);
      }
    }
    upward = !upward;
  }
  qrPlaceFormat(qr, 0);
  return qr;
}

function drawQRCodeToCanvas(text, canvas) {
  const qr = qrBuildMatrix(text);
  const logicalSize = 260;
  const dpr = Math.max(1, window.devicePixelRatio || 1);
  canvas.width = logicalSize * dpr;
  canvas.height = logicalSize * dpr;
  canvas.style.width = `${logicalSize}px`;
  canvas.style.height = `${logicalSize}px`;
  const ctx = canvas.getContext("2d");
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.fillStyle = "#ffffff";
  ctx.fillRect(0, 0, logicalSize, logicalSize);
  const border = 4;
  const scale = Math.max(1, Math.floor(logicalSize / (qr.size + border * 2)));
  const offset =
    Math.floor((logicalSize - (qr.size + border * 2) * scale) / 2) +
    border * scale;
  ctx.fillStyle = "#0f172a";
  for (let y = 0; y < qr.size; y++) {
    for (let x = 0; x < qr.size; x++) {
      if (qr.modules[y][x])
        ctx.fillRect(offset + x * scale, offset + y * scale, scale, scale);
    }
  }
}

async function checkAppUpdate(options = {}) {
  const status = document.getElementById("updateCheckStatus");
  const repoURL =
    options.repoURL || webSettings.updateRepoUrl || DEFAULT_UPDATE_REPO_URL;
  // 版本检测默认与镜像并发竞速，谁先返回用谁；不绑定下载侧的 webSettings.githubProxyEnabled。
  const proxyEnabled = options.proxyEnabled ?? true;
  const proxyURL =
    options.proxyURL || webSettings.githubProxyUrl || DEFAULT_GITHUB_PROXY_URL;
  const params = new URLSearchParams({
    repo: repoURL,
    use_proxy: proxyEnabled ? "1" : "0",
    proxy: proxyURL,
  });

  if (status) {
    status.textContent = "正在检查...";
    status.className = "setting-inline-status";
  }
  try {
    const response = await fetch(
      `${API_ROOT}/app_update/check?${params.toString()}`,
    );
    const payload = await response.json().catch(() => null);
    if (!response.ok || !payload || payload.error) {
      throw new Error((payload && payload.error) || "检查更新失败");
    }
    if (status) {
      status.textContent = payload.update_available
        ? `发现新版本 v${payload.latest_version}`
        : `已是最新版本 v${payload.current_version}`;
      status.className = `setting-inline-status ${payload.update_available ? "success" : ""}`;
    }
    if (payload.update_available || options.showNoUpdate) {
      openUpdateModal(payload);
    }
    return payload;
  } catch (error) {
    if (status) {
      status.textContent = error.message || "检查更新失败";
      status.className = "setting-inline-status error";
    }
    if (!options.silent) {
      showToast("检查更新失败", error.message || "检查更新失败", "error");
    }
    return null;
  }
}

function maybeAutoCheckUpdate() {
  if (!webSettings.autoCheckUpdate) return;
  const key = "musicdl:update_checked_at";
  const now = Date.now();
  try {
    const last = Number(localStorage.getItem(key) || "0");
    if (Number.isFinite(last) && now - last < 6 * 60 * 60 * 1000) {
      return;
    }
    localStorage.setItem(key, String(now));
  } catch (_) {}
  checkAppUpdate({ silent: true });
}

function openUpdateModal(updateInfo = null) {
  const modal = document.getElementById("appUpdateModal");
  if (!modal) return;

  if (updateInfo) {
    modal.dataset.rawDownloadUrl =
      updateInfo.download_url || updateInfo.release_url || "";
    modal.dataset.downloadUrl =
      updateInfo.proxied_url ||
      updateInfo.download_url ||
      updateInfo.release_url ||
      "";
    modal.dataset.releaseUrl = updateInfo.release_url || "";
    const title = document.getElementById("appUpdateTitle");
    const summary = document.getElementById("appUpdateSummary");
    if (title) {
      title.textContent = `关于 go-music-dl v${updateInfo.current_version || "-"}`;
    }
    if (summary) {
      summary.textContent = `当前版本 v${updateInfo.current_version || "-"}，GitHub 最新版本 v${updateInfo.latest_version || "-"}`;
    }
  }

  modal.style.display = "flex";
}

async function openAboutAppModal() {
  const title = document.getElementById("appUpdateTitle");
  if (title) title.textContent = "关于 go-music-dl";
  const summary = document.getElementById("appUpdateSummary");
  if (summary) summary.textContent = "正在读取 GitHub 版本信息...";
  openUpdateModal();
  await checkAppUpdate({ showNoUpdate: true, silent: true });
}

function closeUpdateModal() {
  const modal = document.getElementById("appUpdateModal");
  if (modal) modal.style.display = "none";
}

// ==========================================
// 下载记录
// ==========================================

async function openDownloadRecordsModal() {
  const modal = document.getElementById("downloadRecordsModal");
  if (!modal) return;
  modal.style.display = "flex";

  const countEl = document.getElementById("download-records-count");
  const listEl = document.getElementById("download-records-list");
  if (countEl) countEl.textContent = "加载中...";
  if (listEl) listEl.innerHTML = "";

  try {
    const resp = await fetch(`${API_ROOT}/api/downloads/records`);
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const data = await resp.json();
    const records = data.records || [];

    if (countEl) {
      const success = records.filter(r => r.Status === "success").length;
      const skipped = records.filter(r => r.Status === "skipped").length;
      const failed = records.filter(r => r.Status === "failed").length;
      countEl.textContent = `共 ${records.length} 条  ·  成功 ${success}  ·  跳过 ${skipped}  ·  失败 ${failed}`;
    }

    if (records.length === 0) {
      if (listEl) listEl.innerHTML = '<div style="text-align:center;padding:40px 0;color:var(--text-sub);">暂无下载记录</div>';
      return;
    }

    let html = `<table class="download-records-table" style="width:100%;border-collapse:collapse;font-size:13px;">
      <thead><tr style="background:var(--bg-sub);">
        <th style="padding:8px 10px;text-align:left;border-bottom:1px solid var(--border-color);">歌曲</th>
        <th style="padding:8px 10px;text-align:left;border-bottom:1px solid var(--border-color);">歌手</th>
        <th style="padding:8px 10px;text-align:left;border-bottom:1px solid var(--border-color);">来源</th>
        <th style="padding:8px 10px;text-align:center;border-bottom:1px solid var(--border-color);">状态</th>
        <th style="padding:8px 10px;text-align:right;border-bottom:1px solid var(--border-color);">时间</th>
      </tr></thead><tbody>`;

    for (const r of records) {
      const statusIcon = r.Status === "success" ? "✅" : r.Status === "skipped" ? "⏭️" : "❌";
      const statusClass = r.Status === "failed" ? "color:#e53e3e;" : "";
      const errHint = r.Error ? ` title="${escapeHtml(r.Error)}"` : "";
      const time = r.CreatedAt ? new Date(r.CreatedAt).toLocaleString() : "";
      html += `<tr${errHint} style="border-bottom:1px solid var(--border-color);">
        <td style="padding:6px 10px;">${escapeHtml(r.Name || "")}</td>
        <td style="padding:6px 10px;">${escapeHtml(r.Artist || "")}</td>
        <td style="padding:6px 10px;">${escapeHtml(r.Source || "")}</td>
        <td style="padding:6px 10px;text-align:center;${statusClass}">${statusIcon}</td>
        <td style="padding:6px 10px;text-align:right;white-space:nowrap;color:var(--text-sub);font-size:12px;">${time}</td>
      </tr>`;
    }

    html += "</tbody></table>";
    if (listEl) listEl.innerHTML = html;
  } catch (err) {
    if (countEl) countEl.textContent = "加载失败";
    if (listEl) listEl.innerHTML = `<div style="text-align:center;padding:20px;color:#e53e3e;">加载失败: ${err.message}</div>`;
  }
}

function closeDownloadRecordsModal() {
  const modal = document.getElementById("downloadRecordsModal");
  if (modal) modal.style.display = "none";
}

async function clearDownloadRecords() {
  if (!confirm("确定清空所有下载记录？此操作不可撤销。")) return;

  try {
    const resp = await fetch(`${API_ROOT}/api/downloads/records`, { method: "DELETE" });
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const countEl = document.getElementById("download-records-count");
    const listEl = document.getElementById("download-records-list");
    if (countEl) countEl.textContent = "已清空";
    if (listEl) listEl.innerHTML = '<div style="text-align:center;padding:40px 0;color:var(--text-sub);">暂无下载记录</div>';
  } catch (err) {
    alert("清空失败: " + err.message);
  }
}

async function resetDownloadLogs() {
  if (!confirm("确定重置下载日志文件（下载记录.txt/跳过下载.txt/下载失败.txt）？")) return;
  try {
    const resp = await fetch(`${API_ROOT}/api/downloads/logs`, { method: "DELETE" });
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    alert("✅ 下载日志已重置");
  } catch (err) {
    alert("重置失败: " + err.message);
  }
}

function escapeHtml(str) {
  if (!str) return "";
  return String(str)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

// 选择下载目录文件夹
async function pickDownloadFolder() {
  try {
    if (!window.showDirectoryPicker) {
      // 降级：让用户手动选择预设或输入路径
      document.getElementById("setting-download-dir-preset").value = "__custom__";
      document.getElementById("setting-download-dir").focus();
      return;
    }
    const dirHandle = await window.showDirectoryPicker();
    const path = dirHandle.name;
    // showDirectoryPicker 不暴露完整路径，用名称回填让用户补全
    // 但大多数情况下用户知道自己的目录结构
    const input = document.getElementById("setting-download-dir");
    input.value = input.value || path;
    input.focus();
    input.setSelectionRange(input.value.length, input.value.length);
  } catch (err) {
    if (err.name !== "AbortError") {
      console.warn("文件夹选择器不可用:", err);
    }
  }
}

// ==========================================
// 导入已有曲库
// ==========================================

let importFileContent = null;

function pickImportFile() {
  document.getElementById("import-file-picker").click();
}

function onImportFilePicked(event) {
  const files = event.target.files;
  if (!files || files.length === 0) return;

  // 遍历文件夹，提取音乐文件路径
  const musicExts = [".mp3", ".m4a", ".flac", ".wav", ".ogg", ".wma", ".aac", ".ape", ".dsf"];
  const lines = [];
  for (const file of files) {
    const ext = file.name.includes(".") ? "." + file.name.split(".").pop().toLowerCase() : "";
    if (musicExts.includes(ext)) {
      lines.push(file.webkitRelativePath || file.name);
    }
  }
  importFileContent = lines.join("\n");
  const nameEl = document.getElementById("import-file-name");
  if (nameEl) nameEl.textContent = "📁 " + (files[0].webkitRelativePath?.split("/")[0] || "已选择") + " (" + lines.length + " 个文件)";
  document.getElementById("import-result").style.display = "none";
}

function openImportSongsModal() {
  document.getElementById("downloadRecordsModal").style.display = "none";
  const nameEl = document.getElementById("import-file-name");
  if (nameEl) nameEl.textContent = "未选择文件";
  importFileContent = null;
  document.getElementById("import-file-picker").value = "";
  document.getElementById("import-result").style.display = "none";
  document.getElementById("import-result").innerHTML = "";
  document.getElementById("import-btn").disabled = false;
  document.getElementById("import-btn").innerHTML = '<i class="fa-solid fa-rotate"></i> 解析';
  document.getElementById("importSongsModal").style.display = "flex";
}

async function startImportSongs() {
  if (!importFileContent) {
    alert("请先选择要导入的文件");
    return;
  }

  const btn = document.getElementById("import-btn");
  btn.disabled = true;
  btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 导入中...';

  try {
    const body = { fileContent: importFileContent };
    const resp = await fetch(`${API_ROOT}/api/downloads/import`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    });
    const data = await resp.json();

    const resultDiv = document.getElementById("import-result");
    resultDiv.style.display = "block";

    if (!resp.ok) {
      resultDiv.innerHTML = `<div style="color:#e53e3e;"><strong>导入失败:</strong> ${escapeHtml(data.error || "未知错误")}</div>`;
      return;
    }

    let html = `<div style="color:#10b981;"><strong>导入完成!</strong></div>`;
    html += `<div style="margin-top:8px;font-size:13px;">`;
    html += `总行数: <strong>${data.total}</strong> | `;
    html += `已导入: <strong style="color:#10b981;">${data.imported}</strong> | `;
    html += `已跳过: <strong style="color:var(--text-sub);">${data.skipped}</strong>`;
    html += `</div>`;

    if (data.samples && data.samples.length > 0) {
      html += `<div style="margin-top:8px;font-size:12px;color:var(--text-sub);">`;
      html += `导入示例:`;
      html += `<ul style="margin:4px 0 0 16px;">`;
      for (const s of data.samples) {
        html += `<li>${escapeHtml(s)}</li>`;
      }
      html += `</ul></div>`;
    }

    if (data.dataDir) {
      html += `<div style="margin-top:10px;padding:10px;background:#fef3c7;border-radius:8px;font-size:13px;word-break:break-all;">`;
      html += `<strong>📁 文件位置:</strong><br>`;
      html += `<code style="font-size:12px;">${escapeHtml(data.dataDir)}</code><br>`;
      html += `<span style="font-size:12px;color:#92400e;">成功解析.txt / 不能匹配.txt 已生成在此目录</span>`;
      html += `</div>`;
    }

    resultDiv.innerHTML = html;
  } catch (err) {
    const resultDiv = document.getElementById("import-result");
    resultDiv.style.display = "block";
    resultDiv.innerHTML = `<div style="color:#e53e3e;"><strong>请求失败:</strong> ${escapeHtml(err.message)}</div>`;
  } finally {
    btn.disabled = false;
    btn.innerHTML = '<i class="fa-solid fa-rotate"></i> 解析';
  }
}


function proxiedGithubURL(rawURL, proxyURL, enabled) {
  if (!enabled || !rawURL || !rawURL.startsWith("https://github.com/")) {
    return rawURL;
  }
  return `${(proxyURL || "").replace(/\/$/, "")}/${rawURL}`;
}

function desktopExternalOpenCallback() {
  try {
    if (
      typeof globalThis !== "undefined" &&
      globalThis.callback &&
      typeof globalThis.callback.musicDlOpenDownload === "function"
    ) {
      return globalThis.callback.musicDlOpenDownload;
    }
  } catch (_) {}
  return null;
}

function openClientExternalURL(url, popup = null) {
  const callback = desktopExternalOpenCallback();
  if (callback) {
    callback(url);
    if (popup && !popup.closed) {
      popup.close();
    }
    return true;
  }

  if (popup && !popup.closed) {
    try {
      popup.opener = null;
    } catch (_) {}
    popup.location.href = url;
    return true;
  }

  const opened = window.open(url, "_blank", "noopener,noreferrer");
  if (opened) {
    return true;
  }

  window.location.href = url;
  return false;
}

async function openLatestUpdatePage(target = "download") {
  const modal = document.getElementById("appUpdateModal");
  const repoURL = webSettings.updateRepoUrl || DEFAULT_UPDATE_REPO_URL;
  const proxyEnabled = !!webSettings.githubProxyEnabled;
  const proxyURL = webSettings.githubProxyUrl || DEFAULT_GITHUB_PROXY_URL;
  const callback = desktopExternalOpenCallback();
  const popup = callback ? null : window.open("about:blank", "_blank");

  let downloadURL = modal?.dataset.rawDownloadUrl || "";
  let releaseURL = modal?.dataset.releaseUrl || "";
  if (!downloadURL || !releaseURL) {
    const payload = await checkAppUpdate({
      repoURL,
      proxyEnabled,
      proxyURL,
      showNoUpdate: true,
      silent: true,
    });
    if (payload) {
      downloadURL = payload.download_url || payload.release_url || downloadURL;
      releaseURL = payload.release_url || releaseURL;
      if (modal) {
        modal.dataset.rawDownloadUrl = downloadURL;
        modal.dataset.releaseUrl = releaseURL;
      }
    }
  }

  let url =
    target === "release"
      ? releaseURL || downloadURL
      : downloadURL || releaseURL;
  if (!url) {
    if (popup && !popup.closed) {
      popup.close();
    }
    showToast("无法打开下载页", "没有可用的 GitHub 链接", "error");
    return;
  }
  url = proxiedGithubURL(url, proxyURL, proxyEnabled);
  openClientExternalURL(url, popup);
}

async function openSystemConfig() {
  const modal = document.getElementById("cookieModal");
  try {
    const [cookiesResponse, settingsResponse] = await Promise.all([
      fetch(API_ROOT + "/cookies", { headers: { Accept: "application/json" } }),
      fetch(API_ROOT + "/settings"),
    ]);
    const cookies = await cookiesResponse.json().catch(() => null);
    const settings = await settingsResponse.json().catch(() => null);
    if (handleConfigAuthResponse(cookiesResponse, cookies)) return;
    if (!cookiesResponse.ok || !settingsResponse.ok) {
      throw new Error("加载系统配置失败");
    }
    applyWebSettings(settings);
    for (const [k, v] of Object.entries(cookies || {})) {
      const el = document.getElementById(`cookie-${k}`);
      if (el) el.value = v;
    }
    setAuthFloatLoggedIn(true);
    if (modal) modal.style.display = "flex";
  } catch (error) {
    applyWebSettings(webSettings);
    showToast("系统配置加载失败", error.message || "请稍后重试", "error");
  }
}

function openCookieModal() {
  openSystemConfig();
}

async function saveCookies() {
  const webPageSizeInput = document.getElementById("setting-web-page-size");
  const cliPageSizeInput = document.getElementById("setting-cli-page-size");

  const nextSettings = normalizeWebSettings({
    embedDownload: !!document.getElementById("setting-embed-download")?.checked,
    downloadDir: document.getElementById("setting-download-dir")?.value || "",
    downloadFilenameTemplate:
      document.getElementById("setting-download-filename-template")?.value ||
      "",
    disableFloatingLyrics: !document.getElementById("setting-floating-lyrics")
      ?.checked,
    webPageSize: parsePositiveInt(
      webPageSizeInput?.value,
      DEFAULT_WEB_PAGE_SIZE,
    ),
    cliPageSize: parsePositiveInt(
      cliPageSizeInput?.value,
      DEFAULT_CLI_PAGE_SIZE,
    ),
    autoCheckUpdate: webSettings.autoCheckUpdate,
    autoSwitchInvalidSources: !!document.getElementById(
      "setting-auto-switch-invalid-sources",
    )?.checked,
    autoCacheOnPlay: !!document.getElementById("setting-auto-cache-on-play")
      ?.checked,
    updateRepoUrl: webSettings.updateRepoUrl || DEFAULT_UPDATE_REPO_URL,
    githubProxyEnabled: !!webSettings.githubProxyEnabled,
    githubProxyUrl: webSettings.githubProxyUrl || DEFAULT_GITHUB_PROXY_URL,
    vgChangeCover: !!document.getElementById("setting-vg-change-cover")
      ?.checked,
    vgChangeAudio: !!document.getElementById("setting-vg-change-audio")
      ?.checked,
    vgChangeLyric: !!document.getElementById("setting-vg-change-lyric")
      ?.checked,
    vgExportVideo: !!document.getElementById("setting-vg-export-video")
      ?.checked,
  });

  const data = {};
  document.querySelectorAll('input[id^="cookie-"]').forEach((input) => {
    data[input.id.replace("cookie-", "")] = input.value;
  });

  try {
    const [cookiesResponse, settingsResponse] = await Promise.all([
      fetch(API_ROOT + "/cookies", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: JSON.stringify(data),
      }),
      fetch(API_ROOT + "/settings", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: JSON.stringify(nextSettings),
      }),
    ]);
    const cookiesPayload = await cookiesResponse.json().catch(() => null);
    const savedSettings = await settingsResponse.json().catch(() => null);
    if (
      handleConfigAuthResponse(cookiesResponse, cookiesPayload) ||
      handleConfigAuthResponse(settingsResponse, savedSettings)
    )
      return;
    if (!cookiesResponse.ok || !settingsResponse.ok) {
      throw new Error("保存失败，请稍后重试");
    }
    applyWebSettings(savedSettings || nextSettings);
    setAuthFloatLoggedIn(true);
    alert("保存成功");
    document.getElementById("cookieModal").style.display = "none";
  } catch (error) {
    alert(error.message || "保存失败，请稍后重试");
  }
}

window.addEventListener("scroll", () => {
  const topBtn = document.getElementById("back-to-top");
  const bottomBtn = document.getElementById("back-to-bottom");
  if (topBtn) {
    if (window.scrollY > 300) {
      topBtn.classList.add("show");
    } else {
      topBtn.classList.remove("show");
    }
  }
  if (bottomBtn) {
    if (window.scrollY > 300) {
      bottomBtn.classList.add("show");
    } else {
      bottomBtn.classList.remove("show");
    }
  }
});

function scrollToTop() {
  window.scrollTo({ top: 0, behavior: "smooth" });
}

function scrollToBottom() {
  window.scrollTo({ top: document.body.scrollHeight, behavior: "smooth" });
}

// ==========================================
// 浮动翻页键
// ==========================================

function updateFloatPageNav() {
  const numEl = document.getElementById("float-page-num");
  if (!numEl) return;

  // 本地音乐页
  if (isLocalMusicPageActive()) {
    const bar = document.getElementById("local-music-pagination");
    if (!bar) { numEl.textContent = "1"; return; }
    const cur = parseInt(bar.dataset.currentPage, 10);
    if (Number.isFinite(cur)) numEl.textContent = String(cur);
    return;
  }

  // 搜索结果页：从 URL 获取 page 参数
  const urlParams = new URLSearchParams(window.location.search);
  const page = parseInt(urlParams.get("page"), 10) || 1;
  // 总页数从页面 DOM 获取
  const pageSummary = document.querySelector(".page-summary");
  if (!pageSummary) { numEl.textContent = String(page); return; }
  const match = pageSummary.textContent.match(/第\s*\d+\s*\/\s*(\d+)\s*页/);
  if (!match) { numEl.textContent = String(page); return; }
  numEl.textContent = String(page);
}

function floatPageUp() {
  if (isLocalMusicPageActive()) {
    const bar = document.getElementById("local-music-pagination");
    if (!bar) return;
    const cur = parseInt(bar.dataset.currentPage, 10);
    if (cur > 1) goToPage(cur - 1);
  } else {
    const urlParams = new URLSearchParams(window.location.search);
    const page = parseInt(urlParams.get("page"), 10) || 1;
    if (page > 1) goToPage(page - 1);
  }
}

function floatPageDown() {
  if (isLocalMusicPageActive()) {
    const bar = document.getElementById("local-music-pagination");
    if (!bar) return;
    const cur = parseInt(bar.dataset.currentPage, 10);
    const total = parseInt(bar.dataset.totalPages, 10);
    if (cur < total) goToPage(cur + 1);
  } else {
    // 搜索结果页：goToPage 会处理越界
    const urlParams = new URLSearchParams(window.location.search);
    const page = parseInt(urlParams.get("page"), 10) || 1;
    goToPage(page + 1);
  }
}

// 点击页数 → 输入框 → Enter/blur 跳转
function floatPageNumClick() {
  const numEl = document.getElementById("float-page-num");
  if (!numEl) return;
  const currentVal = numEl.textContent.trim();
  // 如果已经是输入框则忽略
  if (numEl.tagName === "INPUT") return;

  const input = document.createElement("input");
  input.type = "number";
  input.min = "1";
  input.value = currentVal;
  input.className = numEl.className;
  input.id = numEl.id;
  input.style.width = "40px";
  input.style.height = "28px";
  input.style.textAlign = "center";
  input.style.fontSize = "13px";
  input.style.fontWeight = "700";
  input.style.color = "#10b981";
  input.style.border = "2px solid #10b981";
  input.style.borderRadius = "6px";
  input.style.outline = "none";
  input.style.background = "white";

  input.onkeydown = function(e) {
    if (e.key === "Enter") {
      e.preventDefault();
      const val = parseInt(this.value, 10);
      if (Number.isFinite(val) && val > 0) {
        goToPage(val);
      }
    }
    if (e.key === "Escape") {
      this.parentNode.replaceChild(numEl, this);
    }
  };
  input.onblur = function() {
    const val = parseInt(this.value, 10);
    if (Number.isFinite(val) && val > 0) {
      goToPage(val);
    }
  };

  numEl.parentNode.replaceChild(input, numEl);
  input.focus();
  input.select();
}

let defaultDocumentTitle = document.title;
let mediaSessionSyncTimer = 0;
let mediaSessionSyncVersion = 0;
const mediaSessionCoverCache = new Map();

function mediaSessionControllerSupported() {
  return typeof navigator !== "undefined" && !!navigator.mediaSession;
}

function mediaSessionMetadataSupported() {
  return (
    mediaSessionControllerSupported() &&
    typeof window.MediaMetadata === "function"
  );
}

function getCurrentAPlayerAudio() {
  if (!ap || !ap.list || !Array.isArray(ap.list.audios)) return null;
  const index = ap.list.index;
  if (typeof index !== "number" || index < 0) return null;
  return ap.list.audios[index] || null;
}

function getPlaybackCardID(audio) {
  return String(audio?.original_id || audio?.custom_id || "").trim();
}

function buildMediaSessionTrackKey(audio = getCurrentAPlayerAudio()) {
  if (!audio) return "";

  const customId = String(audio.custom_id || "").trim();
  if (customId) return customId;

  const source = String(audio.source || "").trim();
  const name = String(audio.name || "").trim();
  const artist = String(audio.artist || "").trim();
  if (!source && !name && !artist) return "";

  return `${source}::${name}::${artist}`;
}

function normalizeMediaSessionURL(value) {
  const raw = String(value || "").trim();
  if (!raw) return "";

  try {
    return new URL(raw, window.location.href).toString();
  } catch (_) {
    return raw;
  }
}

function isTransientMediaSessionURL(value) {
  const lowered = String(value || "")
    .trim()
    .toLowerCase();
  return lowered.startsWith("data:") || lowered.startsWith("blob:");
}

function extractURLFromBackgroundImage(value) {
  const raw = String(value || "").trim();
  if (!raw) return "";

  const match = raw.match(/^url\((['"]?)(.*)\1\)$/i);
  if (!match || !match[2]) return "";
  return match[2].trim();
}

function buildMediaSessionCoverURL(audio = getCurrentAPlayerAudio()) {
  const candidates = [];
  const trackKey = buildMediaSessionTrackKey(audio);

  if (audio) {
    candidates.push({
      url: audio.cover,
      source: audio.source || "",
    });
  }

  const currentId =
    getPlaybackCardID(audio) || String(currentPlayingId || "").trim();
  if (currentId) {
    const card = Array.from(document.querySelectorAll(".song-card")).find(
      (item) => item?.dataset?.id === currentId,
    );
    if (card) {
      const imgEl = card.querySelector(".cover-wrapper img");
      if (imgEl && imgEl.src) {
        candidates.unshift({
          url: imgEl.src,
          source: card.dataset.source || audio?.source || "",
        });
      }

      if (card.dataset.cover) {
        candidates.push({
          url: card.dataset.cover,
          source: card.dataset.source || audio?.source || "",
        });
      }
    }
  }

  const apPic = document.querySelector(".aplayer-pic");
  if (apPic?.style?.backgroundImage) {
    const playerCover = extractURLFromBackgroundImage(
      apPic.style.backgroundImage,
    );
    if (playerCover) {
      candidates.unshift({
        url: playerCover,
        source: audio?.source || "",
      });
    }
  }

  const fallbackCandidates = [];
  const seen = new Set();
  for (const candidate of candidates) {
    const normalized = normalizeMediaSessionURL(candidate?.url);
    if (!normalized || seen.has(normalized)) continue;
    seen.add(normalized);

    if (isTransientMediaSessionURL(normalized)) {
      fallbackCandidates.push(normalized);
      continue;
    }

    try {
      const parsed = new URL(normalized, window.location.href);
      if (
        parsed.origin === window.location.origin &&
        parsed.pathname === `${API_ROOT}/cover_proxy`
      ) {
        const resolved = parsed.toString();
        if (trackKey) {
          mediaSessionCoverCache.set(trackKey, resolved);
        }
        return resolved;
      }

      const proxy = new URL(`${API_ROOT}/cover_proxy`, window.location.href);
      proxy.searchParams.set("url", parsed.toString());
      const sourceValue = String(candidate?.source || "").trim();
      if (sourceValue) {
        proxy.searchParams.set("source", sourceValue);
      }
      const resolved = proxy.toString();
      if (trackKey) {
        mediaSessionCoverCache.set(trackKey, resolved);
      }
      return resolved;
    } catch (_) {
      if (trackKey) {
        mediaSessionCoverCache.set(trackKey, normalized);
      }
      return normalized;
    }
  }

  if (trackKey) {
    const cached = mediaSessionCoverCache.get(trackKey);
    if (cached) {
      return cached;
    }
  }

  // Do not expose transient blob/data artwork URLs to MediaSession.
  // They often expire quickly and cause a visible "flash then disappear" effect on MPRIS clients.
  if (fallbackCandidates.length > 0 && trackKey) {
    const cached = mediaSessionCoverCache.get(trackKey);
    if (cached && !isTransientMediaSessionURL(cached)) {
      return cached;
    }
  }

  return "";
}

function buildMediaSessionArtwork(audio = getCurrentAPlayerAudio()) {
  const src = buildMediaSessionCoverURL(audio);
  if (!src) return [];

  return [{ src }];
}

function updateDocumentTitleForMedia(audio) {
  if (!audio || !audio.name) {
    document.title = defaultDocumentTitle;
    return;
  }

  const parts = [audio.name];
  if (audio.artist) {
    parts.push(audio.artist);
  }
  document.title = `${parts.join(" - ")} | music-dl`;
}

function shouldPreserveMediaSessionMetadata() {
  return !!ap?.list?.audios?.length;
}

function updateMediaSessionMetadata(audio = getCurrentAPlayerAudio()) {
  if (!mediaSessionControllerSupported()) return;

  if (!audio) {
    if (shouldPreserveMediaSessionMetadata()) {
      return;
    }
    if (mediaSessionMetadataSupported()) {
      navigator.mediaSession.metadata = null;
    }
    updateDocumentTitleForMedia(null);
    return;
  }

  if (!mediaSessionMetadataSupported()) {
    updateDocumentTitleForMedia(audio);
    return;
  }

  const metadata = {
    title: audio.name || "music-dl",
    artist: audio.artist || "",
  };

  if (audio.album) {
    metadata.album = audio.album;
  }

  const artwork = buildMediaSessionArtwork(audio);
  if (artwork.length > 0) {
    metadata.artwork = artwork;
  }

  navigator.mediaSession.metadata = new MediaMetadata(metadata);
  updateDocumentTitleForMedia(audio);
}

function updateMediaSessionPlaybackState() {
  if (!mediaSessionControllerSupported()) return;

  const audio = getCurrentAPlayerAudio();
  if (!ap?.audio || !audio) {
    navigator.mediaSession.playbackState = "none";
    return;
  }

  navigator.mediaSession.playbackState = ap.audio.paused ? "paused" : "playing";
}

function updateMediaSessionPositionState() {
  if (!mediaSessionControllerSupported()) return;
  if (
    !ap?.audio ||
    typeof navigator.mediaSession.setPositionState !== "function"
  )
    return;

  const duration = Number(ap.audio.duration);
  const position = Number(ap.audio.currentTime);
  const playbackRate = Number(ap.audio.playbackRate) || 1;

  if (!Number.isFinite(duration) || duration <= 0) return;
  if (!Number.isFinite(position) || position < 0) return;

  try {
    navigator.mediaSession.setPositionState({
      duration,
      playbackRate,
      position: Math.min(position, duration),
    });
  } catch (_) {}
}

function syncMediaSession(audio = getCurrentAPlayerAudio()) {
  if (!mediaSessionControllerSupported()) return;
  updateMediaSessionMetadata(audio);
  updateMediaSessionPlaybackState();
  updateMediaSessionPositionState();
}

function scheduleMediaSessionSync(
  audio = getCurrentAPlayerAudio(),
  delayMs = 160,
) {
  if (!mediaSessionControllerSupported()) return;

  const expectedId = String(audio?.custom_id || "").trim();
  const syncVersion = ++mediaSessionSyncVersion;

  if (mediaSessionSyncTimer) {
    clearTimeout(mediaSessionSyncTimer);
  }

  mediaSessionSyncTimer = setTimeout(
    () => {
      if (syncVersion !== mediaSessionSyncVersion) return;

      const currentAudio = getCurrentAPlayerAudio();
      if (
        expectedId &&
        currentAudio &&
        String(currentAudio.custom_id || "").trim() !== expectedId
      ) {
        return;
      }

      syncMediaSession(currentAudio || audio);
    },
    Math.max(0, Number(delayMs) || 0),
  );
}

function clearMediaSession() {
  if (!mediaSessionControllerSupported()) return;
  mediaSessionSyncVersion++;
  if (mediaSessionSyncTimer) {
    clearTimeout(mediaSessionSyncTimer);
    mediaSessionSyncTimer = 0;
  }
  if (mediaSessionMetadataSupported()) {
    navigator.mediaSession.metadata = null;
  }
  navigator.mediaSession.playbackState = "none";
  updateDocumentTitleForMedia(null);
}

function switchTrackByOffset(offset) {
  if (!ap?.list?.audios?.length) return;

  const total = ap.list.audios.length;
  const currentIndex =
    typeof ap.list.index === "number" && ap.list.index >= 0 ? ap.list.index : 0;
  const nextIndex = (currentIndex + offset + total) % total;

  ap.list.switch(nextIndex);
  ap.play();
}

function seekCurrentTrack(position) {
  if (!ap?.audio) return;

  const duration = Number(ap.audio.duration);
  if (!Number.isFinite(duration) || duration <= 0) return;

  const target = Math.max(0, Math.min(duration, Number(position) || 0));
  try {
    if (typeof ap.audio.fastSeek === "function") {
      ap.audio.fastSeek(target);
    } else {
      ap.audio.currentTime = target;
    }
  } catch (_) {
    ap.audio.currentTime = target;
  }

  updateMediaSessionPositionState();
}

function adjustCurrentTrackPosition(offset) {
  if (!ap?.audio) return;
  seekCurrentTrack((Number(ap.audio.currentTime) || 0) + offset);
}

function registerMediaSessionAction(action, handler) {
  if (!mediaSessionControllerSupported()) return;
  try {
    navigator.mediaSession.setActionHandler(action, handler);
  } catch (_) {}
}

function bindMediaSessionAudioEvents() {
  if (!ap?.audio || ap.audio.dataset.mediaSessionBound === "1") return;

  ap.audio.dataset.mediaSessionBound = "1";
  const syncPosition = () => updateMediaSessionPositionState();
  const syncState = () => {
    updateMediaSessionPlaybackState();
    updateMediaSessionPositionState();
  };

  [
    "timeupdate",
    "durationchange",
    "loadedmetadata",
    "seeked",
    "ratechange",
  ].forEach((eventName) => {
    ap.audio.addEventListener(eventName, syncPosition);
  });
  ["play", "pause"].forEach((eventName) => {
    ap.audio.addEventListener(eventName, syncState);
  });
  ap.audio.addEventListener("loadedmetadata", () => {
    syncMediaSession();
    scheduleMediaSessionSync(getCurrentAPlayerAudio(), 180);
  });
}

function setupMediaSession() {
  if (!mediaSessionControllerSupported()) return;

  bindMediaSessionAudioEvents();

  registerMediaSessionAction("play", () => {
    if (!ap?.list?.audios?.length) return;
    ap.play();
  });
  registerMediaSessionAction("pause", () => {
    ap?.pause();
  });
  registerMediaSessionAction("stop", () => {
    ap?.pause();
    seekCurrentTrack(0);
    syncMediaSession();
  });
  registerMediaSessionAction("previoustrack", () => {
    switchTrackByOffset(-1);
  });
  registerMediaSessionAction("nexttrack", () => {
    switchTrackByOffset(1);
  });
  registerMediaSessionAction("seekbackward", (details) => {
    adjustCurrentTrackPosition(-(Number(details?.seekOffset) || 10));
  });
  registerMediaSessionAction("seekforward", (details) => {
    adjustCurrentTrackPosition(Number(details?.seekOffset) || 10);
  });
  registerMediaSessionAction("seekto", (details) => {
    if (!details || typeof details.seekTime !== "number") return;
    seekCurrentTrack(details.seekTime);
  });

  bindMediaKeyFallback();
  syncMediaSession();
}

let mediaKeysBound = false;

// bindMediaKeyFallback 监听 WebView 经 DOM 下发的媒体键（部分车机/头机会把方向盘
// 媒体键作为 KeyboardEvent 送进网页），作为系统 MediaSession 之外的兜底切歌途径。
function bindMediaKeyFallback() {
  if (mediaKeysBound) return;
  mediaKeysBound = true;

  document.addEventListener("keydown", function (event) {
    if (event.defaultPrevented || event.isComposing) return;
    if (
      typeof ap === "undefined" ||
      !ap ||
      !ap.list ||
      !ap.list.audios ||
      ap.list.audios.length === 0
    )
      return;

    switch (event.key) {
      case "MediaTrackNext":
        event.preventDefault();
        switchTrackByOffset(1);
        break;
      case "MediaTrackPrevious":
        event.preventDefault();
        switchTrackByOffset(-1);
        break;
      case "MediaPlayPause":
        event.preventDefault();
        togglePlayback();
        break;
      case "MediaStop":
        event.preventDefault();
        if (ap.audio) ap.pause();
        break;
      default:
        break;
    }
  });
}

const KaraokeLyrics = (() => {
  const timeRe = /\[(\d+):(\d+)\.(\d{1,3})\]/g;
  const fallbackLineDuration = 1200;
  let container = null;
  let body = null;
  let currentKey = "";
  let groups = [];
  let activeIndex = -1;
  let visible = false;
  let animationFrame = 0;

  function getProgress(ms, start, end) {
    if (ms <= start) return 0;
    if (!Number.isFinite(end) || end <= start) return 1;
    return Math.max(0, Math.min(1, (ms - start) / (end - start)));
  }

  function getLyricHost() {
    return (
      document.querySelector(".aplayer.aplayer-fixed .aplayer-lrc") ||
      document.querySelector(".aplayer-lrc") ||
      document.querySelector(
        ".aplayer.aplayer-fixed .aplayer-lrc .aplayer-lrc-contents",
      ) ||
      document.querySelector(".aplayer-lrc .aplayer-lrc-contents") ||
      document.body
    );
  }

  function ensureContainer() {
    const host = getLyricHost();
    if (!container) {
      container = document.createElement("div");
      container.id = "karaoke-lyrics";
      container.className = "karaoke-lyrics";
      container.hidden = true;
      container.innerHTML = '<div class="karaoke-lyrics-body"></div>';
      body = container.querySelector(".karaoke-lyrics-body");
    }
    if (container.parentElement !== host) {
      host.appendChild(container);
    }
    container.dataset.fallback = host === document.body ? "true" : "false";
    return container;
  }

  function stopLoop() {
    if (animationFrame) {
      cancelAnimationFrame(animationFrame);
      animationFrame = 0;
    }
  }

  function animationTick() {
    animationFrame = 0;
    update();
    if (visible && ap?.audio && !ap.audio.paused) {
      animationFrame = requestAnimationFrame(animationTick);
    }
  }

  function startLoop() {
    if (animationFrame || !visible || !ap?.audio) return;
    if (ap.audio.paused) {
      update();
      return;
    }
    animationFrame = requestAnimationFrame(animationTick);
  }

  function timeToMs(parts) {
    const minute = Number(parts[1]) || 0;
    const second = Number(parts[2]) || 0;
    let ms = String(parts[3] || "0");
    if (ms.length === 1) ms += "00";
    if (ms.length === 2) ms += "0";
    return minute * 60000 + second * 1000 + Number(ms.slice(0, 3));
  }

  function parseLine(line) {
    timeRe.lastIndex = 0;
    const matches = Array.from(line.matchAll(timeRe));
    if (matches.length === 0) return null;
    const start = timeToMs(matches[0]);
    const words = [];
    for (let i = 0; i < matches.length; i++) {
      const textStart = matches[i].index + matches[i][0].length;
      const textEnd =
        i + 1 < matches.length ? matches[i + 1].index : line.length;
      const text = line.slice(textStart, textEnd);
      if (!text) continue;
      words.push({
        start: timeToMs(matches[i]),
        end: i + 1 < matches.length ? timeToMs(matches[i + 1]) : null,
        text,
      });
    }
    const text = line.replace(timeRe, "").trim();
    return { start, words, text, verbatim: matches.length > 1 };
  }

  function normalizeGroupWords(
    sourceWords,
    groupStart,
    groupEnd,
    fallbackText,
  ) {
    const words =
      Array.isArray(sourceWords) && sourceWords.length > 0
        ? sourceWords
        : [{ text: fallbackText || "", start: groupStart, end: groupEnd }];
    return words
      .map((word, index) => {
        const start = Number(word?.start);
        const nextStart =
          index + 1 < words.length ? Number(words[index + 1]?.start) : NaN;
        let end = Number(word?.end);
        const safeStart = Number.isFinite(start) ? start : groupStart;
        if (!Number.isFinite(end) || end <= safeStart) {
          end =
            Number.isFinite(nextStart) && nextStart > safeStart
              ? nextStart
              : groupEnd;
        }
        return {
          text: String(word?.text || ""),
          start: safeStart,
          end,
        };
      })
      .filter((word) => word.text !== "");
  }

  function normalizeGroups(rawGroups) {
    return (rawGroups || [])
      .map((group, index, list) => {
        const start = Number(group?.start || 0);
        const nextStart =
          index + 1 < list.length ? Number(list[index + 1]?.start || 0) : 0;
        const end =
          nextStart > start ? nextStart : start + fallbackLineDuration;
        const lines = (group?.lines || []).map((line) => ({
          ...line,
          text: String(line?.text || ""),
          words: normalizeGroupWords(line?.words, start, end, line?.text),
        }));
        return { start, end, lines };
      })
      .filter((group) => group.lines.some((line) => line.text));
  }

  function parse(raw) {
    const map = new Map();
    let hasVerbatim = false;
    String(raw || "")
      .split(/\r?\n/)
      .forEach((rawLine) => {
        const line = rawLine.trim();
        if (!line || /^\[[A-Za-z]+:[^\]]*\]$/.test(line)) return;
        const parsed = parseLine(line);
        if (!parsed || !parsed.text) return;
        hasVerbatim = hasVerbatim || parsed.verbatim;
        if (!map.has(parsed.start)) {
          map.set(parsed.start, { start: parsed.start, lines: [] });
        }
        map.get(parsed.start).lines.push(parsed);
      });
    const result = normalizeGroups(
      Array.from(map.values()).sort((a, b) => a.start - b.start),
    );
    const hasMultiLang = result.some((group) => group.lines.length > 1);
    return {
      type: hasVerbatim || hasMultiLang ? "karaoke" : "line",
      groups: result,
    };
  }

  function looksLikeRomajiLine(line) {
    const text = String(line?.text || "").trim();
    if (!text) return false;
    const latinCount = (text.match(/[A-Za-z]/g) || []).length;
    const cjkOrKanaCount = (text.match(/[\u3040-\u30ff\u3400-\u9fff]/g) || [])
      .length;
    return latinCount > 0 && latinCount >= cjkOrKanaCount;
  }

  function splitGroupLines(lines) {
    const [orig, ...extras] = lines || [];
    let roma = null;
    let trans = null;
    extras.forEach((line) => {
      if (!roma && looksLikeRomajiLine(line)) {
        roma = line;
        return;
      }
      if (!trans) {
        trans = line;
        return;
      }
      if (!roma) {
        roma = line;
      }
    });
    return { orig, roma, trans };
  }

  function renderGroup(group, index) {
    const { orig, roma, trans } = splitGroupLines(group.lines);
    const renderWords = (words, fallbackStart, fallbackEnd) =>
      (words || [])
        .map((word) =>
          [
            `<span class="karaoke-word" data-start="${word.start || fallbackStart}" data-end="${word.end || fallbackEnd}" style="--karaoke-progress:0%;">`,
            `<span class="karaoke-word-base">${escapeHTML(word.text)}</span>`,
            `<span class="karaoke-word-fill">${escapeHTML(word.text)}</span>`,
            "</span>",
          ].join(""),
        )
        .join("");
    const renderLine = (line, className, useWordProgress) => {
      if (!line?.text) return "";
      const content =
        useWordProgress && Array.isArray(line.words) && line.words.length > 0
          ? renderWords(line.words, group.start, group.end)
          : escapeHTML(line.text);
      return `<div class="${className}">${content}</div>`;
    };
    return [
      `<div class="karaoke-group" data-index="${index}" data-start="${group.start}">`,
      renderLine(orig, "karaoke-orig", true),
      renderLine(roma, "karaoke-roma", !!roma?.verbatim),
      renderLine(trans, "karaoke-trans", !!trans?.verbatim),
      "</div>",
    ].join("");
  }

  function show(nextGroups) {
    ensureContainer();
    groups = nextGroups || [];
    activeIndex = -1;
    body.innerHTML = groups.map(renderGroup).join("");
    visible = groups.length > 0;
    container.hidden = !visible;
    document.body.classList.toggle("karaoke-lyrics-active", visible);
    startLoop();
  }

  function hide() {
    ensureContainer();
    stopLoop();
    visible = false;
    groups = [];
    activeIndex = -1;
    body.innerHTML = "";
    container.hidden = true;
    document.body.classList.remove("karaoke-lyrics-active");
  }

  async function load(audio) {
    ensureContainer();
    if (!floatingLyricsEnabled()) {
      currentKey = "";
      hide();
      return;
    }
    const key = `${audio?.source || ""}:${audio?.custom_id || ""}:${audio?.raw_lrc || audio?.lrc || ""}`;
    if (!audio || !key.trim()) {
      currentKey = "";
      hide();
      return;
    }
    if (key === currentKey) {
      update();
      return;
    }
    currentKey = key;
    hide();

    const url = audio.raw_lrc || audio.lrc;
    if (!url) return;
    try {
      const response = await fetch(url);
      if (!response.ok) return;
      const raw = await response.text();
      if (key !== currentKey) return;
      const parsed = parse(raw);
      if (parsed.type !== "karaoke") {
        hide();
        return;
      }
      show(parsed.groups);
      update();
      startLoop();
    } catch (_) {
      hide();
    }
  }

  function update() {
    if (!floatingLyricsEnabled()) return;
    if (!visible || !ap?.audio || groups.length === 0) return;
    const ms = Math.max(0, (Number(ap.audio.currentTime) || 0) * 1000);
    let nextIndex = -1;
    for (let i = 0; i < groups.length; i++) {
      if (ms >= groups[i].start) nextIndex = i;
      else break;
    }
    if (nextIndex < 0) return;

    const active = body.querySelector(
      `.karaoke-group[data-index="${nextIndex}"]`,
    );
    if (!active) return;
    if (nextIndex !== activeIndex) {
      body
        .querySelectorAll(".karaoke-group.active")
        .forEach((el) => el.classList.remove("active"));
      active.classList.add("active");
      const targetTop =
        active.offsetTop -
        Math.max(0, (body.clientHeight - active.clientHeight) / 2);
      body.scrollTo({ top: Math.max(0, targetTop), behavior: "smooth" });
      activeIndex = nextIndex;
    }
    body.querySelectorAll(".karaoke-word").forEach((word) => {
      const start = Number(word.dataset.start || 0);
      const end = Number(word.dataset.end || start + fallbackLineDuration);
      const progress = getProgress(ms, start, end);
      // 核心修复：100% 进度时直接加上安全距离，防止强迫症切缝
      word.style.setProperty(
        "--karaoke-progress",
        progress === 1 ? "calc(100% + 8px)" : `${(progress * 100).toFixed(3)}%`,
      );
      word.classList.toggle("is-active", progress > 0 && progress < 1);
    });
  }

  function handlePlayStateChange(isPlaying) {
    if (!floatingLyricsEnabled()) {
      hide();
      return;
    }
    if (isPlaying) {
      startLoop();
      return;
    }
    update();
    stopLoop();
  }

  return { load, update, hide, parse, handlePlayStateChange };
})();

window.KaraokeLyrics = KaraokeLyrics;

// APlayer Config
const ap = new APlayer({
  container: document.getElementById("aplayer"),
  fixed: true,
  autoplay: false,
  theme: "#10b981",
  loop: "all",
  order: "list",
  preload: "metadata",
  volume: 0.7,
  listFolded: false,
  lrcType: 3,
  audio: [],
});

window.ap = ap;

setupMediaSession();
ap.audio.addEventListener("timeupdate", () => KaraokeLyrics.update());
ap.audio.addEventListener("seeked", () => KaraokeLyrics.update());
ap.audio.addEventListener("loadedmetadata", () =>
  KaraokeLyrics.load(getCurrentAPlayerAudio()),
);
ap.audio.addEventListener("play", () =>
  KaraokeLyrics.handlePlayStateChange(true),
);
ap.audio.addEventListener("pause", () =>
  KaraokeLyrics.handlePlayStateChange(false),
);
ap.audio.addEventListener("ended", () =>
  KaraokeLyrics.handlePlayStateChange(false),
);

// === 播放模式切换按钮 ===
(function initPlayModeToggle() {
  const MODES = [
    { order: "list", loop: "all", cls: "mode-order", label: "顺序播放" },
    { order: "random", loop: "all", cls: "mode-random", label: "随机播放" },
    { order: "list", loop: "one", cls: "mode-one", label: "单曲循环" },
  ];

  let modeIndex = parseInt(localStorage.getItem("musicdl:playmode") || "0", 10);
  if (![0, 1, 2].includes(modeIndex)) modeIndex = 0;

  function applyMode(idx) {
    const m = MODES[idx];
    if (ap && ap.options) {
      ap.options.order = m.order;
      ap.options.loop = m.loop;
    }
  }

  let attempts = 0;
  function tryInject() {
    attempts++;
    // 尝试注入，最多重试 30 次 (约 3 秒)
    if (attempts > 30) return;

    const apFixed = document.querySelector(".aplayer.aplayer-fixed");
    if (!apFixed) {
      setTimeout(tryInject, 100);
      return;
    }

    if (document.querySelector(".playmode-toggle-btn")) return;

    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "playmode-toggle-btn";

    function refresh() {
      const m = MODES[modeIndex];
      const icons = {
        "mode-order": "fa-list-ol",
        "mode-random": "fa-shuffle",
        "mode-one": "fa-repeat",
      };
      btn.innerHTML = `<i class="fa-solid ${icons[m.cls]}" aria-hidden="true"></i>`;
      btn.title = m.label;
      btn.setAttribute("aria-label", m.label);
    }

    btn.addEventListener("click", (e) => {
      e.stopPropagation();
      e.preventDefault();
      modeIndex = (modeIndex + 1) % MODES.length;
      localStorage.setItem("musicdl:playmode", modeIndex);
      applyMode(modeIndex);
      refresh();
    });

    apFixed.appendChild(btn);
    refresh();
    if (modeIndex !== 0) applyMode(modeIndex);
  }

  // 等 APlayer 渲染出来后再注入
  setTimeout(tryInject, 100);
})();

setTimeout(() => {
  const apPic = document.querySelector(".aplayer-pic");
  if (apPic) {
    apPic.style.cursor = "pointer";
    apPic.title = "点击打开详情/生成视频";

    apPic.addEventListener(
      "click",
      (e) => {
        if (
          e.target.closest(".aplayer-button") ||
          e.target.closest(".aplayer-play")
        ) {
          return;
        }
        e.stopPropagation();
        e.preventDefault();

        const idx = ap.list.index;
        const audio = ap.list.audios[idx];

        if (audio && audio.custom_id && window.VideoGen) {
          window.VideoGen.open({
            id: audio.custom_id,
            source: audio.source || "netease",
            name: audio.name,
            artist: audio.artist,
            album: audio.album || "",
            cover: audio.cover,
            duration: parseInt(audio.duration) || 0,
            extra: audio.extra || "",
          });
        }
      },
      true,
    );
  }
}, 800);

ap.on("listswitch", (e) => {
  const index = e.index;
  const newAudio = ap.list.audios[index];
  const playbackCardID = getPlaybackCardID(newAudio);
  if (playbackCardID) {
    currentPlayingId = playbackCardID;
    window.currentPlayingId = currentPlayingId;
    highlightCard(currentPlayingId);
    syncAllPlayButtons();

    const vgModal = document.getElementById("vg-modal");
    if (vgModal && vgModal.classList.contains("active") && window.VideoGen) {
      if (
        !window.VideoGen.data ||
        window.VideoGen.data.id !== newAudio.custom_id
      ) {
        window.VideoGen.open({
          id: newAudio.custom_id,
          source: newAudio.source || "netease",
          name: newAudio.name,
          artist: newAudio.artist,
          album: newAudio.album || "",
          cover: newAudio.cover,
          duration: parseInt(newAudio.duration) || 0,
          extra: newAudio.extra || "",
        });
      }
    }
  }
  if (
    newAudio &&
    newAudio.source !== "local" &&
    newAudio.source !== "local-file"
  ) {
    scheduleAutoCache(newAudio);
  }
  syncMediaSession(newAudio || getCurrentAPlayerAudio());
  scheduleMediaSessionSync(newAudio || getCurrentAPlayerAudio(), 180);
  KaraokeLyrics.load(newAudio || getCurrentAPlayerAudio());
});

ap.on("play", () => {
  const idx = ap?.list?.index;
  const audio = typeof idx === "number" ? ap.list.audios[idx] : null;
  const playbackCardID = getPlaybackCardID(audio);
  if (playbackCardID) {
    currentPlayingId = playbackCardID;
    window.currentPlayingId = currentPlayingId;
    highlightCard(currentPlayingId);
  }
  syncAllPlayButtons();
  syncMediaSession(audio || getCurrentAPlayerAudio());
  scheduleMediaSessionSync(audio || getCurrentAPlayerAudio(), 180);
  KaraokeLyrics.load(audio || getCurrentAPlayerAudio());

  if (audio && audio.source !== "local" && audio.source !== "local-file") {
    scheduleAutoCache(audio);
  }

  if (window.VideoGen && window.VideoGen.updatePlayBtnState) {
    window.VideoGen.updatePlayBtnState(true);
  }
});

ap.on("pause", () => {
  syncAllPlayButtons();
  syncMediaSession();
  if (window.VideoGen && window.VideoGen.updatePlayBtnState) {
    window.VideoGen.updatePlayBtnState(false);
  }
});

ap.on("ended", () => {
  currentPlayingId = null;
  window.currentPlayingId = null;
  highlightCard(null);
  syncAllPlayButtons();
  scheduleMediaSessionSync(getCurrentAPlayerAudio(), 180);
  KaraokeLyrics.hide();
});

function highlightCard(targetId) {
  document
    .querySelectorAll(".song-card")
    .forEach((c) => c.classList.remove("playing-active"));
  if (!targetId) return;
  const target = document.querySelector(`.song-card[data-id="${targetId}"]`);
  if (target) {
    target.classList.add("playing-active");
  }
}

function setPlayButtonState(card, isPlaying) {
  if (!card) return;
  const btn = card.querySelector(".btn-play");
  if (!btn) return;
  const icon = btn.querySelector("i");
  if (!icon) return;

  icon.classList.remove("fa-play", "fa-stop");
  icon.classList.add(isPlaying ? "fa-stop" : "fa-play");
  btn.title = isPlaying ? "停止" : "播放";
}

function syncAllPlayButtons() {
  const isActuallyPlaying = ap?.audio && !ap.audio.paused;
  document.querySelectorAll(".song-card").forEach((card) => {
    const id = card.dataset.id;
    const active =
      isActuallyPlaying && currentPlayingId && id === currentPlayingId;
    setPlayButtonState(card, active);
  });
}

function formatDuration(seconds) {
  const s = Number(seconds || 0);
  if (!s || s <= 0) return "-";
  const min = Math.floor(s / 60);
  const sec = Math.floor(s % 60);
  return `${String(min).padStart(2, "0")}:${String(sec).padStart(2, "0")}`;
}

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, (char) => {
    switch (char) {
      case "&":
        return "&amp;";
      case "<":
        return "&lt;";
      case ">":
        return "&gt;";
      case '"':
        return "&quot;";
      case "'":
        return "&#39;";
      default:
        return char;
    }
  });
}

function containsEastAsianChar(value) {
  return /[\u3040-\u30ff\u3400-\u9fff\uf900-\ufaff\uac00-\ud7af]/.test(
    String(value || ""),
  );
}

function trimArtistToken(value) {
  return String(value || "")
    .trim()
    .replace(/^[-_·|\\/,，、；;&\s]+|[-_·|\\/,，、；;&\s]+$/g, "")
    .trim();
}

function splitArtistTokens(artist) {
  const rawArtist = String(artist || "").trim();
  if (!rawArtist) return [];

  let normalized = rawArtist.replace(
    /\s+(feat(?:uring)?\.?|ft\.?|with|x)\s+/gi,
    "|",
  );
  normalized = normalized.replace(/[、，；;]/g, "|");

  if (containsEastAsianChar(rawArtist)) {
    normalized = normalized.replace(/[\/&]/g, "|");
  } else {
    normalized = normalized.replace(/\s+(?:\/|&|\+)\s+/g, "|");
  }

  const tokens = [];
  const seen = new Set();
  normalized.split("|").forEach((item) => {
    const token = trimArtistToken(item);
    const key = token.toLowerCase().replace(/\s+/g, " ").trim();
    if (!key || seen.has(key)) return;
    seen.add(key);
    tokens.push(token);
  });

  return tokens.length > 0 ? tokens : [rawArtist];
}

function buildArtistSearchURL(source, artist) {
  const params = new URLSearchParams({
    q: String(artist || ""),
    type: "song",
    exact_artist: String(artist || ""),
    sources: String(source || ""),
  });
  return `${API_ROOT}/search?${params.toString()}`;
}

function buildAlbumDetailURL(source, albumId) {
  const params = new URLSearchParams({
    id: String(albumId || ""),
    source: String(source || ""),
  });
  return `${API_ROOT}/album?${params.toString()}`;
}

function buildAlbumJumpURL(source, album, artist) {
  const params = new URLSearchParams({
    name: String(album || ""),
    artist: String(artist || ""),
    source: String(source || ""),
  });
  return `${API_ROOT}/album_jump?${params.toString()}`;
}

function getSongAlbumId(song) {
  if (song && song.album_id) return String(song.album_id);
  if (song && song.albumId) return String(song.albumId);
  if (
    song &&
    song.extra &&
    typeof song.extra === "object" &&
    song.extra.album_id
  ) {
    return String(song.extra.album_id);
  }
  return "";
}

function renderArtistLineHTML(song) {
  const artists = splitArtistTokens(song.artist || "");
  const parts = ['<i class="fa-regular fa-user artist-icon"></i>'];

  if (isLocalMusicSourceValue(song.source)) {
    parts.push(`<span>${escapeHTML(song.artist || "未知歌手")}</span>`);
    if (song.album) {
      parts.push('<span class="meta-separator">&middot;</span>');
      parts.push(
        `<span class="meta-link-disabled album-link">${escapeHTML(song.album)}</span>`,
      );
    }
    return parts.join("");
  }

  if (artists.length > 0) {
    artists.forEach((artist, index) => {
      if (index > 0) {
        parts.push('<span class="meta-separator">/</span>');
      }
      parts.push(
        `<a href="${buildArtistSearchURL(song.source, artist)}" class="meta-link artist-link">${escapeHTML(artist)}</a>`,
      );
    });
  } else {
    parts.push("<span>-</span>");
  }

  if (song.album) {
    const albumId = getSongAlbumId(song);
    parts.push('<span class="meta-separator">&middot;</span>');
    const href = albumId
      ? buildAlbumDetailURL(song.source, albumId)
      : buildAlbumJumpURL(song.source, song.album, song.artist || "");
    parts.push(
      `<a href="${href}" class="meta-link album-link">${escapeHTML(song.album)}</a>`,
    );
  }

  return parts.join("");
}

function syncLocalSongActionButtons(card, song) {
  if (!card || !song || !isLocalMusicSourceValue(song.source)) return;
  const actions = card.querySelector(".actions");
  if (!actions) return;

  const extra = normalizeSongExtra(song.extra);
  const deleteBtn = actions.querySelector(".btn-delete-local");
  const insertBefore = (node) => {
    if (deleteBtn) {
      actions.insertBefore(node, deleteBtn);
    } else {
      actions.appendChild(node);
    }
  };

  if (extra.lyric && !actions.querySelector(".btn-lyric")) {
    const lyric = document.createElement("a");
    lyric.id = `lrc-${song.id}`;
    lyric.className = "btn-circle btn-dl btn-lyric";
    lyric.title = "下载歌词";
    lyric.target = "_blank";
    lyric.href = lyricURLsForSong(song).download;
    lyric.innerHTML = '<i class="fa-solid fa-file-lines"></i>';
    const coverBtn = actions.querySelector(".btn-cover");
    if (coverBtn) {
      actions.insertBefore(lyric, coverBtn);
    } else {
      insertBefore(lyric);
    }
  }

  if (song.cover && !actions.querySelector(".btn-cover")) {
    const cover = document.createElement("a");
    cover.className = "btn-circle btn-dl btn-cover";
    cover.title = "下载封面";
    cover.target = "_blank";
    cover.href = buildCoverDownloadURL(song);
    cover.innerHTML = '<i class="fa-regular fa-image"></i>';
    insertBefore(cover);
  }
}

function updateCardWithSong(card, song, options = {}) {
  if (!card || !song) return;
  const oldId = card.dataset.id;
  const shouldInspect = options.inspect !== false;

  card.dataset.id = song.id;
  card.dataset.source = song.source;
  card.dataset.albumId = getSongAlbumId(song);
  card.dataset.album = song.album || "";
  card.dataset.duration = song.duration || 0;
  card.dataset.sortDuration = String(parsePositiveInt(song.duration, 0));
  card.dataset.name = song.name || card.dataset.name;
  card.dataset.artist = song.artist || card.dataset.artist;
  card.dataset.cover = song.cover || "";
  card.dataset.extra = serializeSongExtra(song.extra);

  const titleEl = card.querySelector(".song-info h3");
  if (titleEl) {
    if (song.link) {
      titleEl.innerHTML = `<a href="${song.link}" target="_blank" class="song-title-link" title="打开原始链接">${song.name || ""}</a>`;
    } else {
      titleEl.textContent = song.name || "";
    }
  }

  const artistLine = card.querySelector(".artist-line");
  if (artistLine) {
    artistLine.innerHTML = renderArtistLineHTML(song);
  }

  const sourceTag = card.querySelector(".tag-src, .tag-local");
  if (sourceTag) {
    const isLocal = isLocalMusicSourceValue(song.source);
    sourceTag.textContent = isLocal ? "本地" : song.source;
    sourceTag.classList.toggle("tag-local", isLocal);
    sourceTag.classList.toggle("tag-src", !isLocal);
  }

  const durationTag = card.querySelector(".tag-duration");
  if (durationTag) {
    durationTag.textContent = formatDuration(song.duration);
  }

  const coverWrap = card.querySelector(".cover-wrapper");
  if (coverWrap) {
    let imgEl = coverWrap.querySelector("img");
    if (!imgEl) {
      imgEl = document.createElement("img");
      coverWrap.innerHTML = "";
      coverWrap.appendChild(imgEl);
    }
    imgEl.src = song.cover || "https://via.placeholder.com/150?text=Music";
    imgEl.alt = song.name || "";

    coverWrap.onclick = (e) => {
      e.stopPropagation();
      if (window.VideoGen) {
        window.VideoGen.open({
          id: card.dataset.id,
          source: card.dataset.source,
          name: card.dataset.name,
          artist: card.dataset.artist,
          album: card.dataset.album || "",
          cover: imgEl.src,
          duration: parseInt(card.dataset.duration) || 0,
          extra: card.dataset.extra || "",
        });
      }
    };
  }

  const dl = card.querySelector(".btn-download");
  if (dl) {
    dl.href = buildDownloadURL(
      song.id,
      song.source,
      song.name,
      song.artist,
      song.album || "",
      song.cover || "",
      card.dataset.extra || "",
    );
    dl.id = `dl-${song.id}`;
    dl.title = "保存到本地目录";
  }

  const browserDl = card.querySelector(".btn-browser-download");
  if (browserDl) {
    browserDl.href = buildBrowserDownloadURL(
      song.id,
      song.source,
      song.name,
      song.artist,
      song.album || "",
      song.cover || "",
      card.dataset.extra || "",
    );
    browserDl.id = `browser-dl-${song.id}`;
    browserDl.title = "浏览器下载";
  }

  const lrc = card.querySelector(".btn-lyric");
  if (lrc) {
    lrc.href = lyricURLsForSong(song).download;
    lrc.id = `lrc-${song.id}`;
  }

  const coverBtn = card.querySelector(".btn-cover");
  if (coverBtn) {
    coverBtn.href = buildCoverDownloadURL(song);
  }
  syncLocalSongActionButtons(card, {
    ...song,
    extra: normalizeSongExtra(song.extra),
  });

  const sizeTag = card.querySelector('[id^="size-"]');
  if (sizeTag) {
    sizeTag.id = `size-${song.id}`;
    sizeTag.className = "tag tag-loading";
    sizeTag.innerHTML = '<i class="fa fa-spinner fa-spin"></i>';
  }
  const bitrateTag = card.querySelector('[id^="bitrate-"]');
  if (bitrateTag) {
    bitrateTag.id = `bitrate-${song.id}`;
    bitrateTag.className = "tag tag-loading";
    bitrateTag.innerHTML = '<i class="fa fa-circle-notch fa-spin"></i>';
  }

  if (currentPlayingId === oldId) {
    currentPlayingId = song.id;
  }

  syncAllPlayButtons();
  if (shouldInspect) {
    queueInspectSong(card);
  }
  syncSongToAPlayer(oldId, song);
  if (currentPlayingId === song.id) {
    syncMediaSession();
  }
}

function syncSongToAPlayer(oldId, newSong) {
  if (!ap || !ap.list || !ap.list.audios) return;
  const index = ap.list.audios.findIndex(
    (a) => a.custom_id === oldId || a.original_id === oldId,
  );
  if (index !== -1) {
    const audio = ap.list.audios[index];
    audio.name = newSong.name;
    audio.artist = newSong.artist;
    audio.album = newSong.album || "";
    audio.cover = newSong.cover;
    audio.url = buildStreamURL(
      newSong.id,
      newSong.source,
      newSong.name,
      newSong.artist,
      newSong.album || "",
      newSong.cover || "",
      serializeSongExtra(newSong.extra),
    );
    const lyricURLs = lyricURLsForPlayback(newSong);
    audio.lrc = lyricURLs.line;
    audio.raw_lrc = lyricURLs.auto;
    audio.custom_id = newSong.id;
    audio.original_id = newSong.id;
    audio.source = newSong.source;
    audio.duration = newSong.duration || 0;
    audio.extra = serializeSongExtra(newSong.extra);

    if (ap.list.index === index) {
      ap.list.switch(index);
      if (ap.audio.paused) {
        ap.play();
      }
    }
  }
}

function switchSource(btn, options = {}) {
  const card = btn.closest(".song-card");
  if (!card) return Promise.resolve(false);

  const ds = card.dataset;
  const name = ds.name || "";
  const artist = ds.artist || "";
  const source = ds.source || "";
  if (!name || !source) return Promise.resolve(false);

  btn.disabled = true;
  btn.style.opacity = "0.6";

  const duration = ds.duration || "";
  const url = `${API_ROOT}/switch_source?name=${encodeURIComponent(name)}&artist=${encodeURIComponent(artist)}&source=${encodeURIComponent(source)}&duration=${encodeURIComponent(duration)}`;
  return fetch(url)
    .then((r) => (r.ok ? r.json() : Promise.reject()))
    .then((song) => {
      updateCardWithSong(card, song);
      clearSongCardSelection(card, { deferToolbar: !!options.deferToolbar });
      return true;
    })
    .catch(() => {
      if (!options.silent) {
        alert("换源失败，请稍后重试");
      }
      return false;
    })
    .finally(() => {
      btn.disabled = false;
      btn.style.opacity = "1";
    });
}

function playAllAndJumpTo(btn) {
  const currentCard = btn.closest(".song-card");
  const allCards = Array.from(document.querySelectorAll(".song-card"));
  const clickedIndex = allCards.indexOf(currentCard);

  if (clickedIndex === -1) return;

  const clickedId = currentCard.dataset.id;
  const isActuallyPlaying = ap?.audio && !ap.audio.paused;

  if (currentPlayingId && currentPlayingId === clickedId && isActuallyPlaying) {
    ap.pause();
    try {
      ap.seek(0);
    } catch (e) {}
    currentPlayingId = null;
    highlightCard(null);
    syncAllPlayButtons();
    syncMediaSession();
    return;
  }

  ap.list.clear();
  const playlist = [];

  allCards.forEach((card) => {
    const ds = card.dataset;
    const song = songFromCard(card);
    if (!song) return;
    let coverUrl = ds.cover || "";
    const imgEl = card.querySelector(".cover-wrapper img");
    if (imgEl && imgEl.src) coverUrl = imgEl.src;

    const localMatch = localMusicMatchCache[ds.id];
    const useLocal = localMatch && localMatch.id;
    const playbackSong = useLocal
      ? {
          id: localMatch.id,
          source: "local",
          name: localMatch.name || song.name,
          artist: localMatch.artist || song.artist,
          album: song.album || "",
          cover: song.cover || "",
          duration: song.duration || 0,
          extra: song.extra || "",
        }
      : song;
    const lyricURLs = lyricURLsForPlayback(playbackSong);
    const streamUrl = buildStreamURL(
      playbackSong.id,
      playbackSong.source,
      playbackSong.name,
      playbackSong.artist,
      playbackSong.album || "",
      playbackSong.cover || "",
      playbackSong.extra || "",
    );

    playlist.push({
      name: playbackSong.name,
      artist: playbackSong.artist,
      album: playbackSong.album || "",
      url: streamUrl,
      cover: coverUrl,
      lrc: lyricURLs.line,
      raw_lrc: lyricURLs.auto,
      theme: "#10b981",
      custom_id: playbackSong.id,
      original_id: ds.id,
      source: playbackSong.source,
      duration: playbackSong.duration || parsePositiveInt(ds.duration, 0),
      extra: playbackSong.extra || "",
    });
  });

  ap.list.add(playlist);
  ap.list.switch(clickedIndex);
  ap.play();

  currentPlayingId = clickedId;
  highlightCard(currentPlayingId);
  syncAllPlayButtons();
}

window.playAllAndJumpToId = function (songId) {
  const targetCard = document.querySelector(`.song-card[data-id="${songId}"]`);
  if (targetCard) {
    const btn = targetCard.querySelector(".btn-play");
    if (btn) {
      playAllAndJumpTo(btn);
    }
  }
};

let isBatchMode = false;
let autoSwitchInvalidTimer = 0;
let autoSwitchInvalidInProgress = false;
let autoSwitchInvalidPending = false;
let autoSwitchInvalidLastKey = "";

function clearAutoSwitchInvalidTimer() {
  if (autoSwitchInvalidTimer) {
    window.clearTimeout(autoSwitchInvalidTimer);
    autoSwitchInvalidTimer = 0;
  }
  autoSwitchInvalidPending = false;
}

function resetAutoSwitchInvalidState() {
  clearAutoSwitchInvalidTimer();
  autoSwitchInvalidInProgress = false;
  autoSwitchInvalidLastKey = "";
}

function isAutoSwitchInvalidSourcesEnabled() {
  return webSettings.autoSwitchInvalidSources !== false;
}

function hasPendingSongInspections() {
  return !!document.querySelector('.song-card[data-inspect-pending="1"]');
}

function getInvalidSongCards(root = document) {
  const seen = new Set();
  const cards = [];
  root.querySelectorAll(".tag-fail").forEach((tag) => {
    const card = tag.closest(".song-card");
    if (!card || seen.has(card) || isLocalMusicSourceValue(card.dataset.source))
      return;
    if (card.dataset.autoSwitchInvalidAttempted === "1" && root === document)
      return;
    seen.add(card);
    cards.push(card);
  });
  return cards;
}

function getManualInvalidSongCards(root = document) {
  const seen = new Set();
  const cards = [];
  root.querySelectorAll(".tag-fail").forEach((tag) => {
    const card = tag.closest(".song-card");
    if (!card || seen.has(card) || isLocalMusicSourceValue(card.dataset.source))
      return;
    seen.add(card);
    cards.push(card);
  });
  return cards;
}

function invalidSongCardsKey(cards) {
  return cards
    .map((card) => `${card.dataset.source || ""}:${card.dataset.id || ""}`)
    .sort()
    .join("|");
}

function clearSongCardSelection(card, options = {}) {
  if (!card) return false;
  const cb = card.querySelector(".song-checkbox");
  if (!cb || !cb.checked) return false;
  cb.checked = false;
  card.classList.remove("selected");
  if (!options.deferToolbar) {
    updateBatchToolbar();
  }
  return true;
}

function selectInvalidSongCards(options = {}) {
  const invalidCards = Array.isArray(options.cards)
    ? options.cards
    : getManualInvalidSongCards();
  if (invalidCards.length === 0) {
    if (!options.silent) {
      alert("当前列表中没有检测到无效歌曲");
    }
    return 0;
  }

  let changed = 0;
  const invalidSet = new Set(invalidCards);
  document.querySelectorAll(".song-checkbox").forEach((checkbox) => {
    const card = checkbox.closest(".song-card");
    const shouldCheck = invalidSet.has(card);
    if (!shouldCheck && options.clearExisting !== false) {
      if (checkbox.checked) {
        checkbox.checked = false;
        changed++;
      }
      return;
    }
    if (!shouldCheck) return;
    if (!checkbox.checked) {
      checkbox.checked = true;
      changed++;
    }
  });

  updateBatchToolbar();
  if (!options.silent && changed === 0) {
    alert("无效歌曲已全部选中");
  }
  return invalidCards.length;
}

function scheduleAutoSwitchInvalidSources(
  delay = AUTO_SWITCH_INVALID_DELAY_MS,
) {
  if (!isAutoSwitchInvalidSourcesEnabled()) return;
  if (autoSwitchInvalidInProgress) {
    autoSwitchInvalidPending = true;
    return;
  }
  if (autoSwitchInvalidTimer) {
    window.clearTimeout(autoSwitchInvalidTimer);
  }
  autoSwitchInvalidTimer = window.setTimeout(
    () => {
      autoSwitchInvalidTimer = 0;
      autoSwitchInvalidSources();
    },
    Math.max(0, delay),
  );
}

async function autoSwitchInvalidSources() {
  if (!isAutoSwitchInvalidSourcesEnabled()) return false;
  if (autoSwitchInvalidInProgress) {
    autoSwitchInvalidPending = true;
    return false;
  }
  if (hasPendingSongInspections()) {
    scheduleAutoSwitchInvalidSources(AUTO_SWITCH_INVALID_DELAY_MS);
    return false;
  }

  const invalidCards = getInvalidSongCards();
  if (invalidCards.length === 0) return false;

  const key = invalidSongCardsKey(invalidCards);
  if (key && key === autoSwitchInvalidLastKey) return false;
  autoSwitchInvalidLastKey = key;
  invalidCards.forEach((card) => {
    card.dataset.autoSwitchInvalidAttempted = "1";
  });

  selectInvalidSongCards({ silent: true, cards: invalidCards });
  autoSwitchInvalidInProgress = true;
  showToast(
    "自动换源",
    `检测到 ${invalidCards.length} 首无效歌曲，正在批量换源`,
    "info",
    3500,
  );
  try {
    await batchSwitchSource({
      skipConfirm: true,
      silent: true,
      auto: true,
      cards: invalidCards,
    });
    return true;
  } finally {
    autoSwitchInvalidInProgress = false;
    if (autoSwitchInvalidPending) {
      autoSwitchInvalidPending = false;
      scheduleAutoSwitchInvalidSources();
    }
  }
}

function toggleBatchMode() {
  isBatchMode = !isBatchMode;
  document.body.classList.toggle("batch-mode", isBatchMode);
  const btn = document.getElementById("btn-batch-toggle");
  const toolbar = document.getElementById("batch-toolbar");

  if (!btn || !toolbar) return;

  if (isBatchMode) {
    btn.innerHTML = '<i class="fa-solid fa-xmark"></i> 退出批量';
    btn.style.color = "var(--error-color)";
    toolbar.classList.add("active");
  } else {
    btn.innerHTML = '<i class="fa-solid fa-list-check"></i> 批量操作';
    btn.style.color = "var(--text-sub)";
    toolbar.classList.remove("active");
    document
      .querySelectorAll(".song-checkbox")
      .forEach((cb) => (cb.checked = false));
    updateBatchToolbar();
  }
}

function updateBatchToolbar() {
  const checkedBoxes = document.querySelectorAll(".song-checkbox:checked");
  const count = checkedBoxes.length;
  const selectAllCb = document.getElementById("select-all-checkbox");
  const batchSwitch = document.getElementById("btn-batch-switch");
  const batchDl = document.getElementById("btn-batch-dl");
  const batchDeleteLocal = document.getElementById("btn-batch-delete-local");
  const batchFavLocal = document.getElementById("btn-batch-fav-local");
  const batchFav = document.getElementById("btn-batch-fav");
  const batchRemoveCollection = document.getElementById(
    "btn-batch-remove-collection",
  );

  if (document.getElementById("selected-count")) {
    document.getElementById("selected-count").textContent = count;
  }

  const allBoxes = document.querySelectorAll(".song-checkbox");
  if (allBoxes.length > 0 && selectAllCb) {
    selectAllCb.checked = allBoxes.length === count;
  }

  const selectedSongs = getSelectedSongs();
  const nonLocalCount = selectedSongs.filter(
    (song) => !isLocalMusicSourceValue(song.source),
  ).length;
  const localCount = selectedSongs.filter((song) =>
    isLocalMusicSourceValue(song.source),
  ).length;

  if (count > 0) {
    if (batchSwitch) batchSwitch.disabled = nonLocalCount === 0;
    if (batchDl) batchDl.disabled = nonLocalCount === 0;
    if (batchDeleteLocal) batchDeleteLocal.disabled = localCount === 0;
    if (batchFavLocal) batchFavLocal.disabled = localCount === 0;
    if (batchFav) batchFav.disabled = false;
    if (batchRemoveCollection) batchRemoveCollection.disabled = false;
  } else {
    if (batchSwitch) batchSwitch.disabled = true;
    if (batchDl) batchDl.disabled = true;
    if (batchDeleteLocal) batchDeleteLocal.disabled = true;
    if (batchFavLocal) batchFavLocal.disabled = true;
    if (batchFav) batchFav.disabled = true;
    if (batchRemoveCollection) batchRemoveCollection.disabled = true;
  }

  document
    .querySelectorAll(".song-card")
    .forEach((card) => card.classList.remove("selected"));
  checkedBoxes.forEach((cb) => {
    cb.closest(".song-card").classList.add("selected");
  });
}

function toggleSelectAll(mainCb) {
  const checkboxes = document.querySelectorAll(".song-checkbox");
  checkboxes.forEach((cb) => (cb.checked = mainCb.checked));
  updateBatchToolbar();
}

function selectInvalidSongs() {
  selectInvalidSongCards();
}

function getSelectedSongs() {
  const checkedBoxes = document.querySelectorAll(".song-checkbox:checked");
  const songs = [];
  checkedBoxes.forEach((cb) => {
    const card = cb.closest(".song-card");
    if (card) {
      const song = songFromCard(card);
      if (!song) return;
      const lyricURLs = lyricURLsForSong(song);

      songs.push({
        id: song.id,
        source: song.source,
        name: song.name,
        artist: song.artist,
        duration: song.duration,
        extra: song.extra,
        url: buildDownloadURL(
          song.id,
          song.source,
          song.name,
          song.artist,
          song.album || "",
          song.cover || "",
          song.extra || "",
        ),
        cover: song.cover,
        lrc: lyricURLs.line,
        raw_lrc: lyricURLs.auto,
        theme: "#10b981",
      });
    }
  });
  return songs;
}

// ==========================================
// 下载中面板
// ==========================================

let downloadPanelItems = [];

function showDownloadPanel(songs) {
  closeSwitchPanel();
  const panel = document.getElementById("download-panel");
  const body = document.getElementById("download-panel-body");
  const footer = document.getElementById("download-panel-footer");
  if (!panel || !body || !footer) return;

  downloadPanelItems = songs.map((s, i) => ({
    index: i,
    name: s.name || "",
    artist: s.artist || "",
    status: "wait", // wait / loading / success / skipped / failed
    msg: "",
  }));

  body.innerHTML = downloadPanelItems
    .map(
      (item) => `
    <div class="download-panel-item" id="dp-item-${item.index}">
      <span class="dp-icon dp-wait"><i class="fa-regular fa-circle"></i></span>
      <span>${escapeHtml(item.artist)} - ${escapeHtml(item.name)}</span>
    </div>`,
    )
    .join("");

  footer.textContent = `共 ${songs.length} 首 · 完成 0/${songs.length}`;
  panel.style.display = "flex";
}

function updateDownloadPanelItem(index, status, msg) {
  const item = downloadPanelItems[index];
  if (!item) return;
  item.status = status;
  item.msg = msg || "";

  const el = document.getElementById(`dp-item-${index}`);
  if (!el) return;

  const iconMap = {
    wait: '<i class="fa-regular fa-circle"></i>',
    loading: '<i class="fa-solid fa-spinner fa-spin"></i>',
    success: '<i class="fa-solid fa-check-circle"></i>',
    skipped: '<i class="fa-solid fa-forward-step"></i>',
    failed: '<i class="fa-solid fa-circle-exclamation"></i>',
  };
  const clsMap = {
    wait: "dp-wait",
    loading: "dp-loading",
    success: "dp-success",
    skipped: "dp-skipped",
    failed: "dp-failed",
  };

  const icon = el.querySelector(".dp-icon");
  if (icon) {
    icon.className = `dp-icon ${clsMap[status] || "dp-wait"}`;
    icon.innerHTML = iconMap[status] || iconMap.wait;
  }

  // 正在下载的项滚动到面板中间
  if (status === "loading") {
    el.scrollIntoView({ block: "center", behavior: "smooth" });
  }

  // 更新底部统计
  const done = downloadPanelItems.filter(
    (x) => x.status === "success" || x.status === "skipped" || x.status === "failed",
  ).length;
  const footer = document.getElementById("download-panel-footer");
  if (footer) {
    footer.textContent = `共 ${downloadPanelItems.length} 首 · 完成 ${done}/${downloadPanelItems.length}`;
  }
}

function closeDownloadPanel() {
  const panel = document.getElementById("download-panel");
  if (panel) panel.style.display = "none";
}

// ==========================================
// 换源中面板
// ==========================================

let switchPanelItems = [];

function showSwitchPanel(cards) {
  closeDownloadPanel();
  const panel = document.getElementById("switch-panel");
  const body = document.getElementById("switch-panel-body");
  const footer = document.getElementById("switch-panel-footer");
  if (!panel || !body || !footer) return;

  switchPanelItems = cards.map((card, i) => ({
    index: i,
    name: card.dataset.name || "",
    artist: card.dataset.artist || "",
    status: "wait",
  }));

  body.innerHTML = switchPanelItems
    .map(
      (item) => `
    <div class="download-panel-item" id="sp-item-${item.index}">
      <span class="dp-icon dp-wait"><i class="fa-regular fa-circle"></i></span>
      <span>${escapeHtml(item.artist)} - ${escapeHtml(item.name)}</span>
    </div>`,
    )
    .join("");

  footer.textContent = `共 ${cards.length} 首 · 完成 0/${cards.length}`;
  panel.style.display = "flex";
}

function updateSwitchPanelItem(index, status) {
  const item = switchPanelItems[index];
  if (!item) return;
  item.status = status;

  const el = document.getElementById(`sp-item-${index}`);
  if (!el) return;

  const iconMap = {
    wait: '<i class="fa-regular fa-circle"></i>',
    loading: '<i class="fa-solid fa-spinner fa-spin"></i>',
    success: '<i class="fa-solid fa-check-circle"></i>',
    failed: '<i class="fa-solid fa-circle-exclamation"></i>',
  };
  const clsMap = {
    wait: "dp-wait",
    loading: "dp-loading",
    success: "dp-success",
    failed: "dp-failed",
  };

  const icon = el.querySelector(".dp-icon");
  if (icon) {
    icon.className = `dp-icon ${clsMap[status] || "dp-wait"}`;
    icon.innerHTML = iconMap[status] || iconMap.wait;
  }

  if (status === "loading") {
    el.scrollIntoView({ block: "center", behavior: "smooth" });
  }

  const done = switchPanelItems.filter(
    (x) => x.status === "success" || x.status === "failed",
  ).length;
  const footer = document.getElementById("switch-panel-footer");
  if (footer) {
    footer.textContent = `共 ${switchPanelItems.length} 首 · 完成 ${done}/${switchPanelItems.length}`;
  }
}

function closeSwitchPanel() {
  const panel = document.getElementById("switch-panel");
  if (panel) panel.style.display = "none";
}


async function batchDownload() {
  // 关闭旧面板，准备新任务
  closeDownloadPanel();

  const selectedSongs = getSelectedSongs();
  const songs = selectedSongs.filter(
    (song) => !isLocalMusicSourceValue(song.source),
  );
  const skippedLocalCount = selectedSongs.length - songs.length;
  if (selectedSongs.length === 0) return;
  if (songs.length === 0) {
    alert("选中的歌曲都是本地歌曲，无需批量下载。");
    return;
  }
  const batchDl = document.getElementById("btn-batch-dl");
  const batchSwitch = document.getElementById("btn-batch-switch");
  const originalBatchDlHTML = batchDl ? batchDl.innerHTML : "";

  const skipText =
    skippedLocalCount > 0 ? `\n已跳过 ${skippedLocalCount} 首本地歌曲。` : "";

  // 预检：统计会跳过的数量
  let precheckSkipText = "";
  try {
    const preResp = await fetch(`${API_ROOT}/api/downloads/precheck`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        songs: songs.map((s) => ({ name: s.name, artist: s.artist })),
      }),
    });
    if (preResp.ok) {
      const preData = await preResp.json();
      if (preData.skipped > 0) {
        precheckSkipText = `\n其中 ${preData.skipped} 首已在本地曲库（将自动跳过）`;
      }
    }
  } catch (_) {}

  if (
    !confirm(
      `准备将 ${songs.length} 首歌曲保存到本地目录:\n${webSettings.downloadDir}${skipText}${precheckSkipText}`,
    )
  ) {
    return;
  }

  if (batchDl) {
    batchDl.disabled = true;
    batchDl.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 下载中';
  }
  if (batchSwitch) {
    batchSwitch.disabled = true;
  }

  // 显示下载中面板
  showDownloadPanel(songs);

  let success = 0;
  let warningCount = 0;
  const failures = [];

  try {
    for (let i = 0; i < songs.length; i++) {
      const song = songs[i];
      updateDownloadPanelItem(i, "loading");
      try {
        const result = await requestLocalDownload(song.url);
        // 判断是否跳过
        if (result && result.skipped) {
          updateDownloadPanelItem(i, "skipped");
        } else {
          updateDownloadPanelItem(i, "success");
          success++;
          if (result && result.warning) {
            warningCount++;
          }
        }
      } catch (error) {
        updateDownloadPanelItem(i, "failed", error && error.message ? error.message : "下载失败");
        failures.push({
          song,
          reason: error && error.message ? error.message : "下载失败",
        });
      }
    }

    // 不再弹出 toast，面板常驻由用户手动关闭

    let message = `本地保存完成，成功 ${success}/${songs.length}`;

    if (skippedLocalCount > 0) {
      message += `\n已跳过 ${skippedLocalCount} 首本地歌曲。`;
    }
    message += `\n目录：${webSettings.downloadDir}`;
    if (warningCount > 0) {
      message += `\n\n共 ${warningCount} 首触发了降级提示，请查看终端日志`;
    }
    message += buildBatchFailureMessage(failures, "失败");

    showToast(
      failures.length > 0 ? "下载部分完成" : "下载完成",
      message,
      failures.length > 0 ? "warning" : "success",
      0,
    );
  } finally {
    if (batchDl) {
      batchDl.innerHTML = originalBatchDlHTML;
    }
    updateBatchToolbar();
    if (
      batchSwitch &&
      document.querySelectorAll(".song-checkbox:checked").length === 0
    ) {
      batchSwitch.disabled = true;
    }
  }
}

async function deleteLocalMusic(trackId) {
  const id = String(trackId || "").trim();
  if (!id) {
    throw new Error("缺少本地音乐 ID");
  }

  const response = await fetch(
    `${API_ROOT}/local_music?id=${encodeURIComponent(id)}`,
    {
      method: "DELETE",
    },
  );
  const payload = await response.json().catch(() => null);
  if (!response.ok || !payload || payload.error) {
    throw new Error((payload && payload.error) || "删除失败");
  }
  return payload;
}

function confirmLocalMusicDeletion(songs) {
  const items = Array.isArray(songs) ? songs : [];
  if (items.length === 0) return false;

  const names = items.slice(0, 5).map(formatBatchSongLabel).join("\n");
  const more = items.length > 5 ? `\n...等 ${items.length} 首` : "";
  const scope =
    items.length === 1
      ? `《${formatBatchSongLabel(items[0])}》`
      : `${items.length} 首本地音乐`;
  if (
    !confirm(
      `准备删除 ${scope}。\n删除后文件会从本地下载目录移除，且不可恢复。\n\n${names}${more}`,
    )
  ) {
    return false;
  }
  return confirm(`再次确认：确定永久删除 ${scope} 吗？`);
}

function stopDeletedLocalMusicPlayback(deletedIds) {
  if (!currentPlayingId || !deletedIds || !deletedIds.has(currentPlayingId))
    return;
  try {
    ap?.pause();
    ap?.list?.clear();
  } catch (_) {}
  currentPlayingId = null;
  highlightCard(null);
  syncAllPlayButtons();
  syncMediaSession();
}

async function deleteLocalMusicFromButton(btn) {
  const card = btn?.closest(".song-card");
  const song = songFromCard(card);
  if (!song || !isLocalMusicSourceValue(song.source)) return;
  if (!confirmLocalMusicDeletion([song])) return;

  const originalHTML = btn.innerHTML;
  btn.disabled = true;
  btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i>';

  try {
    await deleteLocalMusic(song.id);
    stopDeletedLocalMusicPlayback(new Set([song.id]));
    await refreshLocalMusicPageAfterMutation();
  } catch (error) {
    alert(error.message || "删除失败");
  } finally {
    if (btn.isConnected) {
      btn.innerHTML = originalHTML;
      btn.disabled = false;
    }
  }
}

async function batchDeleteLocalMusic() {
  const songs = getSelectedSongs().filter((song) =>
    isLocalMusicSourceValue(song.source),
  );
  if (songs.length === 0) return;

  if (!confirmLocalMusicDeletion(songs)) {
    return;
  }

  const batchDelete = document.getElementById("btn-batch-delete-local");
  const originalHTML = batchDelete ? batchDelete.innerHTML : "";
  if (batchDelete) {
    batchDelete.disabled = true;
    batchDelete.innerHTML =
      '<i class="fa-solid fa-spinner fa-spin"></i> 删除中';
  }

  let success = 0;
  const failures = [];
  const deletedIds = new Set();

  try {
    for (const song of songs) {
      try {
        await deleteLocalMusic(song.id);
        success++;
        deletedIds.add(song.id);
      } catch (error) {
        failures.push({
          song,
          reason: error && error.message ? error.message : "删除失败",
        });
      }
    }

    stopDeletedLocalMusicPlayback(deletedIds);

    let message = `批量删除完成，成功 ${success}/${songs.length}`;
    message += buildBatchFailureMessage(failures, "失败");
    alert(message);
    await refreshLocalMusicPageAfterMutation();
  } finally {
    if (batchDelete) {
      batchDelete.innerHTML = originalHTML;
    }
    updateBatchToolbar();
  }
}

async function batchSwitchSource(options = {}) {
  const optionCards = Array.isArray(options.cards)
    ? options.cards.filter((card) => card && card.isConnected)
    : null;
  const checkedBoxes = optionCards
    ? optionCards
        .map((card) => card.querySelector(".song-checkbox"))
        .filter(Boolean)
    : Array.from(document.querySelectorAll(".song-checkbox:checked"));
  if (checkedBoxes.length === 0) return false;

  const candidateCards =
    optionCards || checkedBoxes.map((cb) => cb.closest(".song-card"));
  const cards = candidateCards.filter(
    (card) => card && !isLocalMusicSourceValue(card.dataset.source),
  );
  const skippedLocalCount = candidateCards.length - cards.length;
  if (cards.length === 0) {
    if (!options.silent) {
      alert("选中的歌曲都是本地歌曲，无需批量换源。");
    }
    return false;
  }

  const skipText =
    skippedLocalCount > 0 ? `\n已跳过 ${skippedLocalCount} 首本地歌曲。` : "";
  if (
    !options.skipConfirm &&
    !confirm(
      `准备对 ${cards.length} 首歌曲进行自动换源。\n这可能需要一些时间，请耐心等待。${skipText}`,
    )
  ) {
    return false;
  }

  const batchSwitch = document.getElementById("btn-batch-switch");
  const originalHTML = batchSwitch ? batchSwitch.innerHTML : "";
  if (batchSwitch) {
    batchSwitch.disabled = true;
    batchSwitch.innerHTML =
      '<i class="fa-solid fa-spinner fa-spin"></i> 换源中';
  }

  // 显示换源中面板
  showSwitchPanel(cards);

  const concurrency = Math.min(3, cards.length);
  let nextIndex = 0;
  const runWorker = async () => {
    while (nextIndex < cards.length) {
      const idx = nextIndex++;
      const card = cards[idx];
      updateSwitchPanelItem(idx, "loading");
      const switchBtn = card.querySelector(".btn-switch");
      if (switchBtn) {
        const ok = await switchSource(switchBtn, {
          silent: !!options.silent,
          deferToolbar: true,
        });
        updateSwitchPanelItem(idx, ok ? "success" : "failed");
      } else {
        updateSwitchPanelItem(idx, "failed");
      }
    }
  };
  try {
    await Promise.all(Array.from({ length: concurrency }, runWorker));
    return true;
  } finally {
    if (batchSwitch) {
      batchSwitch.innerHTML = originalHTML;
    }
    updateBatchToolbar();
  }
}

async function batchRemoveFromCollection(colId) {
  const id = String(colId || "").trim();
  const songs = getSelectedSongs();
  if (!id || songs.length === 0) return;

  if (!confirm(`确定从当前自建歌单中取消收藏 ${songs.length} 首歌曲吗？`)) {
    return;
  }

  const btn = document.getElementById("btn-batch-remove-collection");
  const originalHTML = btn ? btn.innerHTML : "";
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 移出中';
  }

  try {
    const response = await fetch(
      `${API_ROOT}/collections/${encodeURIComponent(id)}/songs`,
      {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          songs: songs.map((song) => ({ id: song.id, source: song.source })),
        }),
      },
    );
    const payload = await response.json().catch(() => null);
    if (!response.ok || !payload || payload.error) {
      throw new Error((payload && payload.error) || "批量取消收藏失败");
    }
    alert(`已从当前歌单取消收藏 ${songs.length} 首歌曲。`);
    await refreshCurrentPageContent({ scroll: false });
  } catch (error) {
    alert(error.message || "批量取消收藏失败");
  } finally {
    if (btn && btn.isConnected) {
      btn.innerHTML = originalHTML;
    }
    updateBatchToolbar();
  }
}

// ==========================================
// 自制歌单（本地收藏夹）前端交互
// ==========================================

let pendingFavSong = null;
let activeLocalMusicCollectionId = "";
let localMusicModalState = {
  offset: 0,
  limit: 80,
  total: 0,
  hasMore: false,
  loading: false,
};

function localMusicElements() {
  return {
    modal: document.getElementById("localMusicModal"),
    list: document.getElementById("localMusicList"),
    hint: document.getElementById("localMusicHint"),
    dir: document.getElementById("localMusicDir"),
    targetWrap: document.getElementById("localMusicTargetWrap"),
    targetSelect: document.getElementById("localMusicCollectionSelect"),
    input: document.getElementById("localMusicUploadInput"),
  };
}

function setLocalMusicHint(message, isError = false) {
  const { hint } = localMusicElements();
  if (!hint) return;
  hint.textContent = message || "";
  hint.classList.toggle("error", !!isError);
  hint.style.display = message ? "block" : "none";
}

async function openLocalMusicModal(collectionId = "") {
  activeLocalMusicCollectionId = String(collectionId || "").trim();
  const { modal, list, dir, targetWrap } = localMusicElements();
  if (!modal) return;

  if (list) list.innerHTML = "";
  if (dir) dir.textContent = `下载目录：${webSettings.downloadDir || "-"}`;
  setLocalMusicHint("正在加载本地音乐...");
  modal.style.display = "flex";

  if (activeLocalMusicCollectionId) {
    if (targetWrap) targetWrap.style.display = "none";
  } else {
    if (targetWrap) targetWrap.style.display = "block";
    const loaded = await loadLocalMusicCollections();
    if (!loaded) return;
  }

  refreshLocalMusicList();
}

function closeLocalMusicModal() {
  const { modal } = localMusicElements();
  if (modal) modal.style.display = "none";
}

async function loadLocalMusicCollections() {
  const { targetSelect, list } = localMusicElements();
  if (!targetSelect) return false;

  try {
    const response = await fetch(API_ROOT + "/collections");
    const collections = await response.json().catch(() => []);
    if (!response.ok || !Array.isArray(collections)) {
      throw new Error("歌单列表加载失败");
    }

    targetSelect.innerHTML = "";
    collections.forEach((col) => {
      const option = document.createElement("option");
      option.value = String(col.id || "");
      option.textContent = col.name || `歌单 ${col.id}`;
      targetSelect.appendChild(option);
    });

    if (collections.length === 0) {
      activeLocalMusicCollectionId = "";
      if (list) list.innerHTML = "";
      setLocalMusicHint("请先新建一个自制歌单，再添加本地音乐。");
      return false;
    }

    activeLocalMusicCollectionId = String(collections[0].id || "");
    targetSelect.value = activeLocalMusicCollectionId;
    return true;
  } catch (error) {
    activeLocalMusicCollectionId = "";
    if (list) list.innerHTML = "";
    setLocalMusicHint(error.message || "歌单列表加载失败", true);
    return false;
  }
}

function selectLocalMusicCollection(collectionId) {
  activeLocalMusicCollectionId = String(collectionId || "").trim();
  if (!activeLocalMusicCollectionId) {
    setLocalMusicHint("请选择一个目标歌单。");
    return;
  }
  setLocalMusicHint("正在加载本地音乐...");
  refreshLocalMusicList();
}

async function refreshLocalMusicList(options = {}) {
  const { list, dir } = localMusicElements();
  if (!list) return;
  if (!activeLocalMusicCollectionId) {
    list.innerHTML = "";
    setLocalMusicHint("请选择一个目标歌单。");
    return;
  }
  if (localMusicModalState.loading) return;

  const append = !!options.append;
  if (!append) {
    localMusicModalState.offset = 0;
    localMusicModalState.total = 0;
    localMusicModalState.hasMore = false;
  }

  const params = new URLSearchParams({
    collection_id: activeLocalMusicCollectionId,
    offset: String(localMusicModalState.offset),
    limit: String(localMusicModalState.limit),
  });
  try {
    localMusicModalState.loading = true;
    const response = await fetch(
      `${API_ROOT}/local_music?${params.toString()}`,
    );
    const payload = await response.json().catch(() => null);
    if (!response.ok || !payload || payload.error) {
      throw new Error((payload && payload.error) || "加载失败");
    }
    if (dir) {
      dir.textContent = `下载目录：${payload.download_dir || webSettings.downloadDir || "-"}`;
    }
    renderLocalMusicList(payload, append);
  } catch (error) {
    if (!append) {
      list.innerHTML = "";
    }
    setLocalMusicHint(error.message || "加载本地音乐失败", true);
  } finally {
    localMusicModalState.loading = false;
  }
}

function localMusicMissingText(track) {
  const labels = { title: "标题", artist: "歌手", album: "专辑" };
  const missing = Array.isArray(track?.missing) ? track.missing : [];
  if (missing.length === 0) return "";
  return `缺少${missing.map((key) => labels[key] || key).join("、")}`;
}

function renderLocalMusicList(payload, append = false) {
  const { list } = localMusicElements();
  if (!list) return;

  const tracks = Array.isArray(payload.tracks) ? payload.tracks : [];
  localMusicModalState.total = parsePositiveInt(payload.total, 0);
  localMusicModalState.offset =
    parsePositiveInt(payload.offset, 0) + tracks.length;
  localMusicModalState.hasMore = !!payload.has_more;

  if (!append) {
    list.innerHTML = "";
  } else {
    list.querySelector(".local-music-load-more")?.remove();
  }

  if (!payload.exists) {
    setLocalMusicHint("下载目录还不存在。上传音乐后会自动创建该目录。");
    return;
  }
  if (tracks.length === 0) {
    setLocalMusicHint(
      "下载目录里还没有支持的音频文件，可上传 mp3、flac、m4a、ogg、wav、wma、aac。",
    );
    return;
  }

  setLocalMusicHint("");
  tracks.forEach((track) => {
    const item = document.createElement("div");
    item.className = "local-music-item";

    const title = track.name || track.filename || "未命名音乐";
    const artist = track.artist || "未知歌手";
    const album = track.album || "未知专辑";
    const sizeText = track.size_text || "-";
    const missingText = localMusicMissingText(track);

    item.innerHTML = `
            <div class="local-music-cover"><i class="fa-solid fa-music"></i></div>
            <div class="local-music-main">
                <div class="local-music-title" title="${escapeHTML(title)}">${escapeHTML(title)}</div>
                <div class="local-music-meta">
                    <span>${escapeHTML(artist)}</span>
                    <span>·</span>
                    <span>${escapeHTML(album)}</span>
                    <span>·</span>
                    <span>${escapeHTML(sizeText)}</span>
                    ${missingText ? `<span class="local-music-missing">${escapeHTML(missingText)}</span>` : ""}
                </div>
                <div class="local-music-file" title="${escapeHTML(track.rel_path || track.filename || "")}">
                    ${escapeHTML(track.rel_path || track.filename || "")}
                </div>
            </div>
            <button type="button" class="btn-pill btn-pill-primary local-music-add ${track.already_added ? "is-added" : ""}" ${track.already_added ? "disabled" : ""}>
                <i class="fa-solid ${track.already_added ? "fa-check" : "fa-plus"}"></i> ${track.already_added ? "已添加" : "添加"}
            </button>
        `;

    const btn = item.querySelector(".local-music-add");
    if (btn && !track.already_added) {
      btn.onclick = () => addLocalMusicToCollection(track.id, btn);
    }
    list.appendChild(item);
  });

  if (localMusicModalState.hasMore) {
    const more = document.createElement("button");
    more.type = "button";
    more.className = "btn-pill btn-pill-dl local-music-load-more";
    more.innerHTML = '<i class="fa-solid fa-chevron-down"></i> 加载更多';
    more.onclick = () => refreshLocalMusicList({ append: true });
    list.appendChild(more);
    setLocalMusicHint(
      `已显示 ${localMusicModalState.offset} / ${localMusicModalState.total} 首`,
    );
  } else {
    setLocalMusicHint("");
  }
}

async function uploadLocalMusicFile(input) {
  if (!input || !input.files || input.files.length === 0) return;

  const file = input.files[0];
  const formData = new FormData();
  formData.append("file", file);
  setLocalMusicHint(`正在上传：${file.name}`);

  try {
    const response = await fetch(`${API_ROOT}/local_music/upload`, {
      method: "POST",
      body: formData,
    });
    const payload = await response.json().catch(() => null);
    if (!response.ok || !payload || payload.error) {
      throw new Error((payload && payload.error) || "上传失败");
    }
    input.value = "";
    await refreshLocalMusicList();
  } catch (error) {
    setLocalMusicHint(error.message || "上传失败", true);
  }
}

async function uploadLocalMusicForPage(input) {
  if (!input || !input.files || input.files.length === 0) return;

  const file = input.files[0];
  const formData = new FormData();
  formData.append("file", file);

  try {
    const response = await fetch(`${API_ROOT}/local_music/upload`, {
      method: "POST",
      body: formData,
    });
    const payload = await response.json().catch(() => null);
    if (!response.ok || !payload || payload.error) {
      throw new Error((payload && payload.error) || "上传失败");
    }
    input.value = "";
    await refreshCurrentPageContent({ scroll: false });
  } catch (error) {
    input.value = "";
    alert(error.message || "上传失败");
  }
}

async function addLocalMusicToCollection(trackId, btn) {
  if (!activeLocalMusicCollectionId || !trackId) return;

  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 添加中';
  }

  try {
    const response = await fetch(
      `${API_ROOT}/collections/${activeLocalMusicCollectionId}/local_music`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: trackId }),
      },
    );
    const payload = await response.json().catch(() => null);
    if (!response.ok || !payload || payload.error) {
      throw new Error((payload && payload.error) || "添加失败");
    }

    if (btn) {
      btn.classList.add("is-added");
      btn.innerHTML = '<i class="fa-solid fa-check"></i> 已添加';
    }
    await refreshCurrentPageContent({ scroll: false });
    await refreshLocalMusicList();
  } catch (error) {
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = '<i class="fa-solid fa-plus"></i> 添加';
    }
    alert(error.message || "添加失败");
  }
}

function playAllSongs() {
  const firstPlayBtn = document.querySelector(".song-card .btn-play");
  if (firstPlayBtn) {
    playAllAndJumpTo(firstPlayBtn);
  } else {
    alert("列表为空，无法播放");
  }
}

let pendingBatchFavIds = [];
let pendingBatchFavSongs = [];

function batchAddToCollection() {
  const songs = getSelectedSongs();
  if (songs.length === 0) {
    alert("请先选择歌曲");
    return;
  }
  pendingBatchFavSongs = songs
    .map((song) => ({
      id: song.id,
      source: song.source,
      name: song.name,
      artist: song.artist,
      cover: song.cover,
      duration: song.duration,
      extra: song.extra,
    }))
    .filter((song) => song.id && song.source);
  pendingBatchFavIds = [];
  pendingFavSong = null;
  document.getElementById("addToCollectionModal").style.display = "flex";
  refreshAddToCollectionList();
}

async function submitBatchAddToCollection(colId) {
  const songs = pendingBatchFavSongs.slice();
  pendingBatchFavSongs = [];
  if (!colId || songs.length === 0) return;

  try {
    const response = await fetch(
      `${API_ROOT}/collections/${colId}/songs/batch`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ songs }),
      },
    );
    const payload = await response.json().catch(() => null);
    if (!response.ok || !payload || payload.error) {
      throw new Error((payload && payload.error) || "批量收藏失败");
    }
    document.getElementById("addToCollectionModal").style.display = "none";
    let message = `批量收藏完成：新增 ${payload.added || 0}`;
    if (payload.duplicate) message += `，已存在 ${payload.duplicate}`;
    if (payload.failed) message += `，失败 ${payload.failed}`;
    alert(message);
  } catch (error) {
    alert(error.message || "批量收藏失败");
  }
}

function batchAddLocalMusicToCollection() {
  const songs = getSelectedSongs().filter((song) =>
    isLocalMusicSourceValue(song.source),
  );
  if (songs.length === 0) {
    alert("请先选择本地音乐");
    return;
  }
  pendingBatchFavIds = songs.map((song) => song.id).filter(Boolean);
  pendingBatchFavSongs = [];
  pendingFavSong = null;
  document.getElementById("addToCollectionModal").style.display = "flex";
  refreshAddToCollectionList();
}

async function submitBatchAddLocalMusicToCollection(colId) {
  const ids = pendingBatchFavIds.slice();
  pendingBatchFavIds = [];
  if (!colId || ids.length === 0) return;

  try {
    const response = await fetch(
      `${API_ROOT}/collections/${colId}/local_music/batch`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ids }),
      },
    );
    const payload = await response.json().catch(() => null);
    if (!response.ok || !payload || payload.error) {
      throw new Error((payload && payload.error) || "批量收藏失败");
    }
    document.getElementById("addToCollectionModal").style.display = "none";
    let message = `批量收藏完成：新增 ${payload.added || 0}`;
    if (payload.duplicate) message += `，已存在 ${payload.duplicate}`;
    if (payload.failed) message += `，失败 ${payload.failed}`;
    alert(message);
    await refreshCurrentPageContent({ scroll: false });
  } catch (error) {
    alert(error.message || "批量收藏失败");
  }
}

function openCollectionManager() {
  navigateTo(API_ROOT + "/my_collections");
}

function openLocalMusicPage() {
  navigateTo(API_ROOT + "/local_music_page");
}

function showEditCollectionModal(id = "", name = "", desc = "", cover = "") {
  document.getElementById("editColTitle").textContent = id
    ? "编辑歌单"
    : "新建歌单";
  document.getElementById("editColId").value = id;
  document.getElementById("editColName").value = name;
  document.getElementById("editColDesc").value = desc;

  if (cover && cover.includes("picsum.photos")) {
    document.getElementById("editColCover").value = "";
  } else {
    document.getElementById("editColCover").value = cover;
  }

  document.getElementById("editCollectionModal").style.display = "flex";
}

function showEditCollectionModalFromButton(btn) {
  if (!btn) return;
  showEditCollectionModal(
    btn.dataset.id || "",
    btn.dataset.name || "",
    btn.dataset.description || "",
    btn.dataset.cover || "",
  );
}

function saveCollection() {
  const id = document.getElementById("editColId").value;
  const name = document.getElementById("editColName").value.trim();
  const desc = document.getElementById("editColDesc").value.trim();
  const cover = document.getElementById("editColCover").value.trim();

  if (!name) return alert("名称不能为空");

  const payload = { name, description: desc, cover };
  const isAddingSongModalOpen =
    document.getElementById("addToCollectionModal").style.display === "flex";

  const url = id ? `${API_ROOT}/collections/${id}` : `${API_ROOT}/collections`;
  const method = id ? "PUT" : "POST";

  fetch(url, {
    method: method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  })
    .then((r) => r.json())
    .then((res) => {
      if (res.error) return alert(res.error);

      document.getElementById("editCollectionModal").style.display = "none";

      if (isAddingSongModalOpen) {
        refreshAddToCollectionList();
      } else {
        refreshCurrentPageContent();
      }
    });
}

function setImportCollectionButtonState(btn, imported) {
  if (!btn) return;

  btn.disabled = !!imported;
  if (imported) {
    btn.innerHTML = '<i class="fa-solid fa-check"></i> 已导入';
    btn.style.opacity = "0.85";
  } else {
    btn.innerHTML = '<i class="fa-solid fa-download"></i> 导入本地';
    btn.style.opacity = "";
  }
}

function importCollectionFromButton(btn) {
  if (!btn) return;

  const payload = {
    name: btn.dataset.name || "",
    description: btn.dataset.description || "",
    cover: btn.dataset.cover || "",
    creator: btn.dataset.creator || "",
    track_count: parsePositiveInt(btn.dataset.trackCount, 0),
    source: btn.dataset.source || "",
    external_id: btn.dataset.externalId || "",
    link: btn.dataset.link || "",
    content_type: btn.dataset.contentType || "playlist",
  };

  if (!payload.source || !payload.external_id) {
    alert("缺少导入参数");
    return;
  }

  btn.disabled = true;
  btn.style.opacity = "0.6";

  fetch(`${API_ROOT}/collections/import`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  })
    .then((r) => r.json())
    .then((res) => {
      if (res.error) {
        btn.disabled = false;
        btn.style.opacity = "";
        alert(res.error);
        return;
      }

      setImportCollectionButtonState(btn, true);
      alert(
        res.duplicate
          ? "该歌单/专辑已在本地列表中"
          : "导入成功，已加入本地歌单列表",
      );
    })
    .catch(() => {
      btn.disabled = false;
      btn.style.opacity = "";
      alert("导入失败，请稍后重试");
    });
}

function deleteCollection(id) {
  if (!confirm("确定删除此歌单吗？内部歌曲记录也将被清空。")) return;
  fetch(`${API_ROOT}/collections/${id}`, { method: "DELETE" })
    .then((r) => r.json())
    .then((res) => {
      if (res.error) return alert(res.error);
      refreshCurrentPageContent();
    });
}

function deleteCollectionFromModal(id) {
  if (!confirm("确定删除此歌单吗？内部歌曲记录也将被清空。")) return;
  fetch(`${API_ROOT}/collections/${id}`, { method: "DELETE" })
    .then((r) => r.json())
    .then((res) => {
      if (res.error) return alert(res.error);
      refreshAddToCollectionList();
    });
}

function refreshAddToCollectionList() {
  const container = document.getElementById("addColList");
  container.innerHTML =
    '<div style="text-align: center; color: #a0aec0; padding: 20px;">加载中...</div>';

  fetch(API_ROOT + "/collections")
    .then((r) => r.json())
    .then((data) => {
      if (!data || data.length === 0) {
        container.innerHTML =
          '<div style="text-align: center; color: #a0aec0; padding: 20px;">暂无歌单，请点击上方“新建”创建</div>';
        return;
      }
      container.innerHTML = "";
      data.forEach((col) => {
        const item = document.createElement("div");
        item.className = "collection-item";
        item.style.cursor = "default";

        let cvr = col.cover;
        if (!cvr) cvr = `https://picsum.photos/seed/col_${col.id}/400/400`;

        item.innerHTML = `
                    <div class="col-clickable-area" style="display:flex; align-items:center; flex:1; overflow:hidden; cursor:pointer;" title="收藏到此歌单">
                        <img src="${cvr}" style="width:40px;height:40px;border-radius:6px;object-fit:cover;margin-right:12px;">
                        <div class="collection-name" style="margin:0; font-size:14px; white-space:nowrap; overflow:hidden; text-overflow:ellipsis;">${col.name}</div>
                    </div>
                    <div style="display:flex; gap:6px; margin-left: 10px;">
                        <button class="col-action-btn btn-edit" title="编辑歌单"><i class="fa-solid fa-pen"></i></button>
                        <button class="col-action-btn del btn-del" title="删除歌单"><i class="fa-solid fa-trash"></i></button>
                    </div>
                `;

        item.querySelector(".col-clickable-area").onclick = () => {
          if (pendingBatchFavIds && pendingBatchFavIds.length > 0) {
            submitBatchAddLocalMusicToCollection(col.id);
          } else if (pendingBatchFavSongs && pendingBatchFavSongs.length > 0) {
            submitBatchAddToCollection(col.id);
          } else {
            addSongToCollection(col.id);
          }
        };
        item.querySelector(".btn-edit").onclick = (e) => {
          e.stopPropagation();
          showEditCollectionModal(
            col.id,
            col.name,
            col.description || "",
            col.cover || "",
          );
        };
        item.querySelector(".btn-del").onclick = (e) => {
          e.stopPropagation();
          deleteCollectionFromModal(col.id);
        };

        container.appendChild(item);
      });
    })
    .catch(() => {
      container.innerHTML =
        '<div style="text-align: center; color: #e53e3e; padding: 20px;">加载失败</div>';
    });
}

function openAddToCollectionModal(btn) {
  const card = btn.closest(".song-card");
  if (!card) return;

  let coverUrl = card.dataset.cover || "";
  const imgEl = card.querySelector(".cover-wrapper img");
  if (imgEl && imgEl.src) coverUrl = imgEl.src;

  let extra = {};
  try {
    const parsed = JSON.parse(card.dataset.extra || "{}");
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      extra = parsed;
    }
  } catch (_) {}
  extra.saved_from = "web_ui";

  pendingFavSong = {
    id: card.dataset.id,
    source: card.dataset.source,
    name: card.dataset.name,
    artist: card.dataset.artist,
    duration: parseInt(card.dataset.duration) || 0,
    cover: coverUrl,
    extra: extra,
  };
  pendingBatchFavIds = [];
  pendingBatchFavSongs = [];

  document.getElementById("addToCollectionModal").style.display = "flex";
  refreshAddToCollectionList();
}

function addSongToCollection(colId) {
  if (!pendingFavSong) return;

  fetch(`${API_ROOT}/collections/${colId}/songs`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(pendingFavSong),
  })
    .then((r) => r.json())
    .then((res) => {
      if (res.error) {
        alert(res.error);
      } else {
        alert("成功收藏至您的歌单！");
        document.getElementById("addToCollectionModal").style.display = "none";
      }
    });
}

function removeSongFromCollection(btn, colId, originalSongId, originalSource) {
  if (!confirm("确定将此歌曲移出当前歌单吗？")) return;
  fetch(
    `${API_ROOT}/collections/${colId}/songs?id=${encodeURIComponent(originalSongId)}&source=${encodeURIComponent(originalSource)}`,
    { method: "DELETE" },
  )
    .then((r) => r.json())
    .then((res) => {
      if (res.error) return alert(res.error);
      const card = btn.closest(".song-card");
      if (card) {
        card.style.transition = "all 0.3s";
        card.style.opacity = "0";
        card.style.transform = "translateX(30px)";
        setTimeout(() => {
          refreshCurrentPageContent();
        }, 300);
      } else {
        refreshCurrentPageContent();
      }
    });
}
