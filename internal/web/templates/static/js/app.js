// templates/app.js

const API_ROOT = window.API_ROOT;
const WEB_SETTINGS_KEY = "musicdl:web_settings";
const INSPECT_REQUEST_DELAY_MS = 20;
const INSPECT_MAX_CONCURRENCY = 50;
const INSPECT_SOURCE_MAX_CONCURRENCY = 32;
const INSPECT_SOURCE_MIN_CONCURRENCY = 4;
const INSPECT_RETRY_BASE_DELAY_MS = 1200;
const INSPECT_RETRY_MAX_DELAY_MS = 30000;
const INSPECT_MAX_RETRIES = 2;
const AUTO_SWITCH_INVALID_DELAY_MS = 500;
const DEFAULT_WEB_PAGE_SIZE = 50;
const DEFAULT_CLI_PAGE_SIZE = 20;
const LOCAL_MUSIC_SOURCE = "local";
const LEGACY_LOCAL_MUSIC_SOURCE = "local-file";
const DOWNLOAD_DIR_CUSTOM_VALUE = "__custom__";
const DOWNLOAD_DIR_PRESET_VALUES = [
  "data/downloads",
  "downloads",
  "D:/Music/Downloads",
  "/home/appuser/data",
  "/sdcard/Music",
  "/storage/emulated/0/Music",
  "/sdcard/Download",
];
const DOWNLOAD_DIR_PRESETS = new Set(DOWNLOAD_DIR_PRESET_VALUES);
const DEFAULT_UPDATE_REPO_URL = "https://github.com/guohuiyuan/go-music-dl";
const DEFAULT_GITHUB_PROXY_URL = "https://edgeone.gh-proxy.com";
const DEFAULT_DOWNLOAD_TIP_DURATION = 8;
const OPEN_CONFIG_QUERY = "open_config";
const GITHUB_PROXY_PRESETS = [
  "https://edgeone.gh-proxy.com",
  "https://hk.gh-proxy.com/",
  "https://gh-proxy.com/",
  "https://gh.llkk.cc",
];
const DEFAULT_SHORTCUT_PLAY_PAUSE = "Space";
const DEFAULT_LYRIC_FONT_FAMILY = "system";
const DEFAULT_LYRIC_FONT_SIZE = 24;
const LYRIC_FONT_FAMILIES = Object.freeze({
  system:
    '-apple-system, BlinkMacSystemFont, "Segoe UI", "Microsoft YaHei", sans-serif',
  sans: '"Segoe UI", "Microsoft YaHei", "PingFang SC", "Noto Sans CJK SC", sans-serif',
  serif: '"Songti SC", SimSun, "Noto Serif CJK SC", serif',
  yahei: '"Microsoft YaHei", "PingFang SC", sans-serif',
  pingfang: '"PingFang SC", "Microsoft YaHei", sans-serif',
  "source-han":
    '"Source Han Sans SC", "Noto Sans CJK SC", "Microsoft YaHei", sans-serif',
  lxgw: '"LXGW WenKai", "KaiTi", "STKaiti", serif',
});

function isLocalMusicSourceValue(source) {
  const value = String(source || "").trim();
  return value === LOCAL_MUSIC_SOURCE || value === LEGACY_LOCAL_MUSIC_SOURCE;
}

let webSettings = {
  embedDownload: true,
  downloadToLocal: true,
  downloadDir: "data/downloads",
  downloadFilenameTemplate: "{name} - {artist}",
  downloadTipDuration: DEFAULT_DOWNLOAD_TIP_DURATION,
  webdavEnabled: false,
  webdavUrl: "",
  webdavUsername: "",
  webdavPassword: "",
  webdavDir: "music-dl",
  disableFloatingLyrics: false,
  webPageSize: DEFAULT_WEB_PAGE_SIZE,
  cliPageSize: DEFAULT_CLI_PAGE_SIZE,
  autoCheckUpdate: true,
  autoSwitchInvalidSources: true,
  autoCacheOnPlay: false,
  filterVersionKeywords: false,
  versionFilterKeywords: "Live, Remix, Demo",
  filterShortTracks: false,
  normalizeChineseVariants: true,
  updateRepoUrl: DEFAULT_UPDATE_REPO_URL,
  githubProxyEnabled: false,
  githubProxyUrl: DEFAULT_GITHUB_PROXY_URL,
  sodaSignerUrl: "",
  sodaSignerToken: "",
  sodaClientDir: "",
  sodaLocalSignerAvailable: false,
  vgChangeCover: false,
  vgChangeAudio: false,
  vgChangeLyric: false,
  vgExportVideo: false,
  shortcutPlayPause: DEFAULT_SHORTCUT_PLAY_PAUSE,
  shortcutPrevious: "",
  shortcutNext: "",
  shortcutSeekBack: "",
  shortcutSeekForward: "",
  lyricFontFamily: DEFAULT_LYRIC_FONT_FAMILY,
  lyricFontSize: DEFAULT_LYRIC_FONT_SIZE,
};

function normalizeWebSettings(raw) {
  const next = {
    embedDownload: true,
    downloadToLocal: true,
    downloadDir: "data/downloads",
    downloadFilenameTemplate: "{name} - {artist}",
    downloadTipDuration: DEFAULT_DOWNLOAD_TIP_DURATION,
    webdavEnabled: false,
    webdavUrl: "",
    webdavUsername: "",
    webdavPassword: "",
    webdavDir: "music-dl",
    disableFloatingLyrics: false,
    webPageSize: DEFAULT_WEB_PAGE_SIZE,
    cliPageSize: DEFAULT_CLI_PAGE_SIZE,
    autoCheckUpdate: true,
    autoSwitchInvalidSources: true,
    autoCacheOnPlay: false,
    filterVersionKeywords: false,
    versionFilterKeywords: "Live, Remix, Demo",
    filterShortTracks: false,
    normalizeChineseVariants: true,
    updateRepoUrl: DEFAULT_UPDATE_REPO_URL,
    githubProxyEnabled: false,
    githubProxyUrl: DEFAULT_GITHUB_PROXY_URL,
    sodaSignerUrl: "",
    sodaSignerToken: "",
    sodaClientDir: "",
    sodaLocalSignerAvailable: false,
    vgChangeCover: false,
    vgChangeAudio: false,
    vgChangeLyric: false,
    vgExportVideo: false,
    shortcutPlayPause: DEFAULT_SHORTCUT_PLAY_PAUSE,
    shortcutPrevious: "",
    shortcutNext: "",
    shortcutSeekBack: "",
    shortcutSeekForward: "",
    lyricFontFamily: DEFAULT_LYRIC_FONT_FAMILY,
    lyricFontSize: DEFAULT_LYRIC_FONT_SIZE,
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
  if (
    Number.isInteger(raw.downloadTipDuration) &&
    raw.downloadTipDuration > 0
  ) {
    next.downloadTipDuration = Math.min(raw.downloadTipDuration, 60);
  }
  if (typeof raw.webdavEnabled === "boolean") {
    next.webdavEnabled = raw.webdavEnabled;
  }
  if (typeof raw.webdavUrl === "string" && raw.webdavUrl.trim() !== "") {
    next.webdavUrl = raw.webdavUrl.trim();
  }
  if (
    typeof raw.webdavUsername === "string" &&
    raw.webdavUsername.trim() !== ""
  ) {
    next.webdavUsername = raw.webdavUsername.trim();
  }
  if (typeof raw.webdavPassword === "string") {
    next.webdavPassword = raw.webdavPassword;
  }
  if (typeof raw.webdavDir === "string" && raw.webdavDir.trim() !== "") {
    next.webdavDir = raw.webdavDir.trim().replace(/^\/+|\/+$/g, "");
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
  if (typeof raw.filterVersionKeywords === "boolean") {
    next.filterVersionKeywords = raw.filterVersionKeywords;
  }
  if (typeof raw.versionFilterKeywords === "string") {
    next.versionFilterKeywords =
      raw.versionFilterKeywords.trim() || "Live, Remix, Demo";
  }
  if (typeof raw.filterShortTracks === "boolean") {
    next.filterShortTracks = raw.filterShortTracks;
  }
  if (typeof raw.normalizeChineseVariants === "boolean") {
    next.normalizeChineseVariants = raw.normalizeChineseVariants;
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
  if (typeof raw.sodaSignerUrl === "string") {
    next.sodaSignerUrl = raw.sodaSignerUrl.trim();
  }
  if (typeof raw.sodaSignerToken === "string") {
    next.sodaSignerToken = raw.sodaSignerToken;
  }
  if (typeof raw.sodaClientDir === "string") {
    next.sodaClientDir = raw.sodaClientDir.trim();
  }
  if (typeof raw.sodaLocalSignerAvailable === "boolean") {
    next.sodaLocalSignerAvailable = raw.sodaLocalSignerAvailable;
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
  [
    "shortcutPlayPause",
    "shortcutPrevious",
    "shortcutNext",
    "shortcutSeekBack",
    "shortcutSeekForward",
  ].forEach((key) => {
    if (typeof raw[key] === "string") {
      next[key] = normalizeShortcutBinding(raw[key]);
    }
  });
  if (
    typeof raw.lyricFontFamily === "string" &&
    Object.prototype.hasOwnProperty.call(
      LYRIC_FONT_FAMILIES,
      raw.lyricFontFamily,
    )
  ) {
    next.lyricFontFamily = raw.lyricFontFamily;
  }
  if (Number.isInteger(raw.lyricFontSize)) {
    next.lyricFontSize = Math.max(14, Math.min(48, raw.lyricFontSize));
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

let activeSettingsTab = "general";

function setSettingsTab(tabName) {
  const tabs = Array.from(document.querySelectorAll("[data-settings-tab]"));
  const panels = Array.from(document.querySelectorAll("[data-settings-panel]"));
  if (!tabs.length || !panels.length) return;

  const requested = String(tabName || "").trim();
  const target = panels.some((panel) => panel.dataset.settingsPanel === requested)
    ? requested
    : "general";
  activeSettingsTab = target;

  tabs.forEach((tab) => {
    const active = tab.dataset.settingsTab === target;
    tab.classList.toggle("is-active", active);
    tab.setAttribute("aria-selected", String(active));
    tab.tabIndex = active ? 0 : -1;
  });
  panels.forEach((panel) => {
    panel.hidden = panel.dataset.settingsPanel !== target;
  });
}

function bindSettingsTabs() {
  const tabs = Array.from(document.querySelectorAll("[data-settings-tab]"));
  if (!tabs.length) return;

  tabs.forEach((tab, index) => {
    if (tab.dataset.bound === "1") return;
    tab.dataset.bound = "1";
    tab.addEventListener("click", () => setSettingsTab(tab.dataset.settingsTab));
    tab.addEventListener("keydown", (event) => {
      if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
      event.preventDefault();
      const offset = event.key === "ArrowRight" ? 1 : -1;
      const next = tabs[(index + offset + tabs.length) % tabs.length];
      next.focus();
      setSettingsTab(next.dataset.settingsTab);
    });
  });
  setSettingsTab(activeSettingsTab);
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

function syncWebDAVVisibility() {
  const fields = document.getElementById("webdav-settings-fields");
  if (fields) {
    fields.style.display = webSettings.webdavEnabled ? "block" : "none";
  }
}

function syncSearchFilterControls() {
  const versionToggle = document.getElementById(
    "setting-filter-version-keywords",
  );
  const keywordInput = document.getElementById(
    "setting-version-filter-keywords",
  );
  if (keywordInput) {
    keywordInput.disabled = !versionToggle?.checked;
  }
}

function bindSearchFilterSettingEvents() {
  const versionToggle = document.getElementById(
    "setting-filter-version-keywords",
  );
  if (versionToggle && versionToggle.dataset.bound !== "1") {
    versionToggle.dataset.bound = "1";
    versionToggle.addEventListener("change", syncSearchFilterControls);
  }
  syncSearchFilterControls();
}

function bindWebDAVSettingEvents() {
  const toggle = document.getElementById("setting-webdav-enabled");
  if (!toggle || toggle.dataset.bound === "1") return;
  toggle.dataset.bound = "1";
  toggle.addEventListener("change", () => {
    const fields = document.getElementById("webdav-settings-fields");
    if (fields) fields.style.display = toggle.checked ? "block" : "none";
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

function normalizeShortcutKey(key) {
  const value = String(key || "").trim();
  if (!value) return "";

  const aliases = {
    space: "Space",
    spacebar: "Space",
    esc: "Escape",
    escape: "Escape",
    enter: "Enter",
    return: "Enter",
    tab: "Tab",
    backspace: "Backspace",
    delete: "Delete",
    del: "Delete",
    insert: "Insert",
    ins: "Insert",
    home: "Home",
    end: "End",
    pageup: "PageUp",
    pgup: "PageUp",
    pagedown: "PageDown",
    pgdn: "PageDown",
    arrowleft: "ArrowLeft",
    left: "ArrowLeft",
    arrowright: "ArrowRight",
    right: "ArrowRight",
    arrowup: "ArrowUp",
    up: "ArrowUp",
    arrowdown: "ArrowDown",
    down: "ArrowDown",
    mediaplaypause: "MediaPlayPause",
    mediatracknext: "MediaTrackNext",
    mediatrackprevious: "MediaTrackPrevious",
    mediastop: "MediaStop",
  };
  const lower = value.toLowerCase();
  if (aliases[lower]) return aliases[lower];

  if (/^key[a-z]$/i.test(value)) return value.slice(3).toUpperCase();
  if (/^digit[0-9]$/i.test(value)) return value.slice(5);
  if (/^f([1-9]|1[0-9]|2[0-4])$/i.test(value)) {
    return value.toUpperCase();
  }
  if (value.length === 1) return value.toUpperCase();
  return value.length <= 32 ? value : "";
}

function normalizeShortcutBinding(binding) {
  const raw = String(binding || "").trim();
  if (!raw) return "";

  const modifiers = new Set();
  let key = "";
  for (const part of raw.split("+")) {
    const value = part.trim();
    if (!value) return "";
    const lower = value.toLowerCase();
    if (lower === "ctrl" || lower === "control") {
      modifiers.add("Ctrl");
    } else if (lower === "alt" || lower === "option") {
      modifiers.add("Alt");
    } else if (lower === "shift") {
      modifiers.add("Shift");
    } else if (
      ["meta", "cmd", "command", "win", "super"].includes(lower)
    ) {
      modifiers.add("Meta");
    } else {
      if (key) return "";
      key = normalizeShortcutKey(value);
      if (!key) return "";
    }
  }
  if (!key) return "";

  return [
    ...["Ctrl", "Alt", "Shift", "Meta"].filter((modifier) =>
      modifiers.has(modifier),
    ),
    key,
  ].join("+");
}

function shortcutBindingFromKeyboardEvent(event) {
  const ignoredKeys = new Set(["Control", "Shift", "Alt", "Meta"]);
  if (ignoredKeys.has(event.key)) return "";

  const key = normalizeShortcutKey(event.code || event.key);
  if (!key || key === "Unidentified") return "";

  const modifiers = [];
  if (event.ctrlKey) modifiers.push("Ctrl");
  if (event.altKey) modifiers.push("Alt");
  if (event.shiftKey) modifiers.push("Shift");
  if (event.metaKey) modifiers.push("Meta");
  modifiers.push(key);
  return normalizeShortcutBinding(modifiers.join("+"));
}

function shortcutUsesPaginationKey(binding) {
  const parts = normalizeShortcutBinding(binding).split("+");
  const key = parts[parts.length - 1];
  return key === "PageUp" || key === "PageDown";
}

function normalizeLyricFontFamily(value) {
  const family = String(value || "").trim();
  return Object.prototype.hasOwnProperty.call(LYRIC_FONT_FAMILIES, family)
    ? family
    : DEFAULT_LYRIC_FONT_FAMILY;
}

function normalizeLyricFontSize(value) {
  const size = Number.parseInt(value, 10);
  if (!Number.isFinite(size)) return DEFAULT_LYRIC_FONT_SIZE;
  return Math.max(14, Math.min(48, size));
}

function lyricFontCSSValue(family) {
  return (
    LYRIC_FONT_FAMILIES[normalizeLyricFontFamily(family)] ||
    LYRIC_FONT_FAMILIES[DEFAULT_LYRIC_FONT_FAMILY]
  );
}

function applyLyricAppearance() {
  const family = lyricFontCSSValue(webSettings.lyricFontFamily);
  const size = normalizeLyricFontSize(webSettings.lyricFontSize);
  document.documentElement.style.setProperty("--lyric-font-family", family);
  document.documentElement.style.setProperty("--lyric-font-size", `${size}px`);
}

function updateLyricFontPreview(family, size) {
  const preview = document.getElementById("setting-lyric-font-preview");
  if (!preview) return;
  preview.style.fontFamily = lyricFontCSSValue(family);
  preview.style.fontSize = `${normalizeLyricFontSize(size)}px`;
}

function syncLyricAppearanceControls() {
  const familyInput = document.getElementById("setting-lyric-font-family");
  const sizeInput = document.getElementById("setting-lyric-font-size");
  const sizeValue = document.getElementById(
    "setting-lyric-font-size-value",
  );
  const family = normalizeLyricFontFamily(webSettings.lyricFontFamily);
  const size = normalizeLyricFontSize(webSettings.lyricFontSize);

  if (familyInput) familyInput.value = family;
  if (sizeInput) sizeInput.value = String(size);
  if (sizeValue) sizeValue.textContent = `${size} px`;
  updateLyricFontPreview(family, size);
  applyLyricAppearance();
}

function bindLyricFontSettingEvents() {
  const familyInput = document.getElementById("setting-lyric-font-family");
  const sizeInput = document.getElementById("setting-lyric-font-size");
  const sizeValue = document.getElementById(
    "setting-lyric-font-size-value",
  );
  if (!familyInput || !sizeInput || sizeInput.dataset.bound === "1") return;

  sizeInput.dataset.bound = "1";
  const preview = () => {
    const size = normalizeLyricFontSize(sizeInput.value);
    if (sizeValue) sizeValue.textContent = `${size} px`;
    updateLyricFontPreview(familyInput.value, size);
  };
  familyInput.addEventListener("change", preview);
  sizeInput.addEventListener("input", preview);
}

const PLAYBACK_SHORTCUT_FIELDS = Object.freeze([
  ["shortcutPlayPause", "setting-shortcut-play-pause", "播放 / 暂停"],
  ["shortcutPrevious", "setting-shortcut-previous", "上一首"],
  ["shortcutNext", "setting-shortcut-next", "下一首"],
  ["shortcutSeekBack", "setting-shortcut-seek-back", "后退 10 秒"],
  ["shortcutSeekForward", "setting-shortcut-seek-forward", "前进 10 秒"],
]);

function setShortcutCaptureStatus(message = "", isError = false) {
  const status = document.getElementById("setting-shortcut-status");
  if (!status) return;
  status.textContent =
    message ||
    "点击输入框后按下组合键；Backspace / Delete 清空，Esc 取消。PageUp / PageDown 保留给列表翻页。";
  status.classList.toggle("is-error", isError);
}

function handleShortcutCaptureKeydown(event) {
  if (event.key === "Tab") return;
  event.preventDefault();
  event.stopPropagation();
  const input = event.currentTarget;

  if (event.key === "Escape") {
    input.value = input.dataset.previousValue || "";
    setShortcutCaptureStatus();
    input.blur();
    return;
  }
  if (
    (event.key === "Backspace" || event.key === "Delete") &&
    !event.ctrlKey &&
    !event.altKey &&
    !event.metaKey
  ) {
    input.value = "";
    setShortcutCaptureStatus();
    return;
  }
  if (["Control", "Shift", "Alt", "Meta"].includes(event.key)) return;

  const binding = shortcutBindingFromKeyboardEvent(event);
  if (!binding) return;
  if (shortcutUsesPaginationKey(binding)) {
    setShortcutCaptureStatus(
      "PageUp / PageDown 已用于列表翻页，请换一个按键组合。",
      true,
    );
    return;
  }
  input.value = binding;
  setShortcutCaptureStatus(`已捕获：${binding}`);
}

function syncPlaybackShortcutControls() {
  PLAYBACK_SHORTCUT_FIELDS.forEach(([key, elementId]) => {
    const input = document.getElementById(elementId);
    if (input) input.value = normalizeShortcutBinding(webSettings[key]);
  });
}

function bindPlaybackShortcutSettingEvents() {
  document.querySelectorAll("[data-shortcut-input]").forEach((input) => {
    if (input.dataset.bound === "1") return;
    input.dataset.bound = "1";
    input.addEventListener("focus", () => {
      input.dataset.previousValue = input.value;
      setShortcutCaptureStatus("请按下新的快捷键组合…");
    });
    input.addEventListener("keydown", handleShortcutCaptureKeydown);
    input.addEventListener("blur", () => setShortcutCaptureStatus());
  });
  document.querySelectorAll("[data-shortcut-clear]").forEach((button) => {
    if (button.dataset.bound === "1") return;
    button.dataset.bound = "1";
    button.addEventListener("click", () => {
      const input = document.getElementById(button.dataset.shortcutClear);
      if (!input) return;
      input.value = "";
      setShortcutCaptureStatus();
      input.focus();
    });
  });
}

function readPlaybackShortcutSettingsFromForm() {
  const settings = {};
  const seen = new Map();
  for (const [key, elementId, label] of PLAYBACK_SHORTCUT_FIELDS) {
    const input = document.getElementById(elementId);
    const binding = normalizeShortcutBinding(input?.value || "");
    if (input) input.value = binding;
    settings[key] = binding;
    if (!binding) continue;
    if (shortcutUsesPaginationKey(binding)) {
      throw new Error(`${label} 不能使用 PageUp / PageDown，该按键保留给列表翻页。`);
    }
    if (seen.has(binding)) {
      throw new Error(
        `快捷键冲突：${seen.get(binding)} 和 ${label} 都使用了 ${binding}。`,
      );
    }
    seen.set(binding, label);
  }
  return settings;
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

  const webdavEnabledToggle = document.getElementById(
    "setting-webdav-enabled",
  );
  if (webdavEnabledToggle) {
    webdavEnabledToggle.checked = webSettings.webdavEnabled;
  }
  const webdavUrlInput = document.getElementById("setting-webdav-url");
  if (webdavUrlInput) {
    webdavUrlInput.value = webSettings.webdavUrl;
  }
  const webdavUsernameInput = document.getElementById(
    "setting-webdav-username",
  );
  if (webdavUsernameInput) {
    webdavUsernameInput.value = webSettings.webdavUsername;
  }
  const webdavPasswordInput = document.getElementById(
    "setting-webdav-password",
  );
  if (webdavPasswordInput) {
    webdavPasswordInput.value = webSettings.webdavPassword;
  }
  const webdavDirInput = document.getElementById("setting-webdav-dir");
  if (webdavDirInput) {
    webdavDirInput.value = webSettings.webdavDir;
  }
  const sodaSignerUrlInput = document.getElementById("setting-soda-signer-url");
  if (sodaSignerUrlInput) {
    sodaSignerUrlInput.value = webSettings.sodaSignerUrl;
  }
  const sodaSignerTokenInput = document.getElementById(
    "setting-soda-signer-token",
  );
  if (sodaSignerTokenInput) {
    sodaSignerTokenInput.value = "";
  }
  const sodaClientDirInput = document.getElementById("setting-soda-client-dir");
  if (sodaClientDirInput) {
    sodaClientDirInput.value = webSettings.sodaClientDir;
  }
  const sodaLocalSignerState = document.getElementById(
    "setting-soda-local-signer-state",
  );
  if (sodaLocalSignerState) {
    sodaLocalSignerState.textContent = webSettings.sodaLocalSignerAvailable
      ? "已检测到本机签名器：留空地址即可用。"
      : "未检测到本机签名器：请安装汽水官方客户端（或 Node.js），或填写下面的远程签名服务地址。";
  }
  bindWebDAVSettingEvents();
  syncWebDAVVisibility();

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

  const downloadTipDurationInput = document.getElementById(
    "setting-download-tip-duration",
  );
  if (downloadTipDurationInput) {
    downloadTipDurationInput.value = String(webSettings.downloadTipDuration);
  }

  const floatingLyricsToggle = document.getElementById(
    "setting-floating-lyrics",
  );
  if (floatingLyricsToggle) {
    floatingLyricsToggle.checked = !webSettings.disableFloatingLyrics;
  }
  bindLyricFontSettingEvents();
  syncLyricAppearanceControls();
  bindPlaybackShortcutSettingEvents();
  syncPlaybackShortcutControls();

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

  const filterVersionToggle = document.getElementById(
    "setting-filter-version-keywords",
  );
  if (filterVersionToggle) {
    filterVersionToggle.checked = webSettings.filterVersionKeywords;
  }
  const versionFilterKeywordsInput = document.getElementById(
    "setting-version-filter-keywords",
  );
  if (versionFilterKeywordsInput) {
    versionFilterKeywordsInput.value = webSettings.versionFilterKeywords;
  }
  const filterShortTracksToggle = document.getElementById(
    "setting-filter-short-tracks",
  );
  if (filterShortTracksToggle) {
    filterShortTracksToggle.checked = webSettings.filterShortTracks;
  }
  const normalizeChineseVariantsToggle = document.getElementById(
    "setting-normalize-chinese-variants",
  );
  if (normalizeChineseVariantsToggle) {
    normalizeChineseVariantsToggle.checked =
      webSettings.normalizeChineseVariants;
  }
  bindSearchFilterSettingEvents();

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

function downloadNoticeDuration() {
  const seconds = Number(webSettings.downloadTipDuration);
  return (Number.isFinite(seconds) && seconds > 0
    ? Math.min(seconds, 60)
    : DEFAULT_DOWNLOAD_TIP_DURATION) * 1000;
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

  const close = (immediate = false) => {
    if (!toast.isConnected) return;
    if (immediate) {
      toast.remove();
      return;
    }
    toast.classList.add("app-toast-hide");
    window.setTimeout(() => toast.remove(), 220);
  };
  toast.querySelector(".app-toast-close")?.addEventListener("click", () => close());
  if (duration > 0) {
    window.setTimeout(close, duration);
  }
  return close;
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
  if (!link) return false;
  link.style.pointerEvents = "none";
  link.style.opacity = "0.6";
  try {
    const result = await enqueueDownloadURLs([link.href]);
    showToast("下载队列", result.added ? "已加入等待队列，可在右侧“下载管理”查看。" : "此歌曲已在等待或下载中。", "success", downloadNoticeDuration());
    return true;
  } catch (error) {
    showToast("加入队列失败", error.message, "error", downloadNoticeDuration());
    return false;
  } finally {
    link.style.pointerEvents = "";
    link.style.opacity = "";
  }
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

  searchForm.onsubmit = async (event) => {
    event.preventDefault();
    if (searchForm.dataset.loading === "1") return;

    const submitButton = searchForm.querySelector(".search-btn");
    const submitIcon = submitButton?.querySelector("i");
    searchForm.dataset.loading = "1";
    searchForm.setAttribute("aria-busy", "true");
    if (submitButton) {
      submitButton.disabled = true;
      submitButton.title = "搜索中";
    }
    if (submitIcon) {
      submitIcon.className = "fa-solid fa-spinner fa-spin";
    }

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

    try {
      await navigateTo(targetURL.toString(), { scrollToResults: true });
    } finally {
      if (searchForm.isConnected) {
        delete searchForm.dataset.loading;
        searchForm.removeAttribute("aria-busy");
        if (submitButton) {
          submitButton.disabled = false;
          submitButton.title = "搜索";
        }
        if (submitIcon) {
          submitIcon.className = "fa-solid fa-magnifying-glass";
        }
      }
    }
  };
}

function bindSongCardCovers(root = document) {
  const cards = root.querySelectorAll(".song-card");
  cards.forEach((card) => {
    if (songActionFoldMedia.matches) {
      ensureSongActionFold(card);
    }
    queueInspectSong(card);

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

const songActionFoldMedia = window.matchMedia("(max-width: 768px)");
const refreshSongActionFolds = () => {
  if (!songActionFoldMedia.matches) return;
  document.querySelectorAll(".song-card").forEach(ensureSongActionFold);
};
if (typeof songActionFoldMedia.addEventListener === "function") {
  songActionFoldMedia.addEventListener("change", refreshSongActionFolds);
} else {
  songActionFoldMedia.addListener(refreshSongActionFolds);
}

function ensureSongActionFold(card) {
  const actions = card?.querySelector(".actions");
  if (!actions) return;

  let moreActions = actions.querySelector(".mobile-more-actions");
  if (!moreActions) {
    moreActions = document.createElement("div");
    moreActions.className = "mobile-more-actions";
    actions.appendChild(moreActions);
  }

  Array.from(actions.children).forEach((action) => {
    if (action.classList.contains("btn-dl")) {
      moreActions.appendChild(action);
    }
  });

  let toggle = actions.querySelector(".mobile-more-toggle");
  if (!toggle) {
    toggle = document.createElement("button");
    toggle.type = "button";
    toggle.className = "btn-circle mobile-more-toggle";
    toggle.title = "更多下载操作";
    toggle.setAttribute("aria-label", "更多下载操作");
    toggle.setAttribute("aria-expanded", "false");
    toggle.innerHTML =
      '<i class="fa-solid fa-ellipsis" aria-hidden="true"></i>';
    toggle.addEventListener("click", (event) => {
      event.stopPropagation();
      const expanded = !moreActions.classList.contains("is-open");
      moreActions.classList.toggle("is-open", expanded);
      toggle.setAttribute("aria-expanded", String(expanded));
      toggle.classList.toggle("is-open", expanded);
    });
    actions.appendChild(toggle);
  }

  const hasDownloadActions = Boolean(
    moreActions.querySelector(".btn-dl"),
  );
  toggle.hidden = !hasDownloadActions;
  if (!hasDownloadActions) {
    moreActions.classList.remove("is-open");
    toggle.setAttribute("aria-expanded", "false");
    toggle.classList.remove("is-open");
  }
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
  return webSettings.autoCacheOnPlay === true;
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
  bindSongListTools(root);

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
  syncTVFocusableElements(root);
  scheduleTVInitialFocus();
}

function setSongListToolsOpen(tools, isOpen) {
  if (!tools) return;
  const trigger = tools.querySelector(".song-list-tools-trigger");
  const popover = tools.querySelector(".song-list-tools-popover");
  if (!trigger || !popover) return;
  popover.hidden = !isOpen;
  trigger.setAttribute("aria-expanded", isOpen ? "true" : "false");
}

function closeSongListTools() {
  document.querySelectorAll(".song-list-tools").forEach((tools) => {
    setSongListToolsOpen(tools, false);
  });
}

function toggleSongListTools(event) {
  event?.stopPropagation();
  const tools = event?.currentTarget?.closest(".song-list-tools");
  if (!tools) return;
  const popover = tools.querySelector(".song-list-tools-popover");
  const isOpen = popover ? !popover.hidden : false;
  closeSongListTools();
  setSongListToolsOpen(tools, !isOpen);
}

function bindSongListTools(root = document) {
  const scope = root && typeof root.querySelectorAll === "function" ? root : document;
  scope.querySelectorAll(".song-list-tools").forEach((tools) => {
    if (tools.dataset.bound === "1") return;
    tools.dataset.bound = "1";
    setSongListToolsOpen(tools, false);
  });
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

function syncRightToolbar(nextDoc, currentContainer) {
  const currentToolbar = document.querySelector(".right-toolbar");
  const nextToolbar = nextDoc.querySelector(".right-toolbar");

  if (!nextToolbar) {
    if (currentToolbar) currentToolbar.remove();
    return;
  }

  // 分页区由服务端按当前结果页生成，Ajax 导航时必须整体替换，避免保留上一页的按钮状态。
  if (currentToolbar) {
    currentToolbar.replaceWith(nextToolbar.cloneNode(true));
  } else if (currentContainer) {
    currentContainer.before(nextToolbar.cloneNode(true));
  }
  bindAuthFloat();
  refreshAuthFloat();
}

// 结果区顶部至少要有这么多像素可见，才认为用户已经能看到搜索结果。
const MIN_VISIBLE_RESULTS_PX = 120;

// 小屏（安卓 WebView）首屏几乎被搜索框与搜索源面板占满，搜索后若仍停在页面
// 顶部，结果列表会落在首屏之外，用户会误以为"搜索为空"。这里把结果区（无结果
// 时为空状态提示）滚到视口顶部。结果区已有足够高度可见时不滚动，避免桌面端
// 无谓跳动。只在搜索提交时调用，分页/播放列表等内部导航保持原行为。
function scrollToSearchResults() {
  const target =
    document.querySelector(".list-header") ||
    document.querySelector(".no-results");
  if (!target) {
    return;
  }

  const rect = target.getBoundingClientRect();
  if (rect.top >= 0 && window.innerHeight - rect.top >= MIN_VISIBLE_RESULTS_PX) {
    return;
  }

  window.scrollTo({
    top: Math.max(rect.top + window.scrollY - 12, 0),
    behavior: "auto",
  });
}

function navigateBack() {
  if (window.history.length > 1) {
    window.history.back();
    return;
  }
  void navigateTo(API_ROOT + "/");
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

    cancelPendingSongInspections();
    currentContainer.innerHTML = nextContainer.innerHTML;
    syncRightToolbar(nextDoc, currentContainer);
    defaultDocumentTitle = nextDoc.title || defaultDocumentTitle;
    document.title = defaultDocumentTitle;

    const renderedPage = nextContainer.querySelector(".page-summary");
    const renderedPageMatch = renderedPage
      ? renderedPage.textContent.match(/(\d+)\s*\/\s*(\d+)/)
      : null;
    if (renderedPageMatch && targetURL.searchParams.has("page")) {
      targetURL.searchParams.set("page", renderedPageMatch[1]);
    }

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

    if (options.scrollToResults) {
      scrollToSearchResults();
    } else if (options.scroll !== false) {
      window.scrollTo({ top: 0, behavior: "auto" });
    }

    return true;
  } catch (error) {
    if (error && error.name === "AbortError") {
      return false;
    }
    // WebView 可能不把 console 输出到 logcat，SPA 回退时把错误经桥接回传，
    // 便于在壳层日志里定位是哪一步在安卓端失败。
    try {
      if (globalThis.callback && typeof globalThis.callback.musicDlAppState === "function") {
        globalThis.callback.musicDlAppState(
          "spa_error:" + (error && error.message ? error.message : String(error))
        );
      }
    } catch (_) {}
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
  if (document.querySelector("dialog[open]")) return true;
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

  const binding = shortcutBindingFromKeyboardEvent(event);
  if (!binding) return;
  const actions = [
    [webSettings.shortcutPlayPause, togglePlayback],
    [webSettings.shortcutPrevious, () => switchTrackByOffset(-1)],
    [webSettings.shortcutNext, () => switchTrackByOffset(1)],
    [webSettings.shortcutSeekBack, () => adjustCurrentTrackPosition(-10)],
    [webSettings.shortcutSeekForward, () => adjustCurrentTrackPosition(10)],
  ];
  const action = actions.find(
    ([shortcut]) =>
      shortcut && normalizeShortcutBinding(shortcut) === binding,
  );
  if (!action) return;

  event.preventDefault();
  if (event.repeat && action[0] === webSettings.shortcutPlayPause) return;
  action[1]();
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

  document.addEventListener("click", function (event) {
    if (!event.target.closest(".song-list-tools")) {
      closeSongListTools();
    }
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
  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape") closeSongListTools();
  });

  window.addEventListener("popstate", function () {
    navigateTo(window.location.href, {
      historyMode: "none",
      scroll: false,
    });
  });
}

const TV_UI_QUERY_PARAM = "tv";
const TV_FOCUSABLE_ATTRIBUTE = "data-tv-focusable";
const TV_FOCUSABLE_SELECTOR = '[data-tv-focusable="1"]';
const TV_INTERACTIVE_SELECTOR = [
  "a[href]",
  "button",
  "input:not([type='hidden'])",
  "select",
  "textarea",
  "summary",
  "[role='button']",
  "[role='tab']",
  "[role='link']",
  "[onclick]",
  ".type-option",
  ".source-option",
  ".playlist-card",
  ".song-card",
  ".setting-toggle",
  ".setting-link-row",
  ".collection-item",
  ".rt-btn",
  ".rt-page-num",
  ".aplayer-button",
  ".aplayer-list-toggle",
  ".vg-ctrl-icon",
  ".vg-close",
].join(",");

let tvUIEnabled = false;
let tvUIObserver = null;
let tvInitialFocusTimer = 0;
let tvActiveOverlay = null;
let tvRestoreFocusElement = null;
let tvKeyboardActive = false;

function detectTVUIMode() {
  const params = new URLSearchParams(window.location.search);
  const requested = String(params.get(TV_UI_QUERY_PARAM) || "").toLowerCase();
  if (requested === "1" || requested === "true" || requested === "yes") {
    return true;
  }
  if (requested === "0" || requested === "false" || requested === "no") {
    return false;
  }

  const ua = String(navigator.userAgent || "");
  if (
    /Android[ _-]?TV|Google[ _-]?TV|Fire[ _-]?TV|Smart[ _-]?TV|SMART-TV|BRAVIA|AFT[A-Z0-9]+|MiTV|MiBOX|NVIDIA[ _-]?SHIELD|Web0S|webOS\.TV/i.test(
      ua,
    )
  ) {
    return true;
  }

  try {
    return (
      window.matchMedia("(pointer: none)").matches &&
      window.matchMedia("(min-width: 900px)").matches
    );
  } catch (_) {
    return false;
  }
}

function tvElementVisible(element) {
  if (!element || !element.isConnected) return false;
  if (
    element.hidden ||
    element.matches(":disabled, [aria-disabled='true']") ||
    element.closest(
      "[hidden], [inert], [aria-hidden='true'], [aria-disabled='true']",
    )
  ) {
    return false;
  }
  if (
    element.matches(".source-option") &&
    element.querySelector(".source-checkbox:disabled")
  ) {
    return false;
  }

  const style = window.getComputedStyle(element);
  if (
    style.display === "none" ||
    style.visibility === "hidden" ||
    style.visibility === "collapse" ||
    style.pointerEvents === "none" ||
    Number(style.opacity) === 0
  ) {
    return false;
  }

  const rect = element.getBoundingClientRect();
  return rect.width > 1 && rect.height > 1;
}

function syncTVFocusableElements(root = document) {
  if (!tvUIEnabled || !root || typeof root.querySelectorAll !== "function") {
    return;
  }

  const elements = [];
  if (root.nodeType === Node.ELEMENT_NODE && root.matches(TV_INTERACTIVE_SELECTOR)) {
    elements.push(root);
  }
  root.querySelectorAll(TV_INTERACTIVE_SELECTOR).forEach((element) => {
    elements.push(element);
  });

  elements.forEach((element) => {
    if (
      element.matches("input[type='radio'], input[type='checkbox']") &&
      element.closest("label.type-option, label.setting-toggle")
    ) {
      return;
    }

    const forcedFocusTarget = element.matches("[data-settings-tab]");
    if (
      !forcedFocusTarget &&
      element.hasAttribute("tabindex") &&
      Number(element.getAttribute("tabindex")) < 0
    ) {
      return;
    }
    if (!element.hasAttribute("tabindex")) {
      element.tabIndex = 0;
    }
    element.setAttribute(TV_FOCUSABLE_ATTRIBUTE, "1");
  });
}

function visibleTVOverlays() {
  return Array.from(
    document.querySelectorAll(".modal-overlay, .vg-overlay"),
  ).filter(tvElementVisible);
}

function getActiveTVOverlay() {
  const overlays = visibleTVOverlays();
  return overlays.length ? overlays[overlays.length - 1] : null;
}

function getTVFocusCandidates() {
  syncTVFocusableElements(document);
  const overlay = getActiveTVOverlay();
  const scope = overlay || document;
  return Array.from(scope.querySelectorAll(TV_FOCUSABLE_SELECTOR)).filter(
    tvElementVisible,
  );
}

function activeTVFocusElement() {
  const active = document.activeElement;
  if (active && active.matches && active.matches(TV_FOCUSABLE_SELECTOR)) {
    return active;
  }
  if (active && active.closest) {
    return active.closest(TV_FOCUSABLE_SELECTOR);
  }
  return null;
}

function centerOfRect(rect) {
  return {
    x: rect.left + rect.width / 2,
    y: rect.top + rect.height / 2,
  };
}

function overlapLength(aStart, aEnd, bStart, bEnd) {
  return Math.max(0, Math.min(aEnd, bEnd) - Math.max(aStart, bStart));
}

function bestTVFocusCandidate(direction, current, candidates) {
  const currentRect = current.getBoundingClientRect();
  const currentCenter = centerOfRect(currentRect);
  let best = null;
  let bestScore = Number.POSITIVE_INFINITY;

  candidates.forEach((candidate) => {
    if (candidate === current || !tvElementVisible(candidate)) return;
    const rect = candidate.getBoundingClientRect();
    const center = centerOfRect(rect);
    const dx = center.x - currentCenter.x;
    const dy = center.y - currentCenter.y;

    let primary = 0;
    let perpendicular = 0;
    let overlap = 0;
    if (direction === "left") {
      if (dx >= -4) return;
      primary = -dx;
      perpendicular = Math.abs(dy);
      overlap = overlapLength(currentRect.top, currentRect.bottom, rect.top, rect.bottom);
    } else if (direction === "right") {
      if (dx <= 4) return;
      primary = dx;
      perpendicular = Math.abs(dy);
      overlap = overlapLength(currentRect.top, currentRect.bottom, rect.top, rect.bottom);
    } else if (direction === "up") {
      if (dy >= -4) return;
      primary = -dy;
      perpendicular = Math.abs(dx);
      overlap = overlapLength(currentRect.left, currentRect.right, rect.left, rect.right);
    } else if (direction === "down") {
      if (dy <= 4) return;
      primary = dy;
      perpendicular = Math.abs(dx);
      overlap = overlapLength(currentRect.left, currentRect.right, rect.left, rect.right);
    } else {
      return;
    }

    let score = primary + perpendicular * 1.55 - overlap * 0.55;
    if (overlap <= 0) {
      score += 120;
    }
    if (primary < perpendicular * 0.22) {
      score += 240;
    }
    if (score < bestScore) {
      best = candidate;
      bestScore = score;
    }
  });

  return best;
}

function scrollTVFocusIntoView(element) {
  if (!element || !tvElementVisible(element)) return;
  const rect = element.getBoundingClientRect();
  const topSafe = 54;
  const bottomSafe = Math.max(180, window.innerHeight - 170);
  if (rect.top >= topSafe && rect.bottom <= bottomSafe) {
    return;
  }
  try {
    element.scrollIntoView({
      block: "center",
      inline: "nearest",
      behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches
        ? "auto"
        : "smooth",
    });
  } catch (_) {
    element.scrollIntoView();
  }
}

function markTVKeyboardNavigation() {
  tvKeyboardActive = true;
  document.body.classList.add("tv-keyboard-active");
}

function focusTVElement(element) {
  if (!element || !tvElementVisible(element)) return false;
  try {
    element.focus({ preventScroll: true });
  } catch (_) {
    element.focus();
  }
  markTVKeyboardNavigation();
  scrollTVFocusIntoView(element);
  return document.activeElement === element;
}

function navigateTVFocus(direction) {
  if (!tvUIEnabled) return false;
  const candidates = getTVFocusCandidates();
  if (!candidates.length) return false;

  const current = activeTVFocusElement();
  if (!current) {
    focusTVElement(defaultTVFocusTarget(candidates));
    return true;
  }
  const next = bestTVFocusCandidate(direction, current, candidates);
  if (!next) return false;
  return focusTVElement(next);
}

function firstTVFocusMatch(scope, selectors) {
  for (const selector of selectors) {
    const match = scope.querySelector(selector);
    if (match && tvElementVisible(match)) {
      return match;
    }
  }
  return null;
}

function defaultTVFocusTarget(candidates) {
  const overlay = getActiveTVOverlay();
  const preferred = overlay
    ? firstTVFocusMatch(overlay, [
        "[data-tv-autofocus]",
        ".settings-tab.is-active",
        ".modal-close",
        "button",
        "a[href]",
        "input:not([type='hidden'])",
        "select",
      ])
    : firstTVFocusMatch(document, [
        "[data-tv-default-focus]",
        ".type-option",
        ".source-option",
        "#search-keyword",
        "button",
        "a[href]",
      ]);
  if (preferred && candidates.includes(preferred)) {
    return preferred;
  }
  return candidates[0];
}

function scheduleTVInitialFocus() {
  if (!tvUIEnabled) return;
  window.clearTimeout(tvInitialFocusTimer);
  tvInitialFocusTimer = window.setTimeout(() => {
    syncTVFocusableElements(document);
    const overlay = getActiveTVOverlay();
    const active = activeTVFocusElement();
    const activeBelongsInScope =
      active && (!overlay || overlay.contains(active));
    if (!activeBelongsInScope) {
      focusTVElement(defaultTVFocusTarget(getTVFocusCandidates()));
    }
  }, 80);
}

function activateTVElement(element) {
  if (!element || !tvElementVisible(element)) return false;
  if (element.matches(".song-card")) {
    const playButton = element.querySelector(".btn-play");
    if (playButton) {
      playButton.click();
      return true;
    }
  }
  if (element.matches(".collection-item")) {
    const clickable = element.querySelector(".col-clickable-area");
    if (clickable) {
      clickable.click();
      return true;
    }
  }
  element.click();
  return true;
}

function closeTVTopLayer() {
  if (!tvUIEnabled) return false;

  const videoOverlay = document.getElementById("vg-modal");
  if (
    videoOverlay &&
    tvElementVisible(videoOverlay) &&
    window.VideoGen &&
    typeof window.VideoGen.close === "function"
  ) {
    window.VideoGen.close();
    return true;
  }

  const overlay = getActiveTVOverlay();
  if (overlay) {
    const closeButton = overlay.querySelector(
      "[data-tv-back], .modal-close, [onclick*='style.display = \"none\"'], [onclick*=\"style.display = 'none'\"]",
    );
    if (closeButton) {
      closeButton.click();
    } else {
      overlay.style.display = "none";
    }
    if (tvRestoreFocusElement && tvElementVisible(tvRestoreFocusElement)) {
      window.setTimeout(() => focusTVElement(tvRestoreFocusElement), 0);
    }
    return true;
  }

  const openTools = Array.from(
    document.querySelectorAll(".song-list-tools-popover"),
  ).find((popover) => !popover.hidden);
  if (openTools) {
    closeSongListTools();
    return true;
  }
  return false;
}

function handleTVMediaKey(key) {
  if (
    key === "MediaPlayPause" ||
    key === "PlayPause" ||
    key === "MediaPlay" ||
    key === "MediaPause"
  ) {
    togglePlayback();
    return true;
  }
  if (key === "MediaTrackNext" || key === "MediaFastForward") {
    switchTrackByOffset(1);
    return true;
  }
  if (key === "MediaTrackPrevious" || key === "MediaRewind") {
    switchTrackByOffset(-1);
    return true;
  }
  return false;
}

function handleTVNavigationAction(action) {
  if (!tvUIEnabled) return false;
  if (
    action === "up" ||
    action === "down" ||
    action === "left" ||
    action === "right"
  ) {
    return navigateTVFocus(action);
  }
  if (action === "enter") {
    return activateTVElement(document.activeElement);
  }
  if (action === "back") {
    return closeTVTopLayer();
  }
  if (handleTVMediaKey(action)) {
    return true;
  }
  return false;
}

function handleTVKeyboardEvent(event) {
  if (!tvUIEnabled || event.defaultPrevented || event.isComposing) return;
  if (event.altKey || event.ctrlKey || event.metaKey) return;

  const directionMap = {
    ArrowUp: "up",
    ArrowDown: "down",
    ArrowLeft: "left",
    ArrowRight: "right",
  };
  const direction = directionMap[event.key];
  const current = document.activeElement;
  const currentTag = current ? String(current.tagName || "").toLowerCase() : "";
  const selectingControl =
    currentTag === "select" ||
    (current &&
      current.matches &&
      current.matches("input[type='range'], input[type='file']"));
  const editingText =
    current &&
    current.matches &&
    current.matches(
      "input:not([type='button']):not([type='submit']):not([type='checkbox']):not([type='radio']):not([type='range']), textarea, [contenteditable='true']",
    );

  if (direction) {
    if (
      selectingControl ||
      (editingText &&
        currentTag === "textarea" &&
        (direction === "up" || direction === "down" || direction === "left" || direction === "right")) ||
      (editingText &&
        currentTag === "input" &&
        (direction === "left" || direction === "right"))
    ) {
      return;
    }
    if (handleTVNavigationAction(direction)) {
      event.preventDefault();
      event.stopPropagation();
    }
    return;
  }

  if (event.key === "Enter" || event.key === "NumpadEnter") {
    if (editingText || selectingControl || currentTag === "textarea") {
      return;
    }
    if (handleTVNavigationAction("enter")) {
      event.preventDefault();
      event.stopPropagation();
    }
    return;
  }

  if (event.key === "Escape" || event.key === "BrowserBack") {
    if (handleTVNavigationAction("back")) {
      event.preventDefault();
      event.stopPropagation();
    }
    return;
  }

  if (handleTVMediaKey(event.key)) {
    event.preventDefault();
    event.stopPropagation();
  }
}

function initializeTVMode() {
  tvUIEnabled = detectTVUIMode();
  if (!tvUIEnabled) return;

  document.documentElement.classList.add("tv-ui");
  document.body.classList.add("tv-ui");
  syncTVFocusableElements(document);
  document.addEventListener("keydown", handleTVKeyboardEvent, true);
  document.addEventListener(
    "pointerdown",
    () => {
      tvKeyboardActive = false;
      document.body.classList.remove("tv-keyboard-active");
    },
    { passive: true },
  );
  tvUIObserver = new MutationObserver(() => {
    syncTVFocusableElements(document);
    const nextOverlay = getActiveTVOverlay();
    if (nextOverlay !== tvActiveOverlay) {
      if (nextOverlay && !tvActiveOverlay) {
        tvRestoreFocusElement = activeTVFocusElement();
      }
      tvActiveOverlay = nextOverlay;
      scheduleTVInitialFocus();
    }
  });
  tvUIObserver.observe(document.body, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ["class", "hidden", "style", "aria-hidden"],
  });

  window.__musicDlTVNavigate = navigateTVFocus;
  window.__musicDlTVHandleKey = handleTVNavigationAction;
  window.__musicDlHandleBack = closeTVTopLayer;
  scheduleTVInitialFocus();
}

document.addEventListener("DOMContentLoaded", function () {
  initializeTVMode();
  loadWebSettingsFromCache();
  applyWebSettings(webSettings);
  bindSettingsTabs();
  bindAuthFloat();
  refreshAuthFloat();
  fetchWebSettings().finally(() => maybeAutoCheckUpdate());
  bindPageNavigationEvents();
  initializePageContent(document);
  syncTVFocusableElements(document);
  scheduleTVInitialFocus();
  updateFloatPageNav();
  // 直接打开搜索结果链接，或 SPA 失败回退到整页加载时，同样把结果滚入视口。
  // 仅在 URL 带搜索词时触发，普通页面加载不受影响。
  if (new URLSearchParams(window.location.search).get("q")) {
    scrollToSearchResults();
  }
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

function goToTopLists() {
  const selected = [];
  document.querySelectorAll(".source-checkbox:checked").forEach((cb) => {
    if (cb.dataset.toplistSupported === "true") {
      selected.push(cb.value);
    }
  });

  if (selected.length === 0) {
    navigateTo(API_ROOT + "/toplists");
  } else {
    navigateTo(
      API_ROOT +
        "/toplists?sources=" +
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
    const sortDuration = String(parsePositiveInt(card.dataset.duration, 0));
    if (card.dataset.sortDuration !== sortDuration) {
      card.dataset.sortDuration = sortDuration;
    }
    const sortSize = String(parseSizeToBytes(card.dataset.sortSize || ""));
    if (card.dataset.sortSize !== sortSize) {
      card.dataset.sortSize = sortSize;
    }
    const sortBitrate = String(
      parseBitrateToKbps(card.dataset.sortBitrate || ""),
    );
    if (card.dataset.sortBitrate !== sortBitrate) {
      card.dataset.sortBitrate = sortBitrate;
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
  const sortedCards = cards.slice().sort((a, b) => {
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
  if (cards.every((card, index) => card === sortedCards[index])) {
    return;
  }

  const fragment = document.createDocumentFragment();
  sortedCards.forEach((card) => fragment.appendChild(card));
  list.appendChild(fragment);
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
                    <span class="tag tag-loading" id="size-${escapeHTML(song.id)}">-</span>
                    <span class="tag tag-loading" id="bitrate-${escapeHTML(song.id)}">-</span>
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
    cancelPendingSongInspections();
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

const DUPLICATE_GROUP_PAGE_SIZE = 10;
let activeDuplicatePage = 1;

async function checkDuplicateSongs(page = 1) {
  // 显示 loading 弹窗
  showDuplicateModal(null, true);
  try {
    const params = new URLSearchParams({
      page: String(Math.max(1, parsePositiveInt(page, 1))),
      page_size: String(DUPLICATE_GROUP_PAGE_SIZE),
    });
    const resp = await fetch(`${API_ROOT}/local_music/duplicates?${params}`);
    if (!resp.ok) throw new Error("API error");
    const data = await resp.json();
    activeDuplicatePage = Math.max(1, parsePositiveInt(data.page, 1));
    showDuplicateModal(data.groups || [], false, {
      page: activeDuplicatePage,
      total: Math.max(0, parsePositiveInt(data.total, 0)),
      totalPages: Math.max(1, parsePositiveInt(data.total_pages, 1)),
    });
  } catch (_) {
    showDuplicateModal([], false, { page: 1, total: 0, totalPages: 1 });
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

function showDuplicateModal(groups, loading, pagination = {}) {
  const old = document.getElementById("duplicate-modal-overlay");
  if (old) old.remove();

  const overlay = document.createElement("div");
  overlay.id = "duplicate-modal-overlay";
  overlay.className = "modal-overlay utility-modal-overlay is-open";
  overlay.onclick = (e) => {
    if (e.target === overlay) overlay.remove();
  };

  const modal = document.createElement("div");
  modal.className = "modal utility-modal duplicate-modal";
  const header = document.createElement("div");
  header.className = "modal-header";
  header.innerHTML = [
    '<div><h3><i class="fa-solid fa-triangle-exclamation"></i> 重复歌曲检测</h3><p class="utility-modal-subtitle">按歌曲名和歌手聚合本地音乐</p></div>',
    '<button type="button" class="modal-close" aria-label="关闭重复检测"><i class="fa-solid fa-xmark"></i></button>',
  ].join("");
  header.querySelector(".modal-close")?.addEventListener("click", () => overlay.remove());
  modal.appendChild(header);

  const body = document.createElement("div");
  body.className = "modal-body duplicate-content";

  if (loading) {
    body.innerHTML = '<div class="duplicate-state is-loading"><div><i class="fa-solid fa-spinner fa-spin"></i><br>正在扫描本地音乐...</div></div>';
  } else if (!groups || groups.length === 0) {
    body.innerHTML = '<div class="duplicate-state is-clean"><div><i class="fa-solid fa-circle-check"></i><br>未发现重复歌曲</div></div>';
  } else {
    groups.forEach((g, gi) => {
      const gp = document.createElement("div");
      gp.className = "duplicate-group";
      const firstNumber = ((pagination.page || 1) - 1) * DUPLICATE_GROUP_PAGE_SIZE + gi + 1;
      gp.innerHTML = `<div class="duplicate-group-title">${firstNumber}. ${escapeHTML(g.name)} · ${escapeHTML(g.artist)}</div>`;

      g.songs.forEach((s) => {
        const sizeStr = s.size ? (s.size / 1048576).toFixed(1) + "MB" : "?";
        const durStr = s.duration
          ? Math.floor(s.duration / 60) +
            ":" +
            String(s.duration % 60).padStart(2, "0")
          : "?";
        const row = document.createElement("div");
        row.className = "duplicate-row";
        row.innerHTML = `<span class="duplicate-meta">${escapeHTML(s.ext || "?")} · ${sizeStr} · ${durStr}</span><span class="duplicate-actions"></span>`;
        const actions = row.querySelector(".duplicate-actions");

        const playBtn = document.createElement("button");
        playBtn.type = "button";
        playBtn.className = "btn-pill btn-pill-primary";
        playBtn.innerHTML = '<i class="fa-solid fa-play"></i> 播放';
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
        delBtn.className = "btn-pill btn-pill-danger";
        delBtn.innerHTML = '<i class="fa-solid fa-trash"></i> 删除';
        delBtn.onclick = async () => {
          if (!confirm(`确定删除 "${s.name}"?`)) return;
          const originalHTML = delBtn.innerHTML;
          delBtn.disabled = true;
          delBtn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i>';
          try {
            await deleteLocalMusic(s.id);
            stopDeletedLocalMusicPlayback(new Set([s.id]));
            await refreshLocalMusicPageAfterMutation();
            await checkDuplicateSongs(activeDuplicatePage);
          } catch (error) {
            alert(error.message || "删除失败");
          } finally {
            if (delBtn.isConnected) {
              delBtn.innerHTML = originalHTML;
              delBtn.disabled = false;
            }
          }
        };

        actions.appendChild(playBtn);
        actions.appendChild(delBtn);
        gp.appendChild(row);
      });
      body.appendChild(gp);
    });
    body.insertAdjacentHTML(
      "beforeend",
      renderUtilityModalPagination(
        pagination.page || 1,
        pagination.totalPages || 1,
        "checkDuplicateSongs",
      ),
    );
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

function setSongInspectLoading(card, loading) {
  if (!card) return;
  const sizeTag =
    document.getElementById(`size-${card.dataset.id}`) ||
    card.querySelector('[id^="size-"]');
  const bitrateTag =
    document.getElementById(`bitrate-${card.dataset.id}`) ||
    card.querySelector('[id^="bitrate-"]');

  if (loading) {
    if (sizeTag) {
      sizeTag.className = "tag tag-loading";
      sizeTag.innerHTML = '<i class="fa fa-spinner fa-spin"></i>';
    }
    if (bitrateTag) {
      bitrateTag.className = "tag tag-loading";
      bitrateTag.innerHTML = '<i class="fa fa-circle-notch fa-spin"></i>';
    }
    return;
  }

  if (sizeTag?.querySelector(".fa-spin")) {
    sizeTag.textContent = "-";
    sizeTag.className = "tag";
  }
  if (bitrateTag?.querySelector(".fa-spin")) {
    bitrateTag.textContent = "-";
    bitrateTag.className = "tag";
  }
}

async function inspectSong(card, controller = null) {
  if (!card) return;
  const signal = controller ? controller.signal : undefined;
  const id = card.dataset.id;
  const source = card.dataset.source;
  const duration = card.dataset.duration;
  const extra = card.dataset.extra || "";
  let retryDelay = 0;

  const params = new URLSearchParams({
    id: String(id || ""),
    source: String(source || ""),
    duration: String(duration || ""),
  });
  if (extra !== "" && extra !== "{}" && extra !== "null") {
    params.set("extra", extra);
  }

  try {
    const response = await fetch(`${API_ROOT}/inspect?${params.toString()}`, {
      signal,
    });
    if (signal && signal.aborted) return;
    if (!response.ok) {
      const error = new Error(`HTTP ${response.status}`);
      error.httpStatus = response.status;
      error.retryAfterMs = parseInspectRetryAfter(
        response.headers.get("Retry-After"),
      );
      throw error;
    }

    const data = await response.json();
    if (
      (signal && signal.aborted) ||
      !card.isConnected ||
      !data ||
      typeof data !== "object"
    ) {
      return;
    }
    if (typeof data.valid !== "boolean") {
      throw new Error("invalid inspect response");
    }

    if (data.song && typeof data.song === "object") {
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
      inspectRetryCounts.delete(card);
      delete card.dataset.inspectRetryable;
      card.dataset.inspectCompleted = "1";
      if (sizeTag) {
        sizeTag.textContent = data.size;
        sizeTag.className = "tag tag-success";
        sizeTag.removeAttribute("title");
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
      recordInspectSourceSuccess(source);
    } else if (data.retryable === true || data.status === "retryable") {
      const suggestedDelay = Math.max(
        parsePositiveInt(data.retry_after_ms, 0),
        parseInspectRetryAfter(response.headers.get("Retry-After")),
      );
      retryDelay = recordInspectSourcePressure(source, suggestedDelay);
      card.dataset.inspectRetryable = "1";
      if (sizeTag) {
        sizeTag.textContent = "稍后重试";
        sizeTag.className = "tag tag-loading";
        sizeTag.title = data.reason
          ? `平台暂不可用：${data.reason}`
          : "平台暂不可用";
      }
      if (bitrateTag) {
        bitrateTag.textContent = "-";
        bitrateTag.className = "tag";
      }
    } else {
      inspectRetryCounts.delete(card);
      delete card.dataset.inspectRetryable;
      card.dataset.inspectCompleted = "1";
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
  } catch (error) {
    if (
      error &&
      (error.name === "AbortError" || signal?.aborted || !card.isConnected)
    ) {
      return;
    }
    retryDelay = recordInspectSourcePressure(
      source,
      parsePositiveInt(error?.retryAfterMs, 0),
    );
    const el = card.querySelector('[id^="size-"]');
    if (el) el.textContent = "检测失败";
  } finally {
    setSongInspectLoading(card, false);
    delete card.dataset.inspectPending;
    if (
      retryDelay > 0 &&
      card.isConnected &&
      (!signal || !signal.aborted)
    ) {
      const preparedRetryDelay = prepareInspectRetry(card, retryDelay);
      if (preparedRetryDelay > 0) {
        scheduleInspectRetry(card, preparedRetryDelay);
      }
    }
    if (controller) {
      finishInspectSong(controller);
    }
    if (
      card.isConnected &&
      (!signal || !signal.aborted) &&
      document.querySelector(".tag-fail")
    ) {
      scheduleAutoSwitchInvalidSources();
    }
  }
}

const inspectReadyQueue = [];
const inspectPendingTimers = new Set();
const inspectPendingCards = new Set();
const inspectActiveRequests = new Map();
const inspectSourceStates = new Map();
let inspectRetryCounts = new WeakMap();
let inspectPumpTimer = 0;
let inspectPumpAt = 0;

function inspectNow() {
  return window.performance && typeof window.performance.now === "function"
    ? window.performance.now()
    : Date.now();
}

function parseInspectRetryAfter(value) {
  if (value === null || value === undefined || value === "") return 0;
  const numeric = Number(value);
  if (Number.isFinite(numeric) && numeric >= 0) {
    return Math.min(INSPECT_RETRY_MAX_DELAY_MS, Math.round(numeric * 1000));
  }
  const timestamp = Date.parse(String(value));
  if (!Number.isFinite(timestamp)) return 0;
  return Math.max(
    0,
    Math.min(INSPECT_RETRY_MAX_DELAY_MS, timestamp - Date.now()),
  );
}

function getInspectSourceState(source) {
  const key = String(source || "unknown");
  let state = inspectSourceStates.get(key);
  if (!state) {
    const local = isLocalMusicSourceValue(key);
    state = {
      active: 0,
      maxConcurrency: local
        ? INSPECT_MAX_CONCURRENCY
        : INSPECT_SOURCE_MAX_CONCURRENCY,
      nextAvailableAt: 0,
      cooldownUntil: 0,
      successStreak: 0,
      failureCount: 0,
    };
    inspectSourceStates.set(key, state);
  }
  return state;
}

function recordInspectSourceSuccess(source) {
  const state = getInspectSourceState(source);
  state.failureCount = 0;
  state.successStreak++;
  if (
    !isLocalMusicSourceValue(source) &&
    state.maxConcurrency < INSPECT_SOURCE_MAX_CONCURRENCY &&
    state.successStreak >= 5
  ) {
    state.maxConcurrency++;
    state.successStreak = 0;
  }
}

function recordInspectSourcePressure(source, requestedDelay = 0) {
  if (isLocalMusicSourceValue(source)) return 0;
  const state = getInspectSourceState(source);
  state.successStreak = 0;
  const now = inspectNow();
  if (now >= state.cooldownUntil) {
    state.failureCount = Math.min(5, state.failureCount + 1);
    state.maxConcurrency = Math.max(
      INSPECT_SOURCE_MIN_CONCURRENCY,
      Math.floor(state.maxConcurrency / 2),
    );
    const exponentialDelay = Math.min(
      INSPECT_RETRY_MAX_DELAY_MS,
      INSPECT_RETRY_BASE_DELAY_MS * 2 ** (state.failureCount - 1),
    );
    const delay = Math.max(
      parsePositiveInt(requestedDelay, 0),
      exponentialDelay + Math.floor(Math.random() * 350),
    );
    state.cooldownUntil = now + delay;
  } else if (requestedDelay > 0) {
    state.cooldownUntil = Math.max(
      state.cooldownUntil,
      now + parsePositiveInt(requestedDelay, 0),
    );
  }
  state.nextAvailableAt = Math.max(state.nextAvailableAt, state.cooldownUntil);
  return Math.max(0, state.cooldownUntil - now);
}

function prepareInspectRetry(card, requestedDelay) {
  if (!card || !card.isConnected) return 0;
  const retries = inspectRetryCounts.get(card) || 0;
  if (retries >= INSPECT_MAX_RETRIES) return 0;
  inspectRetryCounts.set(card, retries + 1);
  card.dataset.inspectRetryAttempt = String(retries + 1);
  return Math.min(
    INSPECT_RETRY_MAX_DELAY_MS,
    Math.max(
      parsePositiveInt(requestedDelay, 0),
      INSPECT_RETRY_BASE_DELAY_MS * 2 ** retries,
    ) + Math.floor(Math.random() * 350),
  );
}

function scheduleInspectRetry(card, delay) {
  if (!card || delay <= 0) return;
  const timer = window.setTimeout(() => {
    inspectPendingTimers.delete(timer);
    if (!card.isConnected) return;
    queueInspectSong(card, 0);
  }, delay);
  inspectPendingTimers.add(timer);
}

function pumpInspectQueue() {
  while (
    inspectActiveRequests.size < INSPECT_MAX_CONCURRENCY &&
    inspectReadyQueue.length > 0
  ) {
    const now = inspectNow();
    let selectedIndex = -1;
    let selectedSource = "";
    let earliestWait = Infinity;

    for (let index = 0; index < inspectReadyQueue.length; index++) {
      const candidate = inspectReadyQueue[index];
      if (!candidate?.isConnected) {
        if (candidate) {
          delete candidate.dataset.inspectPending;
          inspectPendingCards.delete(candidate);
        }
        inspectReadyQueue.splice(index, 1);
        index--;
        continue;
      }

      const source = candidate.dataset.source || "unknown";
      const state = getInspectSourceState(source);
      if (state.active >= state.maxConcurrency) continue;
      const availableAt = Math.max(
        state.cooldownUntil,
        state.nextAvailableAt,
      );
      if (availableAt > now) {
        earliestWait = Math.min(earliestWait, availableAt - now);
        continue;
      }
      selectedIndex = index;
      selectedSource = source;
      break;
    }

    if (selectedIndex < 0) {
      if (Number.isFinite(earliestWait)) {
        scheduleInspectPump(earliestWait);
      }
      return;
    }

    const card = inspectReadyQueue.splice(selectedIndex, 1)[0];
    const state = getInspectSourceState(selectedSource);
    const controller = new AbortController();
    inspectActiveRequests.set(controller, {
      card,
      source: selectedSource,
    });
    state.active++;
    state.nextAvailableAt = now + INSPECT_REQUEST_DELAY_MS;
    setSongInspectLoading(card, true);
    void inspectSong(card, controller);
  }
}

function scheduleInspectPump(delay = 0) {
  const now = inspectNow();
  const at = now + Math.max(0, delay);
  if (inspectPumpTimer && inspectPumpAt <= at) return;
  if (inspectPumpTimer) {
    window.clearTimeout(inspectPumpTimer);
  }
  inspectPumpAt = at;
  inspectPumpTimer = window.setTimeout(() => {
    inspectPumpTimer = 0;
    inspectPumpAt = 0;
    pumpInspectQueue();
  }, Math.max(0, delay));
}

function finishInspectSong(controller) {
  const task = inspectActiveRequests.get(controller);
  if (!task) return;
  inspectActiveRequests.delete(controller);
  inspectPendingCards.delete(task.card);
  const state = getInspectSourceState(task.source);
  state.active = Math.max(0, state.active - 1);
  pumpInspectQueue();
}

function cancelPendingSongInspections() {
  inspectPendingTimers.forEach((timer) => window.clearTimeout(timer));
  inspectPendingTimers.clear();

  inspectPendingCards.forEach((card) => {
    delete card.dataset.inspectPending;
  });
  inspectPendingCards.clear();
  inspectReadyQueue.length = 0;
  inspectRetryCounts = new WeakMap();

  if (inspectPumpTimer) {
    window.clearTimeout(inspectPumpTimer);
    inspectPumpTimer = 0;
    inspectPumpAt = 0;
  }

  inspectActiveRequests.forEach((task, controller) => {
    const card = task.card;
    delete card.dataset.inspectPending;
    setSongInspectLoading(card, false);
    controller.abort();
  });
  inspectActiveRequests.clear();
  inspectSourceStates.forEach((state) => {
    state.active = 0;
  });
}

function queueInspectSong(card, delay = 0) {
  if (
    !card ||
    !card.isConnected
  ) {
    return;
  }
  const inspectKey = [
    card.dataset.source || "",
    card.dataset.id || "",
    card.dataset.extra || "",
  ].join(":");
  if (card.dataset.inspectKey !== inspectKey) {
    card.dataset.inspectKey = inspectKey;
    delete card.dataset.inspectCompleted;
    inspectRetryCounts.delete(card);
  }
  if (card.dataset.inspectPending === "1") return;
  if (card.dataset.inspectCompleted === "1") return;

  card.dataset.inspectPending = "1";
  inspectPendingCards.add(card);
  if (delay <= 0) {
    inspectReadyQueue.push(card);
    scheduleInspectPump(0);
    return;
  }
  const timer = window.setTimeout(() => {
    inspectPendingTimers.delete(timer);
    if (!card.isConnected) {
      inspectPendingCards.delete(card);
      delete card.dataset.inspectPending;
      return;
    }
    inspectReadyQueue.push(card);
    scheduleInspectPump(0);
  }, Math.max(0, delay));
  inspectPendingTimers.add(timer);
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

let downloadQueuePage = 1;
let downloadQueueLoading = false;
let downloadQueueLastPoll = 0;
let downloadQueueRenderKey = "";
let downloadQueueFilter = "";
let downloadManagerTab = "queue";

async function enqueueDownloadURLs(urls) {
  const songs = urls.map(raw => {
    const params = new URL(raw, window.location.href).searchParams;
    return Object.fromEntries(["id", "source", "name", "artist", "album", "cover", "extra"].map(key => [key, params.get(key) || ""]).concat([["embed", params.get("embed") === "1"]]));
  });
  const response = await fetch(`${API_ROOT}/api/downloads/queue`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest" },
    body: JSON.stringify({ request_id: crypto.randomUUID(), songs }),
  });
  const data = await response.json();
  if (!response.ok || data.error) throw new Error(data.error || "加入队列失败");
  void loadDownloadQueue();
  return data;
}

async function switchDownloadManagerTab(tab) {
  downloadManagerTab = tab;
  document.getElementById("download-queue-panel").hidden = tab !== "queue";
  document.getElementById("download-history-panel").hidden = tab !== "history";
  document.querySelectorAll("[data-download-tab]").forEach(button => {
    button.setAttribute("aria-selected", String(button.dataset.downloadTab === tab));
  });
  if (tab === "queue") await loadDownloadQueue();
  else await loadDownloadRecordsPage(1);
}

async function loadDownloadQueue(page = downloadQueuePage) {
  if (downloadQueueLoading) return;
  downloadQueueLoading = true;
  downloadQueueLastPoll = Date.now();
  const list = document.getElementById("download-queue-list");
  const summary = document.getElementById("download-queue-summary");
  try {
    const params = new URLSearchParams({page: String(page), status: downloadQueueFilter});
    const response = await fetch(`${API_ROOT}/api/downloads/queue?${params}`, {cache:"no-store"});
    const data = await response.json();
    if (!response.ok || data.error) throw new Error(data.error || "读取队列失败");
    downloadQueuePage = data.page;
    const c = data.counts || {};
    const active = (c.waiting || 0) + (c.downloading || 0);
    const finished = (c.success || 0) + (c.skipped || 0) + (c.failed || 0) + (c.canceled || 0);
    const total = active + finished;
    setDownloadRecordsButtonState(active ? "downloading" : "idle");
    const badge = document.getElementById("download-queue-badge");
    if (badge) { badge.textContent = active > 999 ? "999+" : active; badge.hidden = !active; }
    if (summary) summary.innerHTML = `<div class="queue-overview"><strong>${finished}<small> / ${total} 已处理</small></strong><span>下载中 ${c.downloading || 0} · 等待 ${c.waiting || 0}</span></div><progress max="${total || 1}" value="${finished}" aria-label="队列总体进度"></progress><div class="queue-outcomes">成功 ${c.success || 0} · 跳过 ${c.skipped || 0} · 失败 ${c.failed || 0} · 已取消 ${c.canceled || 0}</div>`;
    const renderKey = JSON.stringify([data.page, downloadQueueFilter, data.tasks]);
    if (list && renderKey !== downloadQueueRenderKey) {
      downloadQueueRenderKey = renderKey;
      const labels = {waiting:"等待下载", downloading:"正在下载", success:"下载成功", skipped:"已下载，跳过", failed:"下载失败", canceled:"已取消"};
      list.innerHTML = (data.tasks || []).map(task => {
        const action = task.Status === "waiting" ? `<button class="btn-pill" onclick="downloadQueueAction(${task.ID},'cancel')">取消</button>` : ["failed","canceled"].includes(task.Status) ? `<button class="btn-pill" onclick="downloadQueueAction(${task.ID},'retry')">重试</button>` : "";
        return `<article class="queue-task"><div class="queue-task-main"><strong>${escapeHtml(task.Name || "未知歌曲")}</strong><span>${escapeHtml(task.Artist || "未知歌手")} · ${escapeHtml(task.Source)}</span><small>${escapeHtml(task.Error || task.Warning || task.Path || task.Directory || "")}</small></div><div class="queue-task-action"><span class="queue-status queue-status-${escapeHtml(task.Status)}">${labels[task.Status] || "未知状态"}</span>${action}</div></article>`;
      }).join("") || '<div class="download-records-empty">暂无任务。选择歌曲后点击“批量下载”即可加入队列。</div>';
    }
    const pagination = document.getElementById("download-queue-pagination");
    if (pagination) pagination.innerHTML = renderUtilityModalPagination(data.page, Math.max(1,data.total_pages), "loadDownloadQueue");
    // Status changes may empty the last page of a filter. Move back on the next poll.
    if (data.page > Math.max(1,data.total_pages)) downloadQueuePage = Math.max(1,data.total_pages);
    const error = document.getElementById("download-queue-error");
    if (error) error.textContent = "";
  } catch (error) {
    const el = document.getElementById("download-queue-error");
    if (el) el.textContent = `队列暂不可读取：${error.message}。将自动重试，已保存任务不会被清空。`;
  } finally {
    downloadQueueLoading = false;
  }
}

async function downloadQueueAction(id, action) {
  try {
    const response = await fetch(`${API_ROOT}/api/downloads/queue/${id}/${action}`, {method:"POST", headers:{"X-Requested-With":"XMLHttpRequest"}});
    const data = await response.json();
    if (!response.ok || data.error) throw new Error(data.error || "操作失败");
    await loadDownloadQueue();
  } catch (error) { showToast("队列操作失败", error.message, "error", downloadNoticeDuration()); }
}

window.setInterval(() => {
  if (document.hidden) return;
  const modal = document.getElementById("downloadRecordsModal");
  if ((modal && modal.style.display === "flex" && downloadManagerTab === "queue") || Date.now() - downloadQueueLastPoll >= 10000) void loadDownloadQueue();
}, 2000);
document.addEventListener("DOMContentLoaded", () => { void loadDownloadQueue(); });

const DOWNLOAD_RECORDS_PAGE_SIZE = 20;
let activeDownloadRecordsPage = 1;

function renderUtilityModalPagination(page, totalPages, onPageChange) {
  if (totalPages <= 1) return "";
  return `<button type="button" class="utility-modal-page-button" onclick="${onPageChange}(${page - 1})" ${page <= 1 ? "disabled" : ""} aria-label="上一页"><i class="fa-solid fa-chevron-left"></i></button>
    <span class="utility-modal-page-status">第 ${page} / ${totalPages} 页</span>
    <button type="button" class="utility-modal-page-button" onclick="${onPageChange}(${page + 1})" ${page >= totalPages ? "disabled" : ""} aria-label="下一页"><i class="fa-solid fa-chevron-right"></i></button>`;
}

async function openDownloadRecordsModal() {
  const modal = document.getElementById("downloadRecordsModal");
  if (!modal) return;
  setDownloadRecordsButtonState("idle");
  modal.style.display = "flex";

  await switchDownloadManagerTab("queue");
}

function setDownloadRecordsButtonState(state = "idle") {
  const button = document.getElementById("download-records-button");
  if (!button) return;

  const downloading = state === "downloading";
  const updated = state === "updated";
  button.classList.toggle("is-downloading", downloading);
  button.classList.toggle("has-download-updates", updated);

  const label = downloading
    ? "下载管理，队列执行中"
    : updated
      ? "下载管理，有新的下载结果"
      : "下载管理";
  button.title = label;
  button.setAttribute("aria-label", label);
}

async function refreshOpenDownloadRecords() {
  const modal = document.getElementById("downloadRecordsModal");
  if (!modal || modal.style.display !== "flex") return false;
  await loadDownloadRecordsPage(1);
  return true;
}

async function loadDownloadRecordsPage(page = 1) {
  const countEl = document.getElementById("download-records-count");
  const listEl = document.getElementById("download-records-list");
  const paginationEl = document.getElementById("download-records-pagination");
  if (countEl) countEl.textContent = "加载中...";
  if (listEl) listEl.innerHTML = "";
  if (paginationEl) paginationEl.innerHTML = "";

  const params = new URLSearchParams({
    page: String(Math.max(1, parsePositiveInt(page, 1))),
    page_size: String(DOWNLOAD_RECORDS_PAGE_SIZE),
  });

  try {
    const resp = await fetch(`${API_ROOT}/api/downloads/records?${params}`);
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const data = await resp.json();
    const records = data.records || [];
    activeDownloadRecordsPage = Math.max(1, parsePositiveInt(data.page, 1));
    const total = Math.max(0, parsePositiveInt(data.total, 0));
    const totalPages = Math.max(1, parsePositiveInt(data.total_pages, 1));

    if (countEl) {
      const success = records.filter(r => r.Status === "success").length;
      const skipped = records.filter(r => r.Status === "skipped").length;
      const failed = records.filter(r => r.Status === "failed").length;
      countEl.textContent = `共 ${total} 条  ·  本页成功 ${success}  ·  跳过 ${skipped}  ·  失败 ${failed}`;
    }

    if (records.length === 0) {
      if (listEl) listEl.innerHTML = '<div class="download-records-empty"><div><i class="fa-regular fa-clock"></i><br>暂无下载记录</div></div>';
      return;
    }

    let html = `<table class="download-records-table">
      <thead><tr>
        <th>歌曲</th>
        <th>歌手</th>
        <th>来源</th>
        <th>状态</th>
        <th>时间</th>
      </tr></thead><tbody>`;

    for (const r of records) {
      const status = r.Status === "success"
        ? { className: "is-success", icon: "fa-check", label: "成功" }
        : r.Status === "skipped"
          ? { className: "is-skipped", icon: "fa-forward", label: "跳过" }
          : { className: "is-failed", icon: "fa-xmark", label: "失败" };
      const errHint = r.Error ? ` title="${escapeHtml(r.Error)}"` : "";
      const time = r.CreatedAt ? new Date(r.CreatedAt).toLocaleString() : "";
      html += `<tr${errHint}>
        <td><div class="download-record-name">${escapeHtml(r.Name || "")}</div></td>
        <td><div class="download-record-artist">${escapeHtml(r.Artist || "")}</div></td>
        <td><span class="download-record-source">${escapeHtml(r.Source || "")}</span></td>
        <td><span class="download-record-status ${status.className}"><i class="fa-solid ${status.icon}"></i>${status.label}</span></td>
        <td class="download-record-time">${escapeHtml(time)}</td>
      </tr>`;
    }

    html += "</tbody></table>";
    if (listEl) listEl.innerHTML = html;
    if (paginationEl) {
      paginationEl.innerHTML = renderUtilityModalPagination(
        activeDownloadRecordsPage,
        totalPages,
        "loadDownloadRecordsPage",
      );
    }
  } catch (err) {
    if (countEl) countEl.textContent = "加载失败";
    if (listEl) listEl.innerHTML = `<div class="download-records-error">加载失败: ${escapeHtml(err.message)}</div>`;
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
    const paginationEl = document.getElementById("download-records-pagination");
    if (countEl) countEl.textContent = "已清空";
    if (listEl) listEl.innerHTML = '<div class="download-records-empty"><div><i class="fa-regular fa-clock"></i><br>暂无下载记录</div></div>';
    if (paginationEl) paginationEl.innerHTML = "";
    activeDownloadRecordsPage = 1;
  } catch (err) {
    alert("清空失败: " + err.message);
  }
}

const PLAYBACK_HISTORY_STORAGE_KEY = "musicdl:playback-history";
const PLAYBACK_HISTORY_LIMIT = 100;
const PLAYBACK_HISTORY_PAGE_SIZE = 10;
let activePlaybackHistoryEntries = [];
let activePlaybackHistoryPage = 1;
let lastPlaybackHistoryKey = "";
let lastPlaybackHistoryAt = 0;

function readPlaybackHistory() {
  try {
    const parsed = JSON.parse(localStorage.getItem(PLAYBACK_HISTORY_STORAGE_KEY) || "[]");
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter((entry) => entry && entry.name && entry.source && entry.id)
      .slice(0, PLAYBACK_HISTORY_LIMIT);
  } catch (_) {
    return [];
  }
}

function writePlaybackHistory(entries) {
  try {
    localStorage.setItem(
      PLAYBACK_HISTORY_STORAGE_KEY,
      JSON.stringify(entries.slice(0, PLAYBACK_HISTORY_LIMIT)),
    );
  } catch (_) {
    // 本地存储不可用时，播放本身不受影响。
  }
}

function playbackHistoryKey(entry) {
  return `${entry.source}\u0000${entry.id}`;
}

function rememberPlaybackHistory(audio) {
  const id = String(getPlaybackCardID(audio) || "").trim();
  const source = String(audio?.source || "").trim();
  const name = String(audio?.name || "").trim();
  if (!id || !source || !name) return;

  const now = Date.now();
  const entry = {
    id,
    source,
    name,
    artist: String(audio?.artist || "").trim(),
    album: String(audio?.album || "").trim(),
    cover: String(audio?.cover || "").trim(),
    duration: Number(audio?.duration || 0) || 0,
    extra: typeof audio?.extra === "string" ? audio.extra : serializeSongExtra(audio?.extra),
    playedAt: now,
  };
  const key = playbackHistoryKey(entry);
  if (key === lastPlaybackHistoryKey && now - lastPlaybackHistoryAt < 10_000) {
    return;
  }
  lastPlaybackHistoryKey = key;
  lastPlaybackHistoryAt = now;

  const records = readPlaybackHistory().filter(
    (item) => playbackHistoryKey(item) !== key,
  );
  records.unshift(entry);
  writePlaybackHistory(records);
}

function formatPlaybackHistoryTime(value) {
  const date = new Date(Number(value || 0));
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleString();
}

function renderPlaybackHistory(page = activePlaybackHistoryPage) {
  const countEl = document.getElementById("playback-history-count");
  const listEl = document.getElementById("playback-history-list");
  const paginationEl = document.getElementById("playback-history-pagination");
  if (!listEl) return;

  activePlaybackHistoryEntries = readPlaybackHistory();
  const total = activePlaybackHistoryEntries.length;
  const totalPages = Math.max(1, Math.ceil(total / PLAYBACK_HISTORY_PAGE_SIZE));
  activePlaybackHistoryPage = Math.min(
    Math.max(1, parsePositiveInt(page, 1)),
    totalPages,
  );
  if (countEl) {
    countEl.textContent = total
      ? `最近播放 ${total} 首`
      : "暂无播放记录";
  }

  if (total === 0) {
    listEl.innerHTML = '<div class="utility-empty-state"><div><i class="fa-regular fa-clock"></i><br>开始播放歌曲后，记录会显示在这里</div></div>';
    if (paginationEl) paginationEl.innerHTML = "";
    return;
  }

  const offset = (activePlaybackHistoryPage - 1) * PLAYBACK_HISTORY_PAGE_SIZE;
  const currentEntries = activePlaybackHistoryEntries.slice(
    offset,
    offset + PLAYBACK_HISTORY_PAGE_SIZE,
  );
  listEl.innerHTML = currentEntries
    .map((entry, index) => {
      const cover = entry.cover
        ? `<img src="${escapeHTML(entry.cover)}" alt="">`
        : '<i class="fa-solid fa-music"></i>';
      const artist = escapeHTML(entry.artist || "未知歌手");
      return `<div class="playback-history-item">
        <div class="playback-history-cover">${cover}</div>
        <div class="playback-history-main">
          <div class="playback-history-name">${escapeHTML(entry.name)}</div>
          <div class="playback-history-meta">${artist}</div>
          <div class="playback-history-time">${escapeHTML(formatPlaybackHistoryTime(entry.playedAt))}</div>
        </div>
        <button type="button" class="playback-history-play" onclick="playPlaybackHistoryItem(${offset + index})"><i class="fa-solid fa-play"></i> 播放</button>
      </div>`;
    })
    .join("");
  if (paginationEl) {
    paginationEl.innerHTML = renderUtilityModalPagination(
      activePlaybackHistoryPage,
      totalPages,
      "renderPlaybackHistory",
    );
  }
}

function openPlaybackHistoryModal() {
  const modal = document.getElementById("playbackHistoryModal");
  if (!modal) return;
  renderPlaybackHistory(1);
  modal.style.display = "flex";
}

function closePlaybackHistoryModal() {
  const modal = document.getElementById("playbackHistoryModal");
  if (modal) modal.style.display = "none";
}

function clearPlaybackHistory() {
  if (!confirm("确定清空播放历史？")) return;
  try {
    localStorage.removeItem(PLAYBACK_HISTORY_STORAGE_KEY);
  } catch (_) {}
  activePlaybackHistoryEntries = [];
  activePlaybackHistoryPage = 1;
  renderPlaybackHistory(1);
}

function playPlaybackHistoryItem(index) {
  const entry = activePlaybackHistoryEntries[index];
  if (!entry || !ap?.list) return;

  const song = {
    id: entry.id,
    source: entry.source,
    name: entry.name,
    artist: entry.artist || "",
    album: entry.album || "",
    cover: entry.cover || "",
    duration: Number(entry.duration || 0) || 0,
    extra: entry.extra || "",
  };
  const lyricURLs = lyricURLsForPlayback(song);
  ap.list.clear();
  ap.list.add([{
    name: song.name,
    artist: song.artist,
    album: song.album,
    url: buildStreamURL(song.id, song.source, song.name, song.artist, song.album, song.cover, song.extra),
    cover: song.cover,
    lrc: lyricURLs.line,
    raw_lrc: lyricURLs.auto,
    theme: "#10b981",
    custom_id: song.id,
    source: song.source,
    duration: song.duration,
    extra: song.extra,
  }]);
  ap.play();
  closePlaybackHistoryModal();
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
    bindSettingsTabs();
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
  const previousSearchFilters = {
    filterVersionKeywords: webSettings.filterVersionKeywords,
    versionFilterKeywords: webSettings.versionFilterKeywords,
    filterShortTracks: webSettings.filterShortTracks,
  };

  let nextSettings;
  try {
    nextSettings = normalizeWebSettings({
      embedDownload: !!document.getElementById("setting-embed-download")
        ?.checked,
      downloadDir: document.getElementById("setting-download-dir")?.value || "",
      downloadFilenameTemplate:
        document.getElementById("setting-download-filename-template")?.value ||
        "",
      downloadTipDuration: parsePositiveInt(
        document.getElementById("setting-download-tip-duration")?.value,
        DEFAULT_DOWNLOAD_TIP_DURATION,
      ),
      webdavEnabled: !!document.getElementById("setting-webdav-enabled")
        ?.checked,
      webdavUrl: document.getElementById("setting-webdav-url")?.value || "",
      webdavUsername:
        document.getElementById("setting-webdav-username")?.value || "",
      webdavPassword:
        document.getElementById("setting-webdav-password")?.value || "",
      webdavDir: document.getElementById("setting-webdav-dir")?.value || "",
      disableFloatingLyrics: !document.getElementById(
        "setting-floating-lyrics",
      )?.checked,
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
      filterVersionKeywords: !!document.getElementById(
        "setting-filter-version-keywords",
      )?.checked,
      versionFilterKeywords:
        document.getElementById("setting-version-filter-keywords")?.value ||
        "Live, Remix, Demo",
      filterShortTracks: !!document.getElementById(
        "setting-filter-short-tracks",
      )?.checked,
      normalizeChineseVariants: !!document.getElementById(
        "setting-normalize-chinese-variants",
      )?.checked,
      updateRepoUrl: webSettings.updateRepoUrl || DEFAULT_UPDATE_REPO_URL,
      githubProxyEnabled: !!webSettings.githubProxyEnabled,
      githubProxyUrl: webSettings.githubProxyUrl || DEFAULT_GITHUB_PROXY_URL,
      sodaSignerUrl:
        document.getElementById("setting-soda-signer-url")?.value || "",
      sodaSignerToken:
        document.getElementById("setting-soda-signer-token")?.value || "",
      sodaClientDir:
        document.getElementById("setting-soda-client-dir")?.value || "",
      vgChangeCover: !!document.getElementById("setting-vg-change-cover")
        ?.checked,
      vgChangeAudio: !!document.getElementById("setting-vg-change-audio")
        ?.checked,
      vgChangeLyric: !!document.getElementById("setting-vg-change-lyric")
        ?.checked,
      vgExportVideo: !!document.getElementById("setting-vg-export-video")
        ?.checked,
      ...readPlaybackShortcutSettingsFromForm(),
      lyricFontFamily: normalizeLyricFontFamily(
        document.getElementById("setting-lyric-font-family")?.value,
      ),
      lyricFontSize: normalizeLyricFontSize(
        document.getElementById("setting-lyric-font-size")?.value,
      ),
    });
  } catch (error) {
    setShortcutCaptureStatus(error.message, true);
    alert(error.message || "快捷键设置无效");
    return;
  }

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
      throw new Error(
        savedSettings?.error || cookiesPayload?.error || "保存失败，请稍后重试",
      );
    }
    applyWebSettings(savedSettings || nextSettings);
    const filtersChanged =
      previousSearchFilters.filterVersionKeywords !==
        webSettings.filterVersionKeywords ||
      previousSearchFilters.versionFilterKeywords !==
        webSettings.versionFilterKeywords ||
      previousSearchFilters.filterShortTracks !==
        webSettings.filterShortTracks;
    if (filtersChanged && document.querySelector(".container")) {
      await refreshCurrentPageContent({ scroll: false });
    }
    setAuthFloatLoggedIn(true);
    alert("保存成功");
    document.getElementById("cookieModal").style.display = "none";
  } catch (error) {
    alert(error.message || "保存失败，请稍后重试");
  }
}

window.addEventListener("scroll", () => {
  const topBtn = document.getElementById("back-to-top");
  if (topBtn) {
    if (window.scrollY > 300) {
      topBtn.classList.add("show");
    } else {
      topBtn.classList.remove("show");
    }
  }
  syncCurrentTrackLocator();
});

function scrollToTop() {
  window.scrollTo({ top: 0, behavior: "smooth" });
}

function findSongCardById(songId) {
  if (!songId) return null;
  const targetId = String(songId);
  return (
    Array.from(document.querySelectorAll(".song-card")).find(
      (card) => card.dataset.id === targetId,
    ) || null
  );
}

function findCurrentPlayingCard() {
  return findSongCardById(currentPlayingId);
}

function syncCurrentTrackLocator() {
  const button = document.getElementById("locate-current-track");
  if (!button) return;
  button.hidden = !findCurrentPlayingCard();
}

function scrollToCurrentPlaying() {
  const card = findCurrentPlayingCard();
  if (!card) {
    syncCurrentTrackLocator();
    showToast("当前播放歌曲不在本页", "可返回原列表或翻到歌曲所在页。", "info");
    return;
  }
  card.scrollIntoView({ behavior: "smooth", block: "center" });
  card.classList.add("locate-flash");
  window.setTimeout(() => card.classList.remove("locate-flash"), 900);
}

// ==========================================
// 浮动翻页键
// ==========================================

function updateFloatPageNav() {
  const numEl = document.getElementById("float-page-num");
  if (!numEl) return;

  // 本地音乐页
  if (isLocalMusicPageActive()) {
    const bar = document.getElementById("localMusicPagePagination");
    if (!bar) {
      setFloatPageNumber(numEl, 1);
      return;
    }
    const cur = parseInt(bar.dataset.currentPage, 10);
    if (Number.isFinite(cur)) setFloatPageNumber(numEl, cur);
    return;
  }

  const { current } = getRenderedSearchPageInfo();
  setFloatPageNumber(numEl, current);
}

function setFloatPageNumber(element, page) {
  if (element.tagName === "INPUT") {
    element.value = String(page);
  } else {
    element.textContent = String(page);
  }
}

function getRenderedSearchPageInfo() {
  const fallback = parseInt(new URLSearchParams(window.location.search).get("page"), 10) || 1;
  const summary = document.querySelector(".page-summary");
  const match = summary ? summary.textContent.match(/(\d+)\s*\/\s*(\d+)/) : null;
  if (!match) return { current: fallback, total: null };
  return {
    current: parseInt(match[1], 10) || fallback,
    total: parseInt(match[2], 10) || null,
  };
}

function floatPageUp() {
  if (isLocalMusicPageActive()) {
    const bar = document.getElementById("localMusicPagePagination");
    if (!bar) return;
    const cur = parseInt(bar.dataset.currentPage, 10);
    if (cur > 1) goToPage(cur - 1);
  } else {
    const { current } = getRenderedSearchPageInfo();
    if (current > 1) goToPage(current - 1);
  }
}

function floatPageDown() {
  if (isLocalMusicPageActive()) {
    const bar = document.getElementById("localMusicPagePagination");
    if (!bar) return;
    const cur = parseInt(bar.dataset.currentPage, 10);
    const total = parseInt(bar.dataset.totalPages, 10);
    if (cur < total) goToPage(cur + 1);
  } else {
    const { current, total } = getRenderedSearchPageInfo();
    if (total && current < total) goToPage(current + 1);
  }
}

// 点击页数 → 输入框 → Enter/blur 跳转
function floatPageNumClick() {
  const numEl = document.getElementById("float-page-num");
  if (!numEl) return;
  if (isLocalMusicPageActive()) {
    if (!document.getElementById("localMusicPagePagination")) return;
  } else if (!getRenderedSearchPageInfo().total) {
    return;
  }
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

  let submitted = false;
  const restore = () => {
    if (input.parentNode) input.parentNode.replaceChild(numEl, input);
  };
  const jump = () => {
    if (submitted) return;
    submitted = true;
    const val = parseInt(input.value, 10);
    restore();
    if (Number.isFinite(val) && val > 0) goToPage(val);
  };

  input.onkeydown = function(e) {
    if (e.key === "Enter") {
      e.preventDefault();
      jump();
    }
    if (e.key === "Escape") {
      submitted = true;
      restore();
    }
  };
  input.onblur = jump;

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
    resumeVideoGenAudioRouting();
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
      if (!floatingLyricsEnabled()) {
        currentKey = "";
        hide();
        return;
      }
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

// APlayer Config. APlayer is bundled with the app. Guard initialization so
// a player failure cannot interrupt the rest of the page.
let ap = null;
try {
  if (typeof APlayer === "undefined") {
    throw new Error("APlayer 资源未加载");
  }
  ap = new APlayer({
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
} catch (error) {
  console.warn("[music-dl] APlayer 初始化失败，播放器功能不可用:", error);
}

window.ap = ap;

function resumeVideoGenAudioRouting() {
  const videoGen = window.VideoGen;
  if (videoGen && typeof videoGen.resumeAudioRouting === "function") {
    videoGen.resumeAudioRouting();
  }
}

// Resume the prepared Web Audio graph during the user's gesture, before
// APlayer starts playback. This avoids rerouting active audio when the user
// later opens the player-cover detail view.
document.addEventListener("pointerdown", resumeVideoGenAudioRouting, {
  capture: true,
  passive: true,
});
document.addEventListener("keydown", resumeVideoGenAudioRouting, {
  capture: true,
});

let lastTaskbarPlaybackState = "";

function syncPlayerTrackControls() {
  const player = document.querySelector(".aplayer.aplayer-fixed");
  const hasTracks = Boolean(ap?.list?.audios?.length);
  player?.classList.toggle("is-empty", !hasTracks);
  if (!player) return;

  const controls = [
    [".aplayer-icon-back", "上一首"],
    [".aplayer-icon-forward", "下一首"],
    [".aplayer-icon-order", "切换播放模式"],
    [".aplayer-icon-loop", "切换播放模式"],
    [".aplayer-icon-menu", "播放列表"],
    [".aplayer-icon-volume-down", "减小音量"],
    [".aplayer-icon-volume-up", "增大音量"],
    [".aplayer-icon-volume-off", "静音或恢复音量"],
    [".aplayer-icon-lrc", "显示或隐藏歌词"],
  ];
  controls.forEach(([selector, label]) => {
    const button = player.querySelector(selector);
    if (!button) return;
    button.title = label;
    button.setAttribute("aria-label", label);
  });

  const playButton = player.querySelector(".aplayer-icon-play");
  if (playButton) {
    const label = ap?.audio && !ap.audio.paused ? "暂停" : "播放";
    playButton.title = label;
    playButton.setAttribute("aria-label", label);
  }
}

function syncTaskbarPlaybackState() {
  const hasTracks = Boolean(ap?.list?.audios?.length);
  const isPlaying = Boolean(
    hasTracks && ap?.audio && !ap.audio.paused && !ap.audio.ended,
  );
  const nextState = `${hasTracks}:${isPlaying}`;
  if (nextState === lastTaskbarPlaybackState) return;
  if (typeof window.musicDlTaskbarState !== "function") return;

  lastTaskbarPlaybackState = nextState;
  Promise.resolve(window.musicDlTaskbarState(hasTracks, isPlaying)).catch(
    () => {
      lastTaskbarPlaybackState = "";
    },
  );
}

window.musicDlTaskbarControl = function (action, value) {
  switch (action) {
    case "previous":
      switchTrackByOffset(-1);
      return true;
    case "toggle":
      togglePlayback();
      return true;
    case "next":
      switchTrackByOffset(1);
      return true;
    case "play":
      if (!ap?.list?.audios?.length) return false;
      resumeVideoGenAudioRouting();
      ap.play();
      return true;
    case "pause":
      ap?.pause();
      return true;
    case "stop":
      ap?.pause();
      seekCurrentTrack(0);
      currentPlayingId = null;
      window.currentPlayingId = null;
      highlightCard(null);
      syncAllPlayButtons();
      return true;
    case "seek": {
      const positionSeconds = (Number(value) || 0) / 1000;
      seekCurrentTrack(positionSeconds);
      return true;
    }
    default:
      return false;
  }
};

setInterval(syncPlayerTrackControls, 500);
setInterval(syncTaskbarPlaybackState, 500);
syncPlayerTrackControls();
syncTaskbarPlaybackState();

const PLAYER_SPEEDS = Object.freeze([
  0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.25, 2.5, 2.75, 3,
]);
let playerSpeed = (() => {
  const saved = Number(localStorage.getItem("musicdl:playbackRate"));
  return PLAYER_SPEEDS.includes(saved) ? saved : 1;
})();

window.PLAYER_SPEEDS = PLAYER_SPEEDS;
window.playerSpeed = playerSpeed;
window.getPlayerPlaybackRate = () => playerSpeed;

function applyPlayerPlaybackRate(rate) {
  if (!ap?.audio) return;
  const normalized = Number(rate) || 1;
  ap.audio.playbackRate = normalized;
  if (window.VideoGen?.isLocalAudio && window.VideoGen?.localAudio) {
    window.VideoGen.localAudio.playbackRate = normalized;
  }
  const btn = document.querySelector(".player-speed-btn");
  if (btn) btn.textContent = `${normalized}x`;
  document.querySelectorAll(".player-speed-option").forEach((option) => {
    option.classList.toggle(
      "active",
      Number(option.dataset.rate) === normalized,
    );
  });
  const vgBtn = document.getElementById("vg-speed-button");
  if (vgBtn) vgBtn.textContent = `${normalized}x`;
  document.querySelectorAll("#vg-speed-menu .vg-speed-option").forEach(
    (option) => {
      option.classList.toggle(
        "active",
        Number(option.dataset.rate) === normalized,
      );
    },
  );
  syncMediaSession();
}

function setPlayerPlaybackRate(rate) {
  const normalized = Number(rate);
  if (!PLAYER_SPEEDS.includes(normalized)) return;
  playerSpeed = normalized;
  window.playerSpeed = normalized;
  localStorage.setItem("musicdl:playbackRate", String(normalized));
  if (window.VideoGen) {
    window.VideoGen.playbackRate = normalized;
  }
  applyPlayerPlaybackRate(normalized);
}

window.setPlayerPlaybackRate = setPlayerPlaybackRate;
window.applyPlayerPlaybackRate = applyPlayerPlaybackRate;

(function initPlayerSpeedControl() {
  let attempts = 0;

  function tryInject() {
    attempts++;
    if (attempts > 30) return;

    const apFixed = document.querySelector(".aplayer.aplayer-fixed");
    if (!apFixed) {
      setTimeout(tryInject, 100);
      return;
    }
    if (document.querySelector(".player-speed-wrap")) return;

    const wrap = document.createElement("div");
    wrap.className = "player-speed-wrap";

    const button = document.createElement("button");
    button.type = "button";
    button.className = "player-speed-btn";
    button.textContent = `${playerSpeed}x`;
    button.title = "播放速度";
    button.setAttribute("aria-label", "播放速度");
    button.setAttribute("aria-haspopup", "true");
    button.setAttribute("aria-expanded", "false");

    const menu = document.createElement("div");
    menu.className = "player-speed-menu";
    menu.hidden = true;
    menu.innerHTML = PLAYER_SPEEDS.map(
      (rate) =>
        `<button type="button" class="player-speed-option${
          rate === playerSpeed ? " active" : ""
        }" data-rate="${rate}">${rate}x</button>`,
    ).join("");

    button.addEventListener("click", (event) => {
      event.stopPropagation();
      const nextHidden = !menu.hidden;
      menu.hidden = nextHidden;
      button.setAttribute("aria-expanded", String(!nextHidden));
    });

    menu.addEventListener("click", (event) => {
      event.stopPropagation();
      const option = event.target.closest(".player-speed-option");
      if (!option) return;
      setPlayerPlaybackRate(Number(option.dataset.rate));
      menu.hidden = true;
      button.setAttribute("aria-expanded", "false");
    });

    document.addEventListener("click", (event) => {
      if (!wrap.contains(event.target)) {
        menu.hidden = true;
        button.setAttribute("aria-expanded", "false");
      }
    });

    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape" && !menu.hidden) {
        menu.hidden = true;
        button.setAttribute("aria-expanded", "false");
      }
    });

    wrap.append(button, menu);
    apFixed.appendChild(wrap);
    applyPlayerPlaybackRate(playerSpeed);
    syncPlayerTrackControls();
  }

  setTimeout(tryInject, 100);
})();

applyPlayerPlaybackRate(playerSpeed);

setupMediaSession();
if (ap?.audio) {
  ap.audio.addEventListener("timeupdate", () => KaraokeLyrics.update());
  ap.audio.addEventListener("seeked", () => KaraokeLyrics.update());
  ap.audio.addEventListener("loadedmetadata", () => {
    KaraokeLyrics.load(getCurrentAPlayerAudio());
    applyPlayerPlaybackRate(playerSpeed);
  });
  ap.audio.addEventListener("emptied", () => {
    applyPlayerPlaybackRate(playerSpeed);
  });
  ap.audio.addEventListener("play", () =>
    KaraokeLyrics.handlePlayStateChange(true),
  );
  ap.audio.addEventListener("pause", () =>
    KaraokeLyrics.handlePlayStateChange(false),
  );
  ap.audio.addEventListener("ended", () =>
    KaraokeLyrics.handlePlayStateChange(false),
  );
}

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
    syncPlayerTrackControls();
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

if (ap && typeof ap.on === "function") {
  ap.on("listswitch", (e) => {
    const index = e.index;
    applyPlayerPlaybackRate(playerSpeed);
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
    resumeVideoGenAudioRouting();
    applyPlayerPlaybackRate(playerSpeed);
    syncTaskbarPlaybackState();
    const idx = ap?.list?.index;
    const audio = typeof idx === "number" ? ap.list.audios[idx] : null;
    rememberPlaybackHistory(audio);
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
    syncTaskbarPlaybackState();
    syncAllPlayButtons();
    syncMediaSession();
    if (window.VideoGen && window.VideoGen.updatePlayBtnState) {
      window.VideoGen.updatePlayBtnState(false);
    }
  });

  ap.on("ended", () => {
    syncTaskbarPlaybackState();
    currentPlayingId = null;
    window.currentPlayingId = null;
    highlightCard(null);
    syncAllPlayButtons();
    scheduleMediaSessionSync(getCurrentAPlayerAudio(), 180);
    KaraokeLyrics.hide();
  });
}

function highlightCard(targetId) {
  document
    .querySelectorAll(".song-card")
    .forEach((c) => c.classList.remove("playing-active"));
  const target = findSongCardById(targetId);
  target?.classList.add("playing-active");
  syncCurrentTrackLocator();
}

function setPlayButtonState(card, isPlaying) {
  if (!card) return;
  const btn = card.querySelector(".btn-play");
  if (!btn) return;
  const icon = btn.querySelector("i");
  if (!icon) return;
  if (
    btn.classList.contains("is-stop") === isPlaying &&
    icon.classList.contains(isPlaying ? "fa-stop" : "fa-play")
  ) {
    return;
  }

  icon.classList.remove("fa-play", "fa-stop");
  icon.classList.add(isPlaying ? "fa-stop" : "fa-play");
  btn.title = isPlaying ? "停止" : "播放";
  btn.setAttribute("aria-label", isPlaying ? "停止" : "播放");
  btn.classList.toggle("is-stop", isPlaying);
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
    if (coverBtn?.parentNode) {
      coverBtn.parentNode.insertBefore(lyric, coverBtn);
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
  ensureSongActionFold(card);
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
  if (autoSwitchInvalidTimer) return;
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
    btn.classList.add("is-active");
    btn.setAttribute("aria-pressed", "true");
    toolbar.classList.add("active");
  } else {
    btn.innerHTML = '<i class="fa-solid fa-list-check"></i> 批量操作';
    btn.classList.remove("is-active");
    btn.setAttribute("aria-pressed", "false");
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
  const batchCopy = document.getElementById("btn-batch-copy-urls");

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
    if (batchCopy) batchCopy.disabled = nonLocalCount === 0;
    if (batchDeleteLocal) batchDeleteLocal.disabled = localCount === 0;
    if (batchFavLocal) batchFavLocal.disabled = localCount === 0;
    if (batchFav) batchFav.disabled = false;
    if (batchRemoveCollection) batchRemoveCollection.disabled = false;
  } else {
    if (batchSwitch) batchSwitch.disabled = true;
    if (batchDl) batchDl.disabled = true;
    if (batchCopy) batchCopy.disabled = !pageHasNonLocalSongs();
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

function pageHasNonLocalSongs() {
  const cards = document.querySelectorAll(".song-card");
  for (const card of cards) {
    const song = songFromCard(card);
    if (song && !isLocalMusicSourceValue(song.source)) return true;
  }
  return false;
}

function toAbsoluteDownloadURL(url) {
  const value = String(url || "");
  if (!value) return value;
  if (/^https?:\/\//i.test(value) || value.startsWith("//")) return value;
  try {
    return new URL(value, window.location.href).href;
  } catch (_) {
    return value;
  }
}

async function copyTextToClipboard(text) {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch (_) {
      // Fall through to the legacy path for webviews without clipboard permission.
    }
  }

  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.top = "0";
  textarea.style.left = "0";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.focus();
  textarea.select();
  textarea.setSelectionRange(0, text.length);
  const copied = document.execCommand && document.execCommand("copy");
  document.body.removeChild(textarea);
  if (!copied) {
    throw new Error("当前环境不支持剪贴板");
  }
}

async function batchCopyDownloadUrls() {
  const selectedSongs = getSelectedSongs().filter(
    (song) => !isLocalMusicSourceValue(song.source),
  );

  let songs = selectedSongs;
  if (songs.length === 0) {
    songs = [];
    document.querySelectorAll(".song-card").forEach((card) => {
      const song = songFromCard(card);
      if (song && !isLocalMusicSourceValue(song.source)) {
        songs.push(song);
      }
    });
  }

  if (songs.length === 0) {
    showToast("没有可复制的下载地址", "当前列表没有可下载的歌曲。", "info");
    return;
  }

  const urls = songs.map((song) =>
    toAbsoluteDownloadURL(
      buildBrowserDownloadURL(
        song.id,
        song.source,
        song.name,
        song.artist,
        song.album || "",
        song.cover || "",
        song.extra || "",
      ),
    ),
  );
  const text = urls.join("\n");

  try {
    await copyTextToClipboard(text);
    const scope =
      selectedSongs.length > 0
        ? `已选 ${selectedSongs.length} 首`
        : `当前页 ${songs.length} 首`;
    showToast(
      "下载地址已复制",
      `${scope}歌曲的下载链接已写入剪贴板，可直接粘贴到 aria2。`,
      "success",
      downloadNoticeDuration(),
    );
  } catch (err) {
    showToast(
      "复制失败",
      err && err.message ? err.message : "请手动复制。",
      "warning",
    );
  }
}

// Use an in-page dialog: macOS WebView does not provide window.confirm panels.
function confirmBatchDownload(count) {
  return confirmAppAction({
    title: "确认批量下载",
    message: `将当前页选中的 ${count} 首歌曲加入下载队列？已有任务会继续执行，新歌曲将排队等待。`,
    confirmLabel: "开始下载",
  });
}

function confirmAppAction({title, message, confirmLabel}) {
  return new Promise((resolve) => {
    const previousFocus = document.activeElement;
    const dialog = document.createElement("dialog");
    dialog.className = "modal utility-modal batch-download-confirm";
    dialog.setAttribute("aria-labelledby", "app-confirm-title");
    dialog.setAttribute("aria-describedby", "app-confirm-message");
    dialog.innerHTML = `
      <div class="modal-header"><h3 id="app-confirm-title"></h3></div>
      <form method="dialog" class="modal-body">
        <p id="app-confirm-message"></p>
        <div class="utility-modal-footer utility-modal-actions">
          <button type="submit" class="btn-pill" value="cancel" autofocus>取消</button>
          <button type="submit" class="btn-pill" value="confirm"></button>
        </div>
      </form>`;
    dialog.querySelector("h3").textContent = title;
    dialog.querySelector("p").textContent = message;
    dialog.querySelector('button[value="confirm"]').textContent = confirmLabel;
    dialog.addEventListener("close", () => {
      const confirmed = dialog.returnValue === "confirm";
      dialog.remove();
      if (previousFocus && previousFocus.isConnected) previousFocus.focus();
      resolve(confirmed);
    }, { once: true });
    document.body.appendChild(dialog);
    dialog.showModal();
  });
}

let batchDownloadPending = false;

async function batchDownload() {
  if (batchDownloadPending) return;
  batchDownloadPending = true;
  try {
    await runBatchDownload();
  } finally {
    batchDownloadPending = false;
  }
}

async function runBatchDownload() {
  const songs = getSelectedSongs().filter(song => !isLocalMusicSourceValue(song.source));
  if (!songs.length) return;
  if (!(await confirmBatchDownload(songs.length))) return;
  try {
    const result = await enqueueDownloadURLs(songs.map(song => song.url));
    showToast("已加入下载队列", `新增 ${result.added} 首，队列中已存在 ${result.existing} 首。可继续翻页追加，进度请查看右侧“下载管理”。`, "success", downloadNoticeDuration());
    await openDownloadRecordsModal();
  } catch (error) {
    showToast("加入队列失败", error.message, "error", downloadNoticeDuration());
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

  const concurrency = Math.min(3, cards.length);
  let nextIndex = 0;
  const runWorker = async () => {
    while (nextIndex < cards.length) {
      const card = cards[nextIndex++];
      const switchBtn = card.querySelector(".btn-switch");
      if (switchBtn) {
        await switchSource(switchBtn, {
          silent: !!options.silent,
          deferToolbar: true,
        });
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
  const { modal, list, dir } = localMusicElements();
  if (!modal) return;

  closeDownloadRecordsModal();
  if (list) list.innerHTML = "";
  if (dir) dir.textContent = `下载目录：${webSettings.downloadDir || "-"}`;
  setLocalMusicHint("正在加载本地音乐...");
  modal.style.display = "flex";
  refreshLocalMusicList();
}

function closeLocalMusicModal() {
  const { modal } = localMusicElements();
  if (modal) modal.style.display = "none";
}

async function refreshLocalMusicList(options = {}) {
  const { list, dir } = localMusicElements();
  if (!list) return;
  if (localMusicModalState.loading) return;

  const append = !!options.append;
  if (!append) {
    localMusicModalState.offset = 0;
    localMusicModalState.total = 0;
    localMusicModalState.hasMore = false;
  }

  const params = new URLSearchParams({
    offset: String(localMusicModalState.offset),
    limit: String(localMusicModalState.limit),
  });
  if (activeLocalMusicCollectionId) {
    params.set("collection_id", activeLocalMusicCollectionId);
  }
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

    const canAddToCollection = !!activeLocalMusicCollectionId;
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
            ${canAddToCollection ? `<button type="button" class="btn-pill btn-pill-primary local-music-add ${track.already_added ? "is-added" : ""}" ${track.already_added ? "disabled" : ""}>
                <i class="fa-solid ${track.already_added ? "fa-check" : "fa-plus"}"></i> ${track.already_added ? "已添加" : "添加"}
            </button>` : ""}
        `;

    const btn = item.querySelector(".local-music-add");
    if (canAddToCollection && btn && !track.already_added) {
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

function setLocalMusicUploadHint(message, isError = false) {
  if (isLocalMusicPageActive()) {
    setLocalMusicPageHint(message, isError);
    return;
  }
  setLocalMusicHint(message, isError);
}

async function uploadLocalMusicFile(input) {
  if (!input || !input.files || input.files.length === 0) return;

  const acceptedExtensions = new Set(["mp3", "flac", "m4a", "ogg", "wav", "wma", "aac"]);
  const files = Array.from(input.files).filter((file) => {
    const extension = file.name.split(".").pop().toLowerCase();
    return acceptedExtensions.has(extension);
  });
  const ignoredCount = input.files.length - files.length;
  if (files.length === 0) {
    input.value = "";
    setLocalMusicUploadHint("所选内容中没有可导入的音频文件。", true);
    return;
  }

  const failures = [];
  let imported = 0;
  try {
    for (const [index, file] of files.entries()) {
      setLocalMusicUploadHint(`正在导入 ${index + 1} / ${files.length}：${file.webkitRelativePath || file.name}`);
      const formData = new FormData();
      formData.append("file", file);

      try {
        const response = await fetch(`${API_ROOT}/local_music/upload`, {
          method: "POST",
          body: formData,
        });
        const payload = await response.json().catch(() => null);
        if (!response.ok || !payload || payload.error) {
          throw new Error((payload && payload.error) || "导入失败");
        }
        imported++;
      } catch (error) {
        failures.push(`${file.name}：${error.message || "导入失败"}`);
      }
    }

    if (isLocalMusicPageActive()) {
      await refreshLocalMusicPageAfterMutation();
    }
    const ignoredText = ignoredCount > 0 ? `，忽略 ${ignoredCount} 个非音频文件` : "";
    if (failures.length > 0) {
      setLocalMusicUploadHint(`已导入 ${imported} 首${ignoredText}，${failures.length} 首失败：${failures[0]}`, true);
    } else {
      setLocalMusicUploadHint(`已导入 ${imported} 首${ignoredText}。`);
    }
  } finally {
    input.value = "";
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
  closeDownloadRecordsModal();
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

async function downloadWholeCollection(button) {
  if (!button || button.disabled) return;
  const originalHTML = button.innerHTML;
  button.disabled = true;
  button.textContent = "正在获取完整曲目…";
  try {
    const response = await fetch(`${API_ROOT}/api/downloads/queue/container`, {
      method: "POST",
      headers: {"Content-Type":"application/json", "X-Requested-With":"XMLHttpRequest"},
      body: JSON.stringify({request_id:crypto.randomUUID(),kind:button.dataset.containerKind,id:button.dataset.containerId,source:button.dataset.containerSource || "",link:button.dataset.containerLink || "",embed:!!webSettings.embedDownload}),
    });
    const data = await response.json();
    if (!response.ok || data.error) throw new Error(data.error || "加入队列失败");
    showToast("完整曲目已加入队列", `共获取 ${data.total} 首，新增 ${data.added} 首，队列中已存在 ${data.existing} 首，跳过本地音乐 ${data.skipped_local} 首。`, "success", downloadNoticeDuration());
    await openDownloadRecordsModal();
  } catch (error) {
    showToast("整单下载失败", error.message || "请稍后重试", "error", downloadNoticeDuration());
  } finally {
    button.disabled = false;
    button.innerHTML = originalHTML;
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
