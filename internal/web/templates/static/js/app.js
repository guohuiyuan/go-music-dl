// templates/app.js

const API_ROOT = window.API_ROOT;
const WEB_SETTINGS_KEY = 'musicdl:web_settings';
const INSPECT_REQUEST_DELAY_MS = 100;

let webSettings = {
    embedDownload: false,
    downloadToLocal: false,
    downloadDir: 'data/downloads'
};

function normalizeWebSettings(raw) {
    const next = {
        embedDownload: false,
        downloadToLocal: false,
        downloadDir: 'data/downloads'
    };

    if (!raw || typeof raw !== 'object') {
        return next;
    }

    if (typeof raw.embedDownload === 'boolean') {
        next.embedDownload = raw.embedDownload;
    }
    if (typeof raw.downloadToLocal === 'boolean') {
        next.downloadToLocal = raw.downloadToLocal;
    }
    if (typeof raw.downloadDir === 'string' && raw.downloadDir.trim() !== '') {
        next.downloadDir = raw.downloadDir.trim();
    }

    return next;
}

function loadWebSettingsFromCache() {
    try {
        const raw = localStorage.getItem(WEB_SETTINGS_KEY);
        if (!raw) return webSettings;
        webSettings = normalizeWebSettings(JSON.parse(raw));
    } catch (_) {
    }
    return webSettings;
}

function persistWebSettingsCache() {
    try {
        localStorage.setItem(WEB_SETTINGS_KEY, JSON.stringify(webSettings));
    } catch (_) {
    }
}

function applyWebSettings(settings) {
    webSettings = normalizeWebSettings(settings);
    persistWebSettingsCache();

    const embedToggle = document.getElementById('setting-embed-download');
    if (embedToggle) {
        embedToggle.checked = webSettings.embedDownload;
    }

    const localToggle = document.getElementById('setting-download-to-local');
    if (localToggle) {
        localToggle.checked = webSettings.downloadToLocal;
    }

    const dirInput = document.getElementById('setting-download-dir');
    if (dirInput) {
        dirInput.value = webSettings.downloadDir;
    }

    refreshDownloadLinks();
}

async function fetchWebSettings() {
    try {
        const response = await fetch(API_ROOT + '/settings');
        if (!response.ok) return;
        const data = await response.json();
        applyWebSettings(data);
    } catch (_) {
    }
}

function buildDownloadRequestURL(id, source, name, artist, cover, extra, options = {}) {
    const params = new URLSearchParams({
        id: String(id || ''),
        source: String(source || ''),
        name: String(name || ''),
        artist: String(artist || '')
    });

    const coverValue = String(cover || '');
    if (coverValue !== '') {
        params.set('cover', coverValue);
    }
    const extraValue = String(extra || '');
    if (extraValue !== '' && extraValue !== '{}' && extraValue !== 'null') {
        params.set('extra', extraValue);
    }
    if (options.embed) {
        params.set('embed', '1');
    }
    if (options.saveLocal) {
        params.set('save_local', '1');
    }

    return `${API_ROOT}/download?${params.toString()}`;
}

function buildStreamURL(id, source, name, artist, cover, extra) {
    return buildDownloadRequestURL(id, source, name, artist, cover, extra, {
        embed: webSettings.embedDownload
    });
}

function buildDownloadURL(id, source, name, artist, cover, extra) {
    return buildDownloadRequestURL(id, source, name, artist, cover, extra, {
        embed: webSettings.embedDownload,
        saveLocal: webSettings.downloadToLocal
    });
}

function updateDownloadButton(link) {
    if (!link) return;

    const card = link.closest('.song-card');
    if (!card) return;

    const ds = card.dataset;
    link.href = buildDownloadURL(ds.id, ds.source, ds.name, ds.artist, ds.cover || '', ds.extra || '');
    link.title = webSettings.downloadToLocal ? '保存到本地目录' : '下载歌曲';
}

function refreshDownloadLinks() {
    document.querySelectorAll('.song-card').forEach(card => {
        updateDownloadButton(card.querySelector('.btn-download'));
    });
}

async function requestLocalDownload(url) {
    const response = await fetch(url, {
        headers: {
            'Accept': 'application/json'
        }
    });
    const data = await response.json().catch(() => null);
    if (!response.ok || !data || data.error) {
        throw new Error((data && data.error) || '保存失败');
    }
    return data;
}

function formatBatchSongLabel(song) {
    const name = (song && song.name) ? song.name : 'Unknown';
    const artist = (song && song.artist) ? song.artist : 'Unknown';
    return `${name} - ${artist}`;
}

function buildBatchFailureMessage(failures, title) {
    if (!failures || failures.length === 0) {
        return '';
    }

    let message = `\n\n${title} ${failures.length} 首：`;
    failures.forEach((item, index) => {
        const reason = item.reason ? `：${item.reason}` : '';
        message += `\n${index + 1}. ${formatBatchSongLabel(item.song)}${reason}`;
    });
    return message;
}

function inferExtFromContentType(contentType) {
    const raw = String(contentType || '').toLowerCase().split(';')[0].trim();
    switch (raw) {
    case 'audio/flac':
    case 'audio/x-flac':
        return 'flac';
    case 'audio/ogg':
    case 'application/ogg':
        return 'ogg';
    case 'audio/mp4':
    case 'audio/x-m4a':
    case 'audio/aac':
    case 'audio/aacp':
        return 'm4a';
    case 'audio/x-ms-wma':
    case 'audio/wma':
        return 'wma';
    default:
        return 'mp3';
    }
}

function getDownloadFilenameFromResponse(response, song) {
    const disposition = response.headers.get('Content-Disposition') || '';
    const encodedMatch = disposition.match(/filename\*\s*=\s*utf-8''([^;]+)/i);
    if (encodedMatch && encodedMatch[1]) {
        try {
            return decodeURIComponent(encodedMatch[1].trim().replace(/^"|"$/g, ''));
        } catch (_) {
        }
    }

    const plainMatch = disposition.match(/filename\s*=\s*"([^"]+)"/i) || disposition.match(/filename\s*=\s*([^;]+)/i);
    if (plainMatch && plainMatch[1]) {
        return plainMatch[1].trim().replace(/^"|"$/g, '');
    }

    return `${formatBatchSongLabel(song)}.${inferExtFromContentType(response.headers.get('Content-Type'))}`;
}

async function requestBrowserDownload(song) {
    const response = await fetch(song.url);
    if (!response.ok) {
        let reason = '';
        try {
            reason = (await response.text()).trim();
        } catch (_) {
        }
        throw new Error(reason || `HTTP ${response.status}`);
    }

    const blob = await response.blob();
    const filename = getDownloadFilenameFromResponse(response, song);
    const objectUrl = URL.createObjectURL(blob);
    const link = document.createElement('a');

    link.href = objectUrl;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);

    setTimeout(() => URL.revokeObjectURL(objectUrl), 30000);

    return {
        warning: response.headers.get('X-MusicDL-Warning') || ''
    };
}

async function handleDownloadClick(link) {
    if (!webSettings.downloadToLocal || !link) {
        return false;
    }

    link.style.pointerEvents = 'none';
    link.style.opacity = '0.6';
    try {
        const data = await requestLocalDownload(link.href);
        let message = `已保存到:\n${data.path || webSettings.downloadDir}`;
        if (data.warning) {
            message += `\n\n提示: ${data.warning}`;
        }
        alert(message);
    } catch (error) {
        alert(error.message || '保存失败');
    } finally {
        link.style.pointerEvents = '';
        link.style.opacity = '';
    }

    return true;
}

document.addEventListener('DOMContentLoaded', function() {
    loadWebSettingsFromCache();
    applyWebSettings(webSettings);
    fetchWebSettings();

    const checkboxes = document.querySelectorAll('.source-checkbox');
    
    const btnAll = document.getElementById('btn-all');
    if(btnAll) {
        btnAll.onclick = () => { checkboxes.forEach(cb => { if (!cb.disabled) cb.checked = true; }); };
    }
    const btnNone = document.getElementById('btn-none');
    if(btnNone) {
        btnNone.onclick = () => { checkboxes.forEach(cb => { if (!cb.disabled) cb.checked = false; }); };
    }

    const initialTypeEl = document.querySelector('input[name="type"]:checked');
    if (initialTypeEl) {
        toggleSearchType(initialTypeEl.value);
    }

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

    syncAllPlayButtons();
});

function toggleSearchType(type) {
    const checkboxes = document.querySelectorAll('.source-checkbox');
    checkboxes.forEach(cb => {
        const isSupported = cb.dataset.supported === "true"; 
        if (type === 'playlist') {
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
    const supported = ['netease', 'qq', 'kugou', 'kuwo'];
    const selected = [];
    document.querySelectorAll('.source-checkbox:checked').forEach(cb => {
        if (supported.includes(cb.value)) {
            selected.push(cb.value);
        }
    });
    
    if (selected.length === 0) {
        window.location.href = API_ROOT + '/recommend?sources=' + supported.join('&sources=');
    } else {
        window.location.href = API_ROOT + '/recommend?sources=' + selected.join('&sources=');
    }
}

function inspectSong(card) {
    const id = card.dataset.id;
    const source = card.dataset.source;
    const duration = card.dataset.duration;
    const extra = card.dataset.extra || '';

    const params = new URLSearchParams({
        id: String(id || ''),
        source: String(source || ''),
        duration: String(duration || '')
    });
    if (extra !== '' && extra !== '{}' && extra !== 'null') {
        params.set('extra', extra);
    }

    fetch(`${API_ROOT}/inspect?${params.toString()}`)
        .then(r => r.json())
        .then(data => {
            const sizeTag = document.getElementById(`size-${id}`);
            const bitrateTag = document.getElementById(`bitrate-${id}`);

            if (data.valid) {
                if (sizeTag) {
                    sizeTag.textContent = data.size;
                    sizeTag.className = "tag tag-success"; 
                }
                if (bitrateTag) {
                    bitrateTag.textContent = data.bitrate;
                    bitrateTag.className = "tag";
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
            }
        })
        .catch(() => {
            const el = document.getElementById(`size-${id}`);
            if(el) el.textContent = "检测失败";
        });
}

function queueInspectSong(card, delay = INSPECT_REQUEST_DELAY_MS) {
    window.setTimeout(() => inspectSong(card), delay);
}

function openCookieModal() {
    document.getElementById('cookieModal').style.display = 'flex';
    Promise.all([
        fetch(API_ROOT + '/cookies').then(r => r.json()),
        fetch(API_ROOT + '/settings').then(r => r.json())
    ]).then(([cookies, settings]) => {
        applyWebSettings(settings);
        for (const [k, v] of Object.entries(cookies || {})) {
            const el = document.getElementById(`cookie-${k}`);
            if (el) el.value = v;
        }
    }).catch(() => {
        applyWebSettings(webSettings);
    });
}

function saveCookies() {
    const nextSettings = normalizeWebSettings({
        embedDownload: !!document.getElementById('setting-embed-download')?.checked,
        downloadToLocal: !!document.getElementById('setting-download-to-local')?.checked,
        downloadDir: document.getElementById('setting-download-dir')?.value || ''
    });

    const data = {};
    document.querySelectorAll('input[id^="cookie-"]').forEach(input => {
        data[input.id.replace('cookie-', '')] = input.value;
    });

    Promise.all([
        fetch(API_ROOT + '/cookies', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        }),
        fetch(API_ROOT + '/settings', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(nextSettings)
        }).then(r => r.ok ? r.json() : Promise.reject())
    ]).then(([, savedSettings]) => {
        applyWebSettings(savedSettings || nextSettings);
        alert('保存成功');
        document.getElementById('cookieModal').style.display = 'none';
    }).catch(() => {
        alert('保存失败，请稍后重试');
    });
}

window.addEventListener('scroll', () => {
    const btn = document.getElementById('back-to-top');
    if(!btn) return;
    if (window.scrollY > 300) {
        btn.classList.add('show');
    } else {
        btn.classList.remove('show');
    }
});

function scrollToTop() {
    window.scrollTo({ top: 0, behavior: 'smooth' });
}

// APlayer Config
const ap = new APlayer({
    container: document.getElementById('aplayer'),
    fixed: true, 
    autoplay: false, 
    theme: '#10b981',
    loop: 'all', 
    order: 'list', 
    preload: 'auto', 
    volume: 0.7, 
    listFolded: false, 
    lrcType: 3, 
    audio: []
});

window.ap = ap; 
let currentPlayingId = null;
window.currentPlayingId = null; 

setTimeout(() => {
    const apPic = document.querySelector('.aplayer-pic');
    if (apPic) {
        apPic.style.cursor = 'pointer';
        apPic.title = '点击打开详情/生成视频';
        
        apPic.addEventListener('click', (e) => {
            if (e.target.closest('.aplayer-button') || e.target.closest('.aplayer-play')) {
                return;
            }
            e.stopPropagation();
            e.preventDefault();
            
            const idx = ap.list.index;
            const audio = ap.list.audios[idx];
            
            if (audio && audio.custom_id && window.VideoGen) {
                window.VideoGen.open({
                    id: audio.custom_id,
                    source: audio.source || 'netease',
                    name: audio.name,
                    artist: audio.artist,
                    cover: audio.cover,
                    duration: 0 
                });
            }
        }, true);
    }
}, 800); 

ap.on('listswitch', (e) => {
    const index = e.index;
    const newAudio = ap.list.audios[index];
    if (newAudio && newAudio.custom_id) {
        currentPlayingId = newAudio.custom_id;
        window.currentPlayingId = currentPlayingId; 
        highlightCard(currentPlayingId);
        syncAllPlayButtons();

        const vgModal = document.getElementById("vg-modal");
        if (vgModal && vgModal.classList.contains("active") && window.VideoGen) {
            if (!window.VideoGen.data || window.VideoGen.data.id !== currentPlayingId) {
                window.VideoGen.open({
                    id: newAudio.custom_id,
                    source: newAudio.source || 'netease',
                    name: newAudio.name,
                    artist: newAudio.artist,
                    cover: newAudio.cover,
                    duration: 0
                });
            }
        }
    }
});

ap.on('play', () => {
    const idx = ap?.list?.index;
    const audio = (typeof idx === 'number') ? ap.list.audios[idx] : null;
    if (audio && audio.custom_id) {
        currentPlayingId = audio.custom_id;
        window.currentPlayingId = currentPlayingId; 
        highlightCard(currentPlayingId);
    }
    syncAllPlayButtons();
    
    if (window.VideoGen && window.VideoGen.updatePlayBtnState) {
        window.VideoGen.updatePlayBtnState(true);
    }
});

ap.on('pause', () => {
    syncAllPlayButtons();
    if (window.VideoGen && window.VideoGen.updatePlayBtnState) {
        window.VideoGen.updatePlayBtnState(false);
    }
});

ap.on('ended', () => {
    currentPlayingId = null;
    window.currentPlayingId = null; 
    highlightCard(null);
    syncAllPlayButtons();
});

function highlightCard(targetId) {
    document.querySelectorAll('.song-card').forEach(c => c.classList.remove('playing-active'));
    if(!targetId) return;
    const target = document.querySelector(`.song-card[data-id="${targetId}"]`);
    if (target) {
        target.classList.add('playing-active');
    }
}

function setPlayButtonState(card, isPlaying) {
    if (!card) return;
    const btn = card.querySelector('.btn-play');
    if(!btn) return;
    const icon = btn.querySelector('i');
    if (!icon) return;

    icon.classList.remove('fa-play', 'fa-stop');
    icon.classList.add(isPlaying ? 'fa-stop' : 'fa-play');
    btn.title = isPlaying ? '停止' : '播放';
}

function syncAllPlayButtons() {
    const isActuallyPlaying = ap?.audio && !ap.audio.paused;
    document.querySelectorAll('.song-card').forEach(card => {
        const id = card.dataset.id;
        const active = isActuallyPlaying && currentPlayingId && id === currentPlayingId;
        setPlayButtonState(card, active);
    });
}

function formatDuration(seconds) {
    const s = Number(seconds || 0);
    if (!s || s <= 0) return '-';
    const min = Math.floor(s / 60);
    const sec = Math.floor(s % 60);
    return `${String(min).padStart(2, '0')}:${String(sec).padStart(2, '0')}`;
}

function updateCardWithSong(card, song) {
    const oldId = card.dataset.id; 

    card.dataset.id = song.id;
    card.dataset.source = song.source;
    card.dataset.duration = song.duration || 0;
    card.dataset.name = song.name || card.dataset.name;
    card.dataset.artist = song.artist || card.dataset.artist;
    card.dataset.cover = song.cover || '';
    card.dataset.extra = song.extra ? JSON.stringify(song.extra) : '';

    const titleEl = card.querySelector('.song-info h3');
    if (titleEl) {
        if (song.link) {
            titleEl.innerHTML = `<a href="${song.link}" target="_blank" class="song-title-link" title="打开原始链接">${song.name || ''}</a>`;
        } else {
            titleEl.textContent = song.name || '';
        }
    }

    const artistLine = card.querySelector('.artist-line');
    if (artistLine) {
        const albumText = song.album ? ` &nbsp;•&nbsp; ${song.album}` : '';
        artistLine.innerHTML = `<i class="fa-regular fa-user" style="font-size:11px;"></i> ${song.artist || ''}${albumText}`;
    }

    const sourceTag = card.querySelector('.tag-src');
    if (sourceTag) sourceTag.textContent = song.source;

    const durationTag = card.querySelector('.tag-duration');
    if (durationTag) {
        durationTag.textContent = formatDuration(song.duration);
    }

    const coverWrap = card.querySelector('.cover-wrapper');
    if (coverWrap) {
        let imgEl = coverWrap.querySelector('img');
        if (!imgEl) {
            imgEl = document.createElement('img');
            coverWrap.innerHTML = '';
            coverWrap.appendChild(imgEl);
        }
        imgEl.src = song.cover || 'https://via.placeholder.com/150?text=Music';
        imgEl.alt = song.name || '';
        
        coverWrap.onclick = (e) => {
            e.stopPropagation();
            if (window.VideoGen) {
                window.VideoGen.open({
                    id: card.dataset.id,
                    source: card.dataset.source,
                    name: card.dataset.name,
                    artist: card.dataset.artist,
                    cover: imgEl.src,
                    duration: parseInt(card.dataset.duration) || 0
                });
            }
        };
    }

    const dl = card.querySelector('.btn-download');
    if (dl) {
        dl.href = buildDownloadURL(song.id, song.source, song.name, song.artist, song.cover || '', card.dataset.extra || '');
        dl.id = `dl-${song.id}`;
        dl.title = webSettings.downloadToLocal ? '保存到本地目录' : '下载歌曲';
    }

    const lrc = card.querySelector('.btn-lyric');
    if (lrc) {
        lrc.href = `${API_ROOT}/download_lrc?id=${encodeURIComponent(song.id)}&source=${song.source}&name=${encodeURIComponent(song.name)}&artist=${encodeURIComponent(song.artist)}`;
        lrc.id = `lrc-${song.id}`;
    }

    const coverBtn = card.querySelector('.btn-cover');
    if (coverBtn) {
        // 让新卡片的封面按钮始终能够使用或使用占位图响应
        let targetCoverUrl = song.cover || 'https://via.placeholder.com/600?text=No+Cover';
        coverBtn.href = `${API_ROOT}/download_cover?url=${encodeURIComponent(targetCoverUrl)}&name=${encodeURIComponent(song.name)}&artist=${encodeURIComponent(song.artist)}`;
    }

    const sizeTag = card.querySelector('[id^="size-"]');
    if (sizeTag) {
        sizeTag.id = `size-${song.id}`;
        sizeTag.className = 'tag tag-loading';
        sizeTag.innerHTML = '<i class="fa fa-spinner fa-spin"></i>';
    }
    const bitrateTag = card.querySelector('[id^="bitrate-"]');
    if (bitrateTag) {
        bitrateTag.id = `bitrate-${song.id}`;
        bitrateTag.className = 'tag tag-loading';
        bitrateTag.innerHTML = '<i class="fa fa-circle-notch fa-spin"></i>';
    }

    if (currentPlayingId === oldId) {
        currentPlayingId = song.id;
    }
    syncAllPlayButtons();
    queueInspectSong(card);
    syncSongToAPlayer(oldId, song);
}

function syncSongToAPlayer(oldId, newSong) {
    if (!ap || !ap.list || !ap.list.audios) return;
    const index = ap.list.audios.findIndex(a => a.custom_id === oldId);
    if (index !== -1) {
        const audio = ap.list.audios[index];
        audio.name = newSong.name;
        audio.artist = newSong.artist;
        audio.cover = newSong.cover;
        audio.url = buildStreamURL(newSong.id, newSong.source, newSong.name, newSong.artist, newSong.cover || '', newSong.extra ? JSON.stringify(newSong.extra) : '');
        audio.lrc = `${API_ROOT}/lyric?id=${encodeURIComponent(newSong.id)}&source=${newSong.source}`;
        audio.custom_id = newSong.id; 
        audio.source = newSong.source; 
        
        if (ap.list.index === index) {
            ap.list.switch(index); 
            if (ap.audio.paused) {
                ap.play();
            }
        }
    }
}

function switchSource(btn) {
    const card = btn.closest('.song-card');
    if (!card) return;

    const ds = card.dataset;
    const name = ds.name || '';
    const artist = ds.artist || '';
    const source = ds.source || '';
    if (!name || !source) return;

    btn.disabled = true;
    btn.style.opacity = '0.6';

    const duration = ds.duration || '';
    const url = `${API_ROOT}/switch_source?name=${encodeURIComponent(name)}&artist=${encodeURIComponent(artist)}&source=${encodeURIComponent(source)}&duration=${encodeURIComponent(duration)}`;
    fetch(url)
        .then(r => r.ok ? r.json() : Promise.reject())
        .then(song => {
            updateCardWithSong(card, song);
        })
        .catch(() => {
            alert('换源失败，请稍后重试');
        })
        .finally(() => {
            btn.disabled = false;
            btn.style.opacity = '1';
        });
}

function playAllAndJumpTo(btn) {
    const currentCard = btn.closest('.song-card');
    const allCards = Array.from(document.querySelectorAll('.song-card'));
    const clickedIndex = allCards.indexOf(currentCard);

    if (clickedIndex === -1) return;

    const clickedId = currentCard.dataset.id;
    const isActuallyPlaying = ap?.audio && !ap.audio.paused;

    if (currentPlayingId && currentPlayingId === clickedId && isActuallyPlaying) {
        ap.pause();
        try { ap.seek(0); } catch (e) {}
        currentPlayingId = null;
        highlightCard(null);
        syncAllPlayButtons();
        return;
    }

    ap.list.clear();
    const playlist = [];

    allCards.forEach(card => {
        const ds = card.dataset;
        let coverUrl = '';
        const imgEl = card.querySelector('.cover-wrapper img');
        if (imgEl) coverUrl = imgEl.src;

        playlist.push({
            name: ds.name,
            artist: ds.artist,
            url: buildStreamURL(ds.id, ds.source, ds.name, ds.artist, ds.cover || '', ds.extra || ''),
            cover: coverUrl,
            lrc: `${API_ROOT}/lyric?id=${encodeURIComponent(ds.id)}&source=${ds.source}`,
            theme: '#10b981',
            custom_id: ds.id,
            source: ds.source
        });
    });

    ap.list.add(playlist);
    ap.list.switch(clickedIndex);
    ap.play();

    currentPlayingId = clickedId;
    highlightCard(currentPlayingId);
    syncAllPlayButtons();
}

window.playAllAndJumpToId = function(songId) {
    const targetCard = document.querySelector(`.song-card[data-id="${songId}"]`);
    if (targetCard) {
        const btn = targetCard.querySelector('.btn-play');
        if (btn) {
            playAllAndJumpTo(btn);
        }
    }
};

let isBatchMode = false;

function toggleBatchMode() {
    isBatchMode = !isBatchMode;
    document.body.classList.toggle('batch-mode', isBatchMode);
    const btn = document.getElementById('btn-batch-toggle');
    const toolbar = document.getElementById('batch-toolbar');
    
    if(!btn || !toolbar) return;

    if (isBatchMode) {
        btn.innerHTML = '<i class="fa-solid fa-xmark"></i> 退出批量';
        btn.style.color = 'var(--error-color)';
        toolbar.classList.add('active'); 
    } else {
        btn.innerHTML = '<i class="fa-solid fa-list-check"></i> 批量操作';
        btn.style.color = 'var(--text-sub)';
        toolbar.classList.remove('active');
        document.querySelectorAll('.song-checkbox').forEach(cb => cb.checked = false);
        updateBatchToolbar();
    }
}

function updateBatchToolbar() {
    const checkedBoxes = document.querySelectorAll('.song-checkbox:checked');
    const count = checkedBoxes.length;
    const selectAllCb = document.getElementById('select-all-checkbox');
    const batchSwitch = document.getElementById('btn-batch-switch');
    const batchDl = document.getElementById('btn-batch-dl');
    
    if(document.getElementById('selected-count')) {
        document.getElementById('selected-count').textContent = count;
    }
    
    const allBoxes = document.querySelectorAll('.song-checkbox');
    if (allBoxes.length > 0 && selectAllCb) {
        selectAllCb.checked = (allBoxes.length === count);
    }

    if (count > 0) {
        if(batchSwitch) batchSwitch.disabled = false;
        if(batchDl) batchDl.disabled = false;
    } else {
        if(batchSwitch) batchSwitch.disabled = true;
        if(batchDl) batchDl.disabled = true;
    }
    
    document.querySelectorAll('.song-card').forEach(card => card.classList.remove('selected'));
    checkedBoxes.forEach(cb => {
        cb.closest('.song-card').classList.add('selected');
    });
}

function toggleSelectAll(mainCb) {
    const checkboxes = document.querySelectorAll('.song-checkbox');
    checkboxes.forEach(cb => cb.checked = mainCb.checked);
    updateBatchToolbar();
}

function selectInvalidSongs() {
    const invalidTags = document.querySelectorAll('.tag-fail');
    if (invalidTags.length === 0) {
        alert('当前列表中没有检测到无效歌曲');
        return;
    }
    
    let count = 0;
    invalidTags.forEach(tag => {
        const card = tag.closest('.song-card');
        if (card) {
            const cb = card.querySelector('.song-checkbox');
            if (cb && !cb.checked) {
                cb.checked = true;
                count++;
            }
        }
    });
    
    if (count === 0) {
        alert('无效歌曲已全部选中');
    }
    updateBatchToolbar();
}

function getSelectedSongs() {
    const checkedBoxes = document.querySelectorAll('.song-checkbox:checked');
    const songs = [];
    checkedBoxes.forEach(cb => {
        const card = cb.closest('.song-card');
        if (card) {
            const ds = card.dataset;
            let coverUrl = '';
            const imgEl = card.querySelector('.cover-wrapper img');
            if (imgEl) coverUrl = imgEl.src;
            
            songs.push({
                id: ds.id,
                source: ds.source,
                name: ds.name,
                artist: ds.artist,
                url: buildDownloadURL(ds.id, ds.source, ds.name, ds.artist, ds.cover || '', ds.extra || ''),
                cover: coverUrl,
                lrc: `${API_ROOT}/lyric?id=${encodeURIComponent(ds.id)}&source=${ds.source}`,
                theme: '#10b981',
                custom_id: ds.id
            });
        }
    });
    return songs;
}

async function batchDownload() {
    const songs = getSelectedSongs();
    if (songs.length === 0) return;
    const batchDl = document.getElementById('btn-batch-dl');
    const batchSwitch = document.getElementById('btn-batch-switch');
    const originalBatchDlHTML = batchDl ? batchDl.innerHTML : '';

    if (webSettings.downloadToLocal) {
        if (!confirm(`准备将 ${songs.length} 首歌曲保存到本地目录：\n${webSettings.downloadDir}`)) {
            return;
        }
    } else {
        if (!confirm(`准备下载 ${songs.length} 首歌曲。\n下载会依次开始，请保持页面打开直到提示完成。`)) {
            return;
        }
    }

    if (batchDl) {
        batchDl.disabled = true;
        batchDl.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 下载中';
    }
    if (batchSwitch) {
        batchSwitch.disabled = true;
    }

    let success = 0;
    let warningCount = 0;
    const failures = [];

    try {
        for (const song of songs) {
            try {
                const result = webSettings.downloadToLocal
                    ? await requestLocalDownload(song.url)
                    : await requestBrowserDownload(song);
                success++;
                if (result && result.warning) {
                    warningCount++;
                }
            } catch (error) {
                failures.push({
                    song,
                    reason: (error && error.message) ? error.message : '下载失败'
                });
            }
        }

        let message = webSettings.downloadToLocal
            ? `本地保存完成，成功 ${success}/${songs.length}`
            : `批量下载已完成，成功 ${success}/${songs.length}`;

        if (webSettings.downloadToLocal) {
            message += `\n目录：${webSettings.downloadDir}`;
        }
        if (warningCount > 0) {
            message += `\n\n共 ${warningCount} 首触发了降级提示，请查看终端日志`;
        }
        message += buildBatchFailureMessage(failures, '失败');

        alert(message);
    } finally {
        if (batchDl) {
            batchDl.innerHTML = originalBatchDlHTML;
        }
        updateBatchToolbar();
        if (batchSwitch && document.querySelectorAll('.song-checkbox:checked').length === 0) {
            batchSwitch.disabled = true;
        }
    }
}

function batchSwitchSource() {
    const checkedBoxes = document.querySelectorAll('.song-checkbox:checked');
    if (checkedBoxes.length === 0) return;

    if (!confirm(`准备对 ${checkedBoxes.length} 首歌曲进行自动换源。\n这可能需要一些时间，请耐心等待。`)) {
        return;
    }

    checkedBoxes.forEach((cb, index) => {
        const card = cb.closest('.song-card');
        if (card) {
            const switchBtn = card.querySelector('.btn-switch');
            if (switchBtn) {
                setTimeout(() => {
                    switchSource(switchBtn);
                }, index * 1000); 
            }
        }
    });
}


// ==========================================
// 自制歌单 (本地收藏夹) 前端交互
// ==========================================

let pendingFavSong = null;

function playAllSongs() {
    const firstPlayBtn = document.querySelector('.song-card .btn-play');
    if (firstPlayBtn) {
        playAllAndJumpTo(firstPlayBtn);
    } else {
        alert('列表为空，无法播放');
    }
}

function openCollectionManager() {
    window.location.href = API_ROOT + '/my_collections';
}

function showEditCollectionModal(id = '', name = '', desc = '', cover = '') {
    document.getElementById('editColTitle').textContent = id ? '编辑歌单' : '新建歌单';
    document.getElementById('editColId').value = id;
    document.getElementById('editColName').value = name;
    document.getElementById('editColDesc').value = desc;
    
    if (cover && cover.includes('picsum.photos')) {
        document.getElementById('editColCover').value = '';
    } else {
        document.getElementById('editColCover').value = cover;
    }
    
    document.getElementById('editCollectionModal').style.display = 'flex';
}

function saveCollection() {
    const id = document.getElementById('editColId').value;
    const name = document.getElementById('editColName').value.trim();
    const desc = document.getElementById('editColDesc').value.trim();
    const cover = document.getElementById('editColCover').value.trim();
    
    if (!name) return alert('名称不能为空');
    
    const payload = { name, description: desc, cover };
    const isAddingSongModalOpen = document.getElementById('addToCollectionModal').style.display === 'flex';
    
    const url = id ? `${API_ROOT}/collections/${id}` : `${API_ROOT}/collections`;
    const method = id ? 'PUT' : 'POST';

    fetch(url, {
        method: method,
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(payload)
    }).then(r => r.json()).then(res => {
        if (res.error) return alert(res.error);
        
        document.getElementById('editCollectionModal').style.display = 'none';
        
        if (isAddingSongModalOpen) {
            refreshAddToCollectionList();
        } else {
            window.location.reload();
        }
    });
}

function deleteCollection(id) {
    if (!confirm('确定删除此歌单吗？内含歌曲记录也将被清空！')) return;
    fetch(`${API_ROOT}/collections/${id}`, { method: 'DELETE' })
        .then(r => r.json())
        .then(res => {
            if (res.error) return alert(res.error);
            window.location.reload();
        });
}

function deleteCollectionFromModal(id) {
    if (!confirm('确定删除此歌单吗？内含歌曲记录也将被清空！')) return;
    fetch(`${API_ROOT}/collections/${id}`, { method: 'DELETE' })
        .then(r => r.json())
        .then(res => {
            if (res.error) return alert(res.error);
            refreshAddToCollectionList();
        });
}

function refreshAddToCollectionList() {
    const container = document.getElementById('addColList');
    container.innerHTML = '<div style="text-align: center; color: #a0aec0; padding: 20px;">加载中...</div>';
    
    fetch(API_ROOT + '/collections')
        .then(r => r.json())
        .then(data => {
            if (!data || data.length === 0) {
                container.innerHTML = '<div style="text-align: center; color: #a0aec0; padding: 20px;">暂无歌单，请点击上方「新建」创建</div>';
                return;
            }
            container.innerHTML = '';
            data.forEach(col => {
                const item = document.createElement('div');
                item.className = 'collection-item';
                item.style.cursor = 'default'; 
                
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
                
                item.querySelector('.col-clickable-area').onclick = () => addSongToCollection(col.id);
                item.querySelector('.btn-edit').onclick = (e) => {
                    e.stopPropagation();
                    showEditCollectionModal(col.id, col.name, col.description || '', col.cover || '');
                };
                item.querySelector('.btn-del').onclick = (e) => {
                    e.stopPropagation();
                    deleteCollectionFromModal(col.id);
                };

                container.appendChild(item);
            });
        }).catch(() => {
            container.innerHTML = '<div style="text-align: center; color: #e53e3e; padding: 20px;">加载失败</div>';
        });
}

function openAddToCollectionModal(btn) {
    const card = btn.closest('.song-card');
    if (!card) return;
    
    let coverUrl = '';
    const imgEl = card.querySelector('.cover-wrapper img');
    if (imgEl) coverUrl = imgEl.src;

    pendingFavSong = {
        id: card.dataset.id,
        source: card.dataset.source,
        name: card.dataset.name,
        artist: card.dataset.artist,
        duration: parseInt(card.dataset.duration) || 0,
        cover: coverUrl,
        extra: { saved_from: "web_ui" }
    };
    
    document.getElementById('addToCollectionModal').style.display = 'flex';
    refreshAddToCollectionList();
}

function addSongToCollection(colId) {
    if (!pendingFavSong) return;
    
    fetch(`${API_ROOT}/collections/${colId}/songs`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(pendingFavSong)
    }).then(r => r.json()).then(res => {
        if (res.error) {
            alert(res.error);
        } else {
            alert('成功收藏至您的歌单！');
            document.getElementById('addToCollectionModal').style.display = 'none';
        }
    });
}

function removeSongFromCollection(btn, colId, originalSongId, originalSource) {
    if (!confirm('确定将此歌曲移出当前歌单吗？')) return;
    fetch(`${API_ROOT}/collections/${colId}/songs?id=${encodeURIComponent(originalSongId)}&source=${encodeURIComponent(originalSource)}`, { method: 'DELETE' })
        .then(r => r.json())
        .then(res => {
            if(res.error) return alert(res.error);
            const card = btn.closest('.song-card');
            if (card) {
                card.style.transition = 'all 0.3s';
                card.style.opacity = '0';
                card.style.transform = 'translateX(30px)';
                setTimeout(() => {
                    window.location.reload();
                }, 300);
            } else {
                window.location.reload();
            }
        });
}
