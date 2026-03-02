// templates/app.js

const API_ROOT = window.API_ROOT;

document.addEventListener('DOMContentLoaded', function() {
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
        setTimeout(() => inspectSong(card), index * 100);
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

    fetch(`${API_ROOT}/inspect?id=${encodeURIComponent(id)}&source=${source}&duration=${duration}`)
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

function openCookieModal() {
    document.getElementById('cookieModal').style.display = 'flex';
    fetch(API_ROOT + '/cookies').then(r => r.json()).then(data => {
        for (const [k, v] of Object.entries(data)) {
            const el = document.getElementById(`cookie-${k}`);
            if(el) el.value = v;
        }
    });
}

function saveCookies() {
    const data = {};
    document.querySelectorAll('input[id^="cookie-"]').forEach(input => {
        data[input.id.replace('cookie-', '')] = input.value;
    });
    fetch(API_ROOT + '/cookies', {
        method: 'POST', 
        body: JSON.stringify(data)
    }).then(() => {
        alert('保存成功！');
        document.getElementById('cookieModal').style.display = 'none';
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
        dl.href = `${API_ROOT}/download?id=${encodeURIComponent(song.id)}&source=${song.source}&name=${encodeURIComponent(song.name)}&artist=${encodeURIComponent(song.artist)}`;
        dl.id = `dl-${song.id}`;
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
    inspectSong(card);
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
        audio.url = `${API_ROOT}/download?id=${encodeURIComponent(newSong.id)}&source=${newSong.source}&name=${encodeURIComponent(newSong.name)}&artist=${encodeURIComponent(newSong.artist)}`;
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
            url: `${API_ROOT}/download?id=${encodeURIComponent(ds.id)}&source=${ds.source}&name=${encodeURIComponent(ds.name)}&artist=${encodeURIComponent(ds.artist)}`,
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
                url: `${API_ROOT}/download?id=${encodeURIComponent(ds.id)}&source=${ds.source}&name=${encodeURIComponent(ds.name)}&artist=${encodeURIComponent(ds.artist)}`,
                cover: coverUrl,
                lrc: `${API_ROOT}/lyric?id=${encodeURIComponent(ds.id)}&source=${ds.source}`,
                theme: '#10b981',
                custom_id: ds.id
            });
        }
    });
    return songs;
}

function batchDownload() {
    const songs = getSelectedSongs();
    if (songs.length === 0) return;

    if (!confirm(`准备下载 ${songs.length} 首歌曲。\n注意：浏览器可能会拦截多个弹窗，请务必允许本站点的弹窗！`)) {
        return;
    }

    songs.forEach((s, index) => {
        setTimeout(() => {
            const link = document.createElement('a');
            link.href = s.url;
            link.download = ''; 
            link.target = '_blank';
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
        }, index * 800); 
    });
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