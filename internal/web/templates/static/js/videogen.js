(function () {
    // =================================================================
    // 共享核心算法
    // =================================================================
    const FFT = {
        windowed: null, mags: null, previousMags: null,
        reset: function() { this.previousMags = null; },
        fft: function(data) {
            const n = data.length;
            if (n <= 1) return data;
            const half = n / 2, even = new Float32Array(half), odd = new Float32Array(half);
            for (let i = 0; i < half; i++) { even[i] = data[2 * i]; odd[i] = data[2 * i + 1]; }
            const q = this.fft(even), r = this.fft(odd), output = new Float32Array(n);
            for (let k = 0; k < half; k++) { const t = r[k]; output[k] = q[k] + t; output[k + half] = q[k] - t; }
            return output;
        },
        getFrequencyData: function(pcmData, fftSize, smoothing) {
            const half = fftSize / 2;
            if (!this.windowed || this.windowed.length !== fftSize) {
                this.windowed = new Float32Array(fftSize);
                this.mags = new Uint8Array(half);
                this.previousMags = new Float32Array(half);
            }
            for(let i=0; i<fftSize; i++) {
                const val = (i < pcmData.length) ? pcmData[i] : 0;
                this.windowed[i] = val * (0.5 * (1 - Math.cos(2 * Math.PI * i / (fftSize - 1))));
            }
            const rawFFT = this.fft(this.windowed);
            for(let i=0; i<half; i++) {
                let mag = Math.abs(rawFFT[i]) / fftSize;
                mag = mag * 2.0;
                mag = smoothing * this.previousMags[i] + (1 - smoothing) * mag;
                this.previousMags[i] = mag;
                let db = 20 * Math.log10(mag + 1e-6);
                const minDb = -100, maxDb = -10;
                let val = (db - minDb) * (255 / (maxDb - minDb));
                if(val < 0) val = 0; if(val > 255) val = 255;
                this.mags[i] = val;
            }
            return this.mags;
        }
    };

    function processVisualizerBars(freqData) {
        const barsCount = 180, barHeights = [];
        const maxIdx = Math.floor(freqData.length * 0.8), minIdx = 1; 
        for(let i=0; i<barsCount; i++) {
            const logRange = Math.log(maxIdx / minIdx);
            const idx = minIdx * Math.exp(logRange * (i / barsCount));
            const lower = Math.floor(idx), upper = Math.ceil(idx), frac = idx - lower;
            let val = (freqData[lower] || 0) * (1 - frac) + (freqData[upper] || 0) * frac;
            val *= 1 + (i / barsCount) * 0.8;
            if (val > 255) val = 255;
            let h = 2; 
            if (val > 0) h += Math.pow(val / 255.0, 2.5) * 40; 
            barHeights.push(h);
        }
        return { heights: barHeights };
    }

    function drawVisualizerRings(ctx, cx, cy, radius, heights) {
        ctx.save(); ctx.translate(cx, cy);
        const barsCount = heights.length, barWidth = 1.5, halfWidth = barWidth / 2;
        for (let i = 0; i < barsCount; i++) {
            ctx.save();
            ctx.rotate((Math.PI * 2 / barsCount) * i - Math.PI / 2);
            const h = heights[i] || 2, hue = (i / barsCount) * 360; 
            ctx.fillStyle = `hsla(${hue}, 100%, 65%, 0.9)`;
            ctx.beginPath();
            if (ctx.roundRect) ctx.roundRect(-halfWidth, -radius - h, barWidth, h, 0.5);
            else ctx.rect(-halfWidth, -radius - h, barWidth, h);
            ctx.fill(); ctx.restore(); 
        }
        ctx.restore(); 
    }

    // =================================================================
    // 独立新窗口渲染线程 (Worker 环境)
    // =================================================================
    if (window.isRenderWorker) {
        const fallbackLineDurationWorker = 1200;

        function lyricProgressWorker(nowMs, start, end) {
            if (nowMs <= start) return 0;
            if (!Number.isFinite(end) || end <= start) return 1;
            return Math.max(0, Math.min(1, (nowMs - start) / (end - start)));
        }

        function normalizeGroupWordsWorker(sourceWords, groupStart, groupEnd, fallbackText) {
            const words = Array.isArray(sourceWords) && sourceWords.length > 0
                ? sourceWords
                : [{ text: fallbackText || '', start: groupStart, end: groupEnd }];
            return words.map((word, index) => {
                const start = Number(word?.start);
                const nextStart = index + 1 < words.length ? Number(words[index + 1]?.start) : NaN;
                let end = Number(word?.end);
                const safeStart = Number.isFinite(start) ? start : groupStart;
                if (!Number.isFinite(end) || end <= safeStart) {
                    end = Number.isFinite(nextStart) && nextStart > safeStart ? nextStart : groupEnd;
                }
                return {
                    text: String(word?.text || ''),
                    start: safeStart,
                    end
                };
            }).filter(word => word.text !== '');
        }

        function normalizeLyricGroupsWorker(rawGroups) {
            return (rawGroups || []).map((group, index, list) => {
                const start = Number(group?.start || 0);
                const nextStart = index + 1 < list.length ? Number(list[index + 1]?.start || 0) : 0;
                const end = nextStart > start ? nextStart : start + fallbackLineDurationWorker;
                const lines = (group?.lines || []).map((line) => ({
                    ...line,
                    text: String(line?.text || ''),
                    words: normalizeGroupWordsWorker(line?.words, start, end, line?.text)
                }));
                return { start, end, time: start / 1000, lines };
            }).filter(group => group.lines.some(line => line.text));
        }

        function looksLikeRomajiLineWorker(line) {
            const text = String(line?.text || '').trim();
            if (!text) return false;
            const latinCount = (text.match(/[A-Za-z]/g) || []).length;
            const cjkOrKanaCount = (text.match(/[\u3040-\u30ff\u3400-\u9fff]/g) || []).length;
            return latinCount > 0 && latinCount >= cjkOrKanaCount;
        }

        function splitLyricGroupLinesWorker(lines) {
            const [orig, ...extras] = lines || [];
            let roma = null;
            let trans = null;
            extras.forEach((line) => {
                if (!roma && looksLikeRomajiLineWorker(line)) {
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

        function wrapPlainTextWorker(ctx, text, maxW) {
            const lines = [];
            let currentLine = '';
            const chars = Array.from(String(text || ''));
            for (let i = 0; i < chars.length; i++) {
                const next = currentLine + chars[i];
                if (ctx.measureText(next).width > maxW && currentLine.length > 0) {
                    if (/[a-zA-Z]/.test(chars[i]) && currentLine.includes(' ')) {
                        const lastSpace = currentLine.lastIndexOf(' ');
                        lines.push(currentLine.substring(0, lastSpace));
                        currentLine = currentLine.substring(lastSpace + 1) + chars[i];
                    } else {
                        lines.push(currentLine);
                        currentLine = chars[i];
                    }
                } else {
                    currentLine = next;
                }
            }
            if (currentLine) lines.push(currentLine);
            return lines;
        }

        function wrapWordSegmentsWorker(ctx, words, maxW) {
            const lines = [];
            let currentLine = [];
            let currentWidth = 0;
            words.forEach((word) => {
                const width = ctx.measureText(word.text || '').width;
                if (currentLine.length > 0 && currentWidth + width > maxW) {
                    lines.push(currentLine);
                    currentLine = [];
                    currentWidth = 0;
                }
                currentLine.push(word);
                currentWidth += width;
            });
            if (currentLine.length > 0) lines.push(currentLine);
            return lines;
        }

        function createLineOnlyGroupsWorker(lyricRaw) {
            return normalizeLyricGroupsWorker((lyricRaw || []).map((item) => ({
                start: Math.round((Number(item?.time) || 0) * 1000),
                lines: [{ text: String(item?.text || ''), words: [] }]
            })));
        }

        async function runOfflineRender(data) {
            const apiRoot = data.apiRoot;
            const statusText = document.getElementById("status-text");
            const progressFill = document.getElementById("progress-fill");
            const titleEl = document.getElementById("title");
            const previewCanvas = document.getElementById("preview-canvas");
            
            const setStatus = (title, desc, pct) => {
                if(title) titleEl.textContent = title;
                if(desc) statusText.textContent = desc;
                if(pct !== undefined) progressFill.style.width = pct + "%";
            };

            try {
                let initRes;
                let audioBuffer;
                const audioCtx = new (window.AudioContext || window.webkitAudioContext)();

                if (data.customAudioFile) {
                    setStatus("正在初始化...", "正在向服务器投递您的本地音乐...", 5);
                    const fd = new FormData();
                    fd.append("id", data.id);
                    fd.append("source", data.source);
                    fd.append("audio_file", data.customAudioFile);

                    initRes = await fetch(`${apiRoot}/videogen/init`, {
                        method: "POST",
                        body: fd
                    }).then(r => r.json());
                    if (initRes.error) throw new Error(initRes.error);

                    setStatus("解码音频...", "解析本地高清音频数据...", 15);
                    const arr = await data.customAudioFile.arrayBuffer();
                    audioBuffer = await audioCtx.decodeAudioData(arr);
                } else {
                    setStatus("正在初始化...", "下载音频与初始化并行中...", 5);
                    const audioDownloadUrl = `${apiRoot}/download?id=${encodeURIComponent(data.id)}&source=${encodeURIComponent(data.source)}`;
                    const [initResult, audioArr] = await Promise.all([
                        fetch(`${apiRoot}/videogen/init`, {
                            method: "POST", headers: { "Content-Type": "application/json" },
                            body: JSON.stringify({ id: data.id, source: data.source }),
                        }).then(r => r.json()),
                        fetch(audioDownloadUrl).then(r => r.arrayBuffer())
                    ]);
                    initRes = initResult;
                    if (initRes.error) throw new Error(initRes.error);

                    setStatus("解码音频...", "解析音频数据...", 15);
                    audioBuffer = await audioCtx.decodeAudioData(audioArr);
                }
                    
                setStatus("加载视觉资源...", "准备 1080P 超清渲染画板", 25);
                
                const logicalW = 1280, logicalH = 720, scaleFactor = 1.5; 
                const width = logicalW * scaleFactor, height = logicalH * scaleFactor;
                
                const canvas = document.createElement("canvas");
                canvas.width = width; canvas.height = height;
                const ctx = canvas.getContext("2d");
                
                previewCanvas.width = width; previewCanvas.height = height;
                const previewCtx = previewCanvas.getContext("2d");

                let bgMedia = null;
                if (data.isVideoBg) {
                    bgMedia = document.createElement("video");
                    bgMedia.src = data.rawCover; bgMedia.muted = true; bgMedia.loop = true;
                    bgMedia.setAttribute('playsinline', ''); 
                    await bgMedia.play(); bgMedia.pause(); 
                } else {
                    bgMedia = new Image(); bgMedia.crossOrigin = "Anonymous";
                    let coverSrc = data.rawCover;
                    if (!data.rawCover.startsWith("data:")) coverSrc = `${apiRoot}/download_cover?url=${encodeURIComponent(data.rawCover)}&name=render&artist=render`;
                    await Promise.race([
                        new Promise(r => { bgMedia.onload = r; bgMedia.onerror = () => { bgMedia.src = "https://via.placeholder.com/600"; setTimeout(r, 1000); }; bgMedia.src = coverSrc; }),
                        new Promise((_, r) => setTimeout(() => r(new Error("资源加载超时")), 15000))
                    ]);
                }
                
                const fps = 30;
                const duration = audioBuffer.duration;
                const totalFrames = Math.floor(duration * fps);
                const rawData = audioBuffer.getChannelData(0);
                const samplesPerFrame = Math.floor(audioBuffer.sampleRate / fps);
                const batchSize = 30; 
                const lyricGroups = Array.isArray(data.lyricGroups) && data.lyricGroups.length > 0
                    ? normalizeLyricGroupsWorker(data.lyricGroups)
                    : createLineOnlyGroupsWorker(data.lyricRaw);
                const renderKaraoke = data.lyricMode === 'karaoke' && lyricGroups.length > 0;
                 
                FFT.reset();
                setStatus("超清渲染中", "0%", 30);
                
                const canvasToJpegBlob = (targetCanvas, quality) => new Promise((resolve, reject) => {
                    if (targetCanvas.toBlob) {
                        targetCanvas.toBlob((blob) => {
                            if (blob) resolve(blob);
                            else reject(new Error("Frame encode failed"));
                        }, "image/jpeg", quality);
                        return;
                    }

                    try {
                        const dataUrl = targetCanvas.toDataURL("image/jpeg", quality);
                        const payload = dataUrl.split(",")[1] || "";
                        const binary = atob(payload);
                        const bytes = new Uint8Array(binary.length);
                        for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
                        resolve(new Blob([bytes], { type: "image/jpeg" }));
                    } catch (err) {
                        reject(err);
                    }
                });

                const uploadBatch = async (frames, startIdx) => {
                    const form = new FormData();
                    form.append("session_id", initRes.session_id);
                    form.append("start_idx", String(startIdx));
                    frames.forEach((blob, index) => {
                        const frameNum = String(startIdx + index).padStart(5, "0");
                        form.append("frames", blob, `frame_${frameNum}.jpg`);
                    });

                    const res = await fetch(`${apiRoot}/videogen/frame`, {
                        method: "POST",
                        body: form
                    });
                    const body = await res.json().catch(() => ({}));
                    if (!res.ok || body.error) {
                        throw new Error(body.error || `Frame upload failed: ${res.status}`);
                    }
                };
                
                const seekVideo = async (time) => {
                    if (!data.isVideoBg || !bgMedia.duration) return;
                    const tt = time % bgMedia.duration;
                    bgMedia.currentTime = tt;
                    if (Math.abs(bgMedia.currentTime - tt) < 0.1 && bgMedia.readyState >= 3) return;
                    await new Promise(r => {
                        const onSeeked = () => { bgMedia.removeEventListener('seeked', onSeeked); r(); };
                        setTimeout(() => { bgMedia.removeEventListener('seeked', onSeeked); r(); }, 500); 
                        bgMedia.addEventListener('seeked', onSeeked);
                    });
                };

                const drawWrappedLines = (lines, x, startY, lineHeight, color, alpha) => {
                    ctx.fillStyle = color;
                    let y = startY + lineHeight / 2;
                    for (const lineText of lines) {
                        ctx.globalAlpha = alpha;
                        ctx.fillText(lineText, x, y);
                        y += lineHeight;
                    }
                    ctx.globalAlpha = 1;
                    return startY + (lines.length * lineHeight);
                };

                const karaokeTextColor = "#ffffff";
                const karaokeAccentColor = "#12bd85";
                const karaokeStrokeText = (text, x, y, lineHeight, fillColor, strokeColor, alpha) => {
                    ctx.save();
                    ctx.globalAlpha = alpha;
                    ctx.lineJoin = "round";
                    ctx.lineWidth = Math.max(2, lineHeight * 0.08);
                    ctx.strokeStyle = strokeColor;
                    ctx.fillStyle = fillColor;
                    ctx.strokeText(text, x, y);
                    ctx.fillText(text, x, y);
                    ctx.restore();
                };

                const drawKaraokeWordLine = (words, x, y, lineHeight, nowMs, baseColor, fillColor, alpha) => {
                    ctx.lineJoin = "round"; // 确保边缘绝对圆润无尖刺
                    ctx.lineWidth = Math.max(3, lineHeight * 0.12); // 稍微加粗，还原图1厚实感
                    ctx.globalAlpha = alpha;
                    const strokeSpill = ctx.lineWidth; // 计算边框向外溢出的安全区距离

                    let cursorX = x;
                    
                    // 第1层：先画一整句的【底层绿边】
                    ctx.strokeStyle = karaokeAccentColor;
                    words.forEach((word) => {
                        const text = String(word?.text || '');
                        if (text) { ctx.strokeText(text, cursorX, y); cursorX += ctx.measureText(text).width; }
                    });

                    // 第2层：再画一整句的【底层白字】（字压在边上，内部绝对纯净无色块）
                    cursorX = x;
                    ctx.fillStyle = baseColor;
                    words.forEach((word) => {
                        const text = String(word?.text || '');
                        if (text) { ctx.fillText(text, cursorX, y); cursorX += ctx.measureText(text).width; }
                    });

                    // 高级裁剪层：精确切出当前的进度光束
                    ctx.save();
                    ctx.beginPath();
                    cursorX = x;
                    words.forEach((word) => {
                        const text = String(word?.text || '');
                        if (!text) return;
                        const width = ctx.measureText(text).width;
                        const progress = lyricProgressWorker(nowMs, Number(word.start || 0), Number(word.end || 0));
                        
                        if (progress > 0) {
                            // 核心修复：100%时刻意放宽右侧裁剪区，且向左延伸防止切掉首字描边
                            const clipRight = progress === 1 ? width + strokeSpill : width * progress;
                            ctx.rect(cursorX - strokeSpill, y - lineHeight, strokeSpill + clipRight, lineHeight * 2);
                        }
                        cursorX += width;
                    });
                    ctx.clip();

                    // 第3层：在进度裁剪区内画【高亮白边】
                    cursorX = x;
                    ctx.strokeStyle = karaokeTextColor;
                    words.forEach((word) => {
                        const text = String(word?.text || '');
                        if (text) { ctx.strokeText(text, cursorX, y); cursorX += ctx.measureText(text).width; }
                    });

                    // 第4层：在进度裁剪区内画【高亮绿字】
                    cursorX = x;
                    ctx.fillStyle = fillColor;
                    words.forEach((word) => {
                        const text = String(word?.text || '');
                        if (text) { ctx.fillText(text, cursorX, y); cursorX += ctx.measureText(text).width; }
                    });

                    ctx.restore();
                    ctx.globalAlpha = 1;
                };

                const drawLineLyrics = (time, lx, baseLy, maxWidth, gap) => {
                    let activeIdx = -1;
                    for (let i = 0; i < data.lyricRaw.length; i++) {
                        if (time >= data.lyricRaw[i].time) activeIdx = i;
                        else break;
                    }
                    if (activeIdx === -1) return;

                    let lyricsBlocks = [];
                    let activeBlockIndex = -1;
                    for (let offset = -4; offset <= 4; offset++) {
                        const idx = activeIdx + offset;
                        if (idx >= 0 && idx < data.lyricRaw.length) {
                            const isCurrent = offset === 0;
                            ctx.font = isCurrent ? "bold 36px sans-serif" : "600 26px sans-serif";
                            const lineHeight = isCurrent ? 48 : 34;
                            const textLines = wrapPlainTextWorker(ctx, data.lyricRaw[idx].text, maxWidth);
                            const blockHeight = (textLines.length - 1) * lineHeight;

                            lyricsBlocks.push({
                                textLines,
                                isCurrent,
                                lineHeight,
                                blockHeight,
                                font: ctx.font,
                                color: isCurrent ? "#ffffff" : "rgba(255,255,255,0.85)",
                                shadowBlur: isCurrent ? 6 : 4,
                                shadowOffset: isCurrent ? 2 : 1
                            });
                            if (isCurrent) activeBlockIndex = lyricsBlocks.length - 1;
                        }
                    }

                    if (activeBlockIndex === -1) return;
                    const activeBlock = lyricsBlocks[activeBlockIndex];
                    activeBlock.startY = baseLy - (activeBlock.blockHeight / 2);
                    for (let i = activeBlockIndex + 1; i < lyricsBlocks.length; i++) {
                        const prev = lyricsBlocks[i - 1];
                        lyricsBlocks[i].startY = prev.startY + prev.blockHeight + gap + (prev.lineHeight / 2) + (lyricsBlocks[i].lineHeight / 2);
                    }
                    for (let i = activeBlockIndex - 1; i >= 0; i--) {
                        const next = lyricsBlocks[i + 1];
                        lyricsBlocks[i].startY = next.startY - lyricsBlocks[i].blockHeight - gap - (next.lineHeight / 2) - (lyricsBlocks[i].lineHeight / 2);
                    }

                    for (const block of lyricsBlocks) {
                        ctx.font = block.font;
                        ctx.fillStyle = block.color;
                        ctx.shadowColor = "rgba(0,0,0,0.9)";
                        ctx.shadowBlur = block.shadowBlur;
                        ctx.shadowOffsetX = block.shadowOffset;
                        ctx.shadowOffsetY = block.shadowOffset;
                        let lineY = block.startY;
                        for (const lineText of block.textLines) {
                            let alpha = 1;
                            const dist = Math.abs(lineY - baseLy);
                            if (dist > 230) alpha = Math.max(0, 1 - (dist - 230) / 70);
                            if (alpha > 0) {
                                ctx.globalAlpha = alpha;
                                ctx.fillText(lineText, lx, lineY);
                                ctx.globalAlpha = 1;
                            }
                            lineY += block.lineHeight;
                        }
                    }
                };

                const drawKaraokeLyrics = (timeMs, lx, baseLy, maxWidth) => {
                    const karaokeFillColor = karaokeAccentColor;
                    const createLineLayout = (line, font, lineHeight, useWordProgress) => {
                        if (!line?.text) {
                            return { useWordProgress: false, wordLines: [], textLines: [], lineHeight, height: 0 };
                        }
                        ctx.font = font;
                        if (useWordProgress && Array.isArray(line.words) && line.words.length > 0) {
                            const wordLines = wrapWordSegmentsWorker(ctx, line.words, maxWidth);
                            const textLines = wordLines.map((lineWords) => lineWords.map((word) => word.text).join(''));
                            return {
                                useWordProgress: true,
                                wordLines,
                                textLines,
                                lineHeight,
                                height: textLines.length * lineHeight
                            };
                        }
                        const textLines = wrapPlainTextWorker(ctx, line.text, maxWidth);
                        return {
                            useWordProgress: false,
                            wordLines: [],
                            textLines,
                            lineHeight,
                            height: textLines.length * lineHeight
                        };
                    };
                    const drawLineLayout = (layout, x, startY, font, now, baseColor, alpha, isCurrent) => {
                        if (!layout || layout.textLines.length === 0) return startY;
                        ctx.font = font;
                        if (layout.useWordProgress && isCurrent) {
                            layout.wordLines.forEach((lineWords, lineIndex) => {
                                const y = startY + (lineIndex * layout.lineHeight) + layout.lineHeight / 2;
                                drawKaraokeWordLine(lineWords, x, y, layout.lineHeight, now, baseColor, karaokeFillColor, alpha);
                            });
                        } else {
                            layout.textLines.forEach((lineText, lineIndex) => {
                                const y = startY + (lineIndex * layout.lineHeight) + layout.lineHeight / 2;
                                karaokeStrokeText(lineText, x, y, layout.lineHeight, baseColor, karaokeAccentColor, alpha);
                            });
                        }
                        return startY + layout.height;
                    };
                    let activeIdx = -1;
                    for (let i = 0; i < lyricGroups.length; i++) {
                        if (timeMs >= lyricGroups[i].start) activeIdx = i;
                        else break;
                    }
                    if (activeIdx === -1) return;

                    const blocks = [];
                    let currentBlockIndex = -1;
                    for (let offset = -2; offset <= 2; offset++) {
                        const idx = activeIdx + offset;
                        if (idx < 0 || idx >= lyricGroups.length) continue;

                        const group = lyricGroups[idx];
                        const { orig, roma, trans } = splitLyricGroupLinesWorker(group.lines);
                        if (!orig) continue;

                        const isCurrent = offset === 0;
                        const blockAlpha = isCurrent ? 1 : 0.72;
                        const origFont = isCurrent ? "bold 40px sans-serif" : "700 28px sans-serif";
                        const origLineHeight = isCurrent ? 52 : 38;
                        const subGap = isCurrent ? 10 : 8;
                        const transFont = isCurrent ? "600 24px sans-serif" : "500 18px sans-serif";
                        const transLineHeight = isCurrent ? 30 : 22;
                        const romaFont = isCurrent ? "500 20px sans-serif" : "500 16px sans-serif";
                        const romaLineHeight = isCurrent ? 26 : 20;

                        const origLayout = createLineLayout(orig, origFont, origLineHeight, true);
                        const romaLayout = createLineLayout(roma, romaFont, romaLineHeight, !!roma?.verbatim);
                        const transLayout = createLineLayout(trans, transFont, transLineHeight, !!trans?.verbatim);

                        const blockHeight =
                            origLayout.height +
                            (romaLayout.height > 0 ? (subGap + romaLayout.height) : 0) +
                            (transLayout.height > 0 ? (subGap + transLayout.height) : 0);

                        blocks.push({
                            isCurrent,
                            alpha: blockAlpha,
                            origFont,
                            origLayout,
                            transFont,
                            transLayout,
                            romaFont,
                            romaLayout,
                            blockHeight,
                            subGap
                        });
                        if (isCurrent) currentBlockIndex = blocks.length - 1;
                    }

                    if (currentBlockIndex === -1) return;
                    const blockGap = 28;
                    blocks[currentBlockIndex].topY = baseLy - (blocks[currentBlockIndex].blockHeight / 2);
                    for (let i = currentBlockIndex + 1; i < blocks.length; i++) {
                        const prev = blocks[i - 1];
                        blocks[i].topY = prev.topY + prev.blockHeight + blockGap;
                    }
                    for (let i = currentBlockIndex - 1; i >= 0; i--) {
                        const next = blocks[i + 1];
                        blocks[i].topY = next.topY - blocks[i].blockHeight - blockGap;
                    }

                    blocks.forEach((block) => {
                        ctx.shadowColor = "rgba(0,0,0,0.9)";
                        ctx.shadowBlur = block.isCurrent ? 8 : 6;
                        ctx.shadowOffsetX = 2;
                        ctx.shadowOffsetY = 2;

                        let currentY = block.topY;

                        currentY = drawLineLayout(
                            block.origLayout,
                            lx,
                            currentY,
                            block.origFont,
                            timeMs,
                            karaokeTextColor,
                            block.alpha,
                            block.isCurrent
                        );

                        if (block.romaLayout.height > 0) {
                            currentY += block.subGap;
                            currentY = drawLineLayout(
                                block.romaLayout,
                                lx,
                                currentY,
                                block.romaFont,
                                timeMs,
                                karaokeTextColor,
                                block.alpha,
                                block.isCurrent
                            );
                        }

                        if (block.transLayout.height > 0) {
                            currentY += block.subGap;
                            drawLineLayout(
                                block.transLayout,
                                lx,
                                currentY,
                                block.transFont,
                                timeMs,
                                karaokeTextColor,
                                block.alpha,
                                block.isCurrent
                            );
                        }
                    });
                };
                 
                const drawFrame = async (frameIdx) => {
                    const time = frameIdx / fps;
                    if (data.isVideoBg) await seekVideo(time);
          
                    const fftSize = 2048; 
                    const startSample = Math.max(0, Math.floor((frameIdx * samplesPerFrame) - (fftSize / 4))); 
                    
                    let pcmSlice = rawData.subarray(startSample, startSample + fftSize);
                    if (pcmSlice.length < fftSize) {
                        const padded = new Float32Array(fftSize);
                        padded.set(pcmSlice); pcmSlice = padded;
                    }
                    
                    const freqData = FFT.getFrequencyData(pcmSlice, fftSize, 0.65);
                    const visResult = processVisualizerBars(freqData);
          
                    ctx.clearRect(0, 0, width, height); 
                    ctx.save(); ctx.scale(scaleFactor, scaleFactor);
                    
                    let mw = data.isVideoBg ? bgMedia.videoWidth : bgMedia.width;
                    let mh = data.isVideoBg ? bgMedia.videoHeight : bgMedia.height;
                    if (!mw) mw = logicalW; if (!mh) mh = logicalH; 
          
                    const baseRatio = Math.max(logicalW / mw, logicalH / mh);
                    let imgScale = 1.0;
                    if (!data.isVideoBg) {
                        const cycle = 20, progress = (time % (cycle * 2)) / cycle, ease = progress < 1 ? progress : 2 - progress; 
                        imgScale = 1.0 + (ease * ease * (3 - 2 * ease) * 0.1);
                    }
                    
                    const finalRatio = baseRatio * imgScale;
                    const bgW = mw * finalRatio, bgH = mh * finalRatio;
                    const bgX = (logicalW - bgW) / 2, bgY = (logicalH - bgH) / 2;
                    
                    ctx.drawImage(bgMedia, bgX, bgY, bgW, bgH);
          
                    const cx = 320, cy = logicalH / 2, discRadius = 200, barBaseRadius = discRadius + 2; 
                    drawVisualizerRings(ctx, cx, cy, barBaseRadius, visResult.heights);
        
                    ctx.save(); ctx.translate(cx, cy);
                    ctx.beginPath(); ctx.arc(0, 0, discRadius, 0, Math.PI * 2); ctx.fillStyle = "#111"; ctx.fill();
                    ctx.strokeStyle = "rgba(255,255,255,0.1)"; ctx.lineWidth = 4; ctx.stroke();
                    
                    const grad = ctx.createRadialGradient(0,0,discRadius*0.5, 0,0,discRadius);
                    grad.addColorStop(0, '#1a1a1a'); grad.addColorStop(0.5, '#222'); grad.addColorStop(1, '#111');
                    ctx.fillStyle = grad; ctx.fill();
          
                    ctx.save(); ctx.rotate(time * 0.4); ctx.beginPath(); ctx.arc(0, 0, coverRadius = discRadius * 0.65, 0, Math.PI * 2); ctx.clip(); 
                    ctx.drawImage(bgMedia, 0, 0, mw, mh, -coverRadius, -coverRadius, coverRadius * 2, coverRadius * 2); ctx.restore();
                    ctx.restore(); 
          
                    const lx = 600, baseLy = logicalH / 2, maxWidth = logicalW - lx - 40, gap = 20;
                    ctx.textAlign = "left";
                    ctx.textBaseline = "middle";
                    if (renderKaraoke) drawKaraokeLyrics(time * 1000, lx, baseLy, maxWidth);
                    else drawLineLyrics(time, lx, baseLy, maxWidth, gap);
                    
                    ctx.font = "bold 26px sans-serif"; ctx.fillStyle = "#fff"; ctx.textAlign = "center";
                    ctx.shadowColor = "rgba(0,0,0,0.9)"; ctx.shadowBlur = 8;
                    ctx.fillText(data.name, cx, logicalH - 50);
                    ctx.font = "18px sans-serif"; ctx.fillStyle = "rgba(255,255,255,0.9)";
                    ctx.fillText(data.artist, cx, logicalH - 20);
                    
                    ctx.restore(); 
                    
                    if (frameIdx % 10 === 0 || frameIdx === totalFrames - 1) {
                        previewCtx.clearRect(0,0,width,height);
                        previewCtx.drawImage(canvas, 0, 0);
                    }
                };
                
                let frameIdx = 0;
                let uploadPromise = Promise.resolve();
                const renderStartTime = performance.now();
                while (frameIdx < totalFrames) {
                  let framesBuffer = [];
                  const batchStartIdx = frameIdx;
                  for (let i = 0; i < batchSize && frameIdx < totalFrames; i++) {
                    await drawFrame(frameIdx);
                    framesBuffer.push(await canvasToJpegBlob(canvas, 0.92));
                    frameIdx++;
                  }
                  await uploadPromise;
                  uploadPromise = uploadBatch(framesBuffer, batchStartIdx);
                  const pct = Math.round((frameIdx / totalFrames) * 100);
                  const elapsed = (performance.now() - renderStartTime) / 1000;
                  const eta = frameIdx > 0 ? Math.round(elapsed / frameIdx * (totalFrames - frameIdx)) : 0;
                  const etaMin = Math.floor(eta / 60);
                  const etaSec = eta % 60;
                  const etaStr = etaMin > 0 ? `${etaMin}分${etaSec}秒` : `${etaSec}秒`;
                  setStatus("超清帧渲染中...", `已完成 ${pct}%  (${frameIdx}/${totalFrames} 帧)  预计剩余 ${etaStr}`, 30 + pct * 0.65);
                }
                await uploadPromise;
                
                setStatus("正在合成最终视频...", "合并无损音频与画面帧", 98);
                const finalRes = await fetch(`${apiRoot}/videogen/finish`, {
                  method: "POST", headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ session_id: initRes.session_id, name: `${data.name} - ${data.artist}` }),
                }).then(r => r.json());
        
                if (finalRes.error) throw new Error(finalRes.error);
                
                document.getElementById('loading-ui').style.display = 'none';
                document.getElementById('success-ui').style.display = 'block';
                document.getElementById("dl-link").href = apiRoot + finalRes.url;
                document.getElementById("dl-link").download = `${data.name}.mp4`;
                
            } catch(e) {
                console.error(e);
                document.getElementById('loading-ui').style.display = 'none';
                document.getElementById('error-ui').style.display = 'block';
                document.getElementById('error-text').textContent = e.message;
            }
        }
        
        runOfflineRender(window.renderData);
        return; 
    }

    // =================================================================
    // 网页主播放界面
    // =================================================================
    const lyricTimeRe = /\[(\d+):(\d+)\.(\d{1,3})\]/g;
    const fallbackLineDuration = 1200;

    function escapeHTML(value) {
        return String(value ?? '').replace(/[&<>"']/g, (char) => {
            switch (char) {
            case '&':
                return '&amp;';
            case '<':
                return '&lt;';
            case '>':
                return '&gt;';
            case '"':
                return '&quot;';
            case '\'':
                return '&#39;';
            default:
                return char;
            }
        });
    }

    function lyricTimeToMs(parts) {
        const minute = Number(parts[1]) || 0;
        const second = Number(parts[2]) || 0;
        let ms = String(parts[3] || '0');
        if (ms.length === 1) ms += '00';
        if (ms.length === 2) ms += '0';
        return minute * 60000 + second * 1000 + Number(ms.slice(0, 3));
    }

    function lyricProgress(nowMs, start, end) {
        if (nowMs <= start) return 0;
        if (!Number.isFinite(end) || end <= start) return 1;
        return Math.max(0, Math.min(1, (nowMs - start) / (end - start)));
    }

    function parseLyricLine(line) {
        lyricTimeRe.lastIndex = 0;
        const matches = Array.from(String(line || '').matchAll(lyricTimeRe));
        if (matches.length === 0) return null;

        const start = lyricTimeToMs(matches[0]);
        const words = [];
        for (let i = 0; i < matches.length; i++) {
            const textStart = matches[i].index + matches[i][0].length;
            const textEnd = i + 1 < matches.length ? matches[i + 1].index : line.length;
            const text = line.slice(textStart, textEnd);
            if (!text) continue;
            words.push({
                start: lyricTimeToMs(matches[i]),
                end: i + 1 < matches.length ? lyricTimeToMs(matches[i + 1]) : null,
                text
            });
        }

        const text = line.replace(lyricTimeRe, '').trim();
        return { start, time: start / 1000, words, text, verbatim: matches.length > 1 };
    }

    function normalizeGroupWords(sourceWords, groupStart, groupEnd, fallbackText) {
        const words = Array.isArray(sourceWords) && sourceWords.length > 0
            ? sourceWords
            : [{ text: fallbackText || '', start: groupStart, end: groupEnd }];
        return words
            .map((word, index) => {
                const start = Number(word?.start);
                const nextStart = index + 1 < words.length ? Number(words[index + 1]?.start) : NaN;
                let end = Number(word?.end);
                const safeStart = Number.isFinite(start) ? start : groupStart;
                if (!Number.isFinite(end) || end <= safeStart) {
                    end = Number.isFinite(nextStart) && nextStart > safeStart ? nextStart : groupEnd;
                }
                return {
                    text: String(word?.text || ''),
                    start: safeStart,
                    end
                };
            })
            .filter(word => word.text !== '');
    }

    function normalizeLyricGroups(rawGroups) {
        return (rawGroups || []).map((group, index, list) => {
            const start = Number(group?.start || 0);
            const nextStart = index + 1 < list.length ? Number(list[index + 1]?.start || 0) : 0;
            const end = nextStart > start ? nextStart : start + fallbackLineDuration;
            const lines = (group?.lines || []).map((line) => ({
                ...line,
                text: String(line?.text || ''),
                words: normalizeGroupWords(line?.words, start, end, line?.text)
            }));
            return {
                start,
                end,
                time: start / 1000,
                lines
            };
        }).filter(group => group.lines.some(line => line.text));
    }

    function looksLikeRomajiLine(line) {
        const text = String(line?.text || '').trim();
        if (!text) return false;
        const latinCount = (text.match(/[A-Za-z]/g) || []).length;
        const cjkOrKanaCount = (text.match(/[\u3040-\u30ff\u3400-\u9fff]/g) || []).length;
        return latinCount > 0 && latinCount >= cjkOrKanaCount;
    }

    function splitLyricGroupLines(lines) {
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

    function wrapPlainText(ctx, text, maxW) {
        const lines = [];
        let currentLine = '';
        const chars = Array.from(String(text || ''));
        for (let i = 0; i < chars.length; i++) {
            const next = currentLine + chars[i];
            if (ctx.measureText(next).width > maxW && currentLine.length > 0) {
                if (/[a-zA-Z]/.test(chars[i]) && currentLine.includes(' ')) {
                    const lastSpace = currentLine.lastIndexOf(' ');
                    lines.push(currentLine.substring(0, lastSpace));
                    currentLine = currentLine.substring(lastSpace + 1) + chars[i];
                } else {
                    lines.push(currentLine);
                    currentLine = chars[i];
                }
            } else {
                currentLine = next;
            }
        }
        if (currentLine) lines.push(currentLine);
        return lines;
    }

    function wrapWordSegments(ctx, words, maxW) {
        const lines = [];
        let currentLine = [];
        let currentWidth = 0;
        words.forEach((word) => {
            const width = ctx.measureText(word.text || '').width;
            if (currentLine.length > 0 && currentWidth + width > maxW) {
                lines.push(currentLine);
                currentLine = [];
                currentWidth = 0;
            }
            currentLine.push(word);
            currentWidth += width;
        });
        if (currentLine.length > 0) lines.push(currentLine);
        return lines;
    }

    function createLineOnlyGroups(lyricRaw) {
        return normalizeLyricGroups((lyricRaw || []).map((item) => ({
            start: Math.round((Number(item?.time) || 0) * 1000),
            lines: [{ text: String(item?.text || ''), words: [] }]
        })));
    }

    function parseLyrics(raw) {
        const map = new Map();
        let hasVerbatim = false;
        String(raw || '').split(/\r?\n/).forEach((rawLine) => {
            const line = rawLine.trim();
            if (!line || /^\[[A-Za-z]+:[^\]]*\]$/.test(line)) return;
            const parsed = parseLyricLine(line);
            if (!parsed || !parsed.text) return;
            hasVerbatim = hasVerbatim || parsed.verbatim;
            if (!map.has(parsed.start)) {
                map.set(parsed.start, { start: parsed.start, time: parsed.time, lines: [] });
            }
            map.get(parsed.start).lines.push(parsed);
        });

        const groups = normalizeLyricGroups(Array.from(map.values()).sort((a, b) => a.start - b.start));
        const hasMultiLang = groups.some(group => group.lines.length > 1);
        return {
            type: hasVerbatim || hasMultiLang ? 'karaoke' : 'line',
            groups
        };
    }

    function renderKaraokeWordsHTML(words, fallbackStart, fallbackEnd) {
        return (words || []).map((word) => [
            `<span class="vg-word" data-start="${word.start || fallbackStart}" data-end="${word.end || fallbackEnd}" style="--karaoke-progress:0%;">`,
            `<span class="vg-word-base">${escapeHTML(word.text)}</span>`,
            `<span class="vg-word-fill">${escapeHTML(word.text)}</span>`,
            '</span>'
        ].join('')).join('');
    }

    function renderKaraokeLineHTML(line, className, group, useWordProgress) {
        if (!line?.text) return '';
        const content = useWordProgress && Array.isArray(line.words) && line.words.length > 0
            ? renderKaraokeWordsHTML(line.words, group.start, group.end)
            : escapeHTML(line.text);
        return `<div class="${className}">${content}</div>`;
    }

    function buildVideoGenLyricURL(songData) {
        if (window.buildLyricRequestURL) {
            return window.buildLyricRequestURL(songData, 'lyric', 'auto');
        }

        const params = new URLSearchParams({
            id: String(songData?.id || ''),
            source: String(songData?.source || ''),
            name: String(songData?.name || ''),
            artist: String(songData?.artist || ''),
            album: String(songData?.album || ''),
            duration: String(songData?.duration || 0),
            format: 'auto'
        });
        const extra = typeof songData?.extra === 'string'
            ? songData.extra
            : JSON.stringify(songData?.extra || {});
        if (extra && extra !== '{}' && extra !== 'null') {
            params.set('extra', extra);
        }
        return `${window.API_ROOT}/lyric?${params.toString()}`;
    }

    window.VideoGen = {
      data: null, customVisual: null, lyricTimes: [], lyricRaw: [], lyricGroups: [], lyricMode: 'line', lastActiveIndex: -1,
      audioCtx: null, analyser: null, sourceNode: null, localSourceNode: null, 
      isPlaying: false, rtCanvas: null, rtCtx: null, animationId: null, isVideoBg: false,
      resizeObserver: null, isDraggingProgress: false, lyricsAnimationId: null,
      
      // 本地音频支持
      isLocalAudio: false, localAudio: null, _currentAudioEl: null, _currentLocalAudioFile: null,

      apTimeHandler: null, apPlayHandler: null, apPauseHandler: null, apEndHandler: null,
  
      formatTime: function(s) {
          if (isNaN(s) || !isFinite(s)) return "00:00";
          const m = Math.floor(s / 60), sec = Math.floor(s % 60);
          return `${m < 10 ? '0' : ''}${m}:${sec < 10 ? '0' : ''}${sec}`;
      },

      // === 新增：音量控制相关方法 ===
      setVolume: function(vol) {
          if (this.isLocalAudio && this.localAudio) {
              this.localAudio.volume = vol;
          } else if (window.ap) {
              window.ap.volume && window.ap.volume(vol, true);
          }
          const vt = document.getElementById('vg-vol-text'); if (vt) vt.textContent = Math.round(vol * 100) + '%';
          this.updateVolIcon(vol);
      },

      updateVolIcon: function(vol) {
          const icon = document.getElementById('vg-vol-icon');
          if (!icon) return;
          if (vol === 0) icon.className = "fa-solid fa-volume-xmark";
          else if (vol < 0.5) icon.className = "fa-solid fa-volume-low";
          else icon.className = "fa-solid fa-volume-high";
      },

      toggleMute: function() {
          const vb = document.getElementById('vg-volume-bar');
          if (!vb) return;
          let currentVol = vb.value / 100;
          if (currentVol > 0) {
              this._lastVol = currentVol;
              vb.value = 0;
              this.setVolume(0);
          } else {
              let targetVol = this._lastVol || 0.7;
              vb.value = targetVol * 100;
              this.setVolume(targetVol);
          }
      },

      handleFileSelect: function (input) {
        if (input.files && input.files[0]) {
          const file = input.files[0], reader = new FileReader();
          reader.onload = (e) => { this.customVisual = e.target.result; this.updateVisuals(this.customVisual, file.type.startsWith("video/")); };
          reader.readAsDataURL(file);
        }
        input.value = "";
      },

      handleAudioSelect: function (input) {
        if (input.files && input.files[0]) {
            const file = input.files[0];
            this._currentLocalAudioFile = file; // 保存文件以便导出时传给服务器
            if (!this.localAudio) {
                this.localAudio = document.createElement("audio");
                this.localAudio.crossOrigin = "anonymous";
            }
            this.localAudio.src = URL.createObjectURL(file);
            this.isLocalAudio = true;
            
            const fileName = file.name.replace(/\.[^/.]+$/, "");
            document.getElementById("vg-title").textContent = fileName;
            this.data.name = fileName; 

            // [新增]：更换本地歌曲时，自动将作者置空
            document.getElementById("vg-artist").textContent = "";
            this.data.artist = "";

            if (window.ap && !window.ap.audio.paused) window.ap.pause();
            
            this.attachEvents(this.localAudio);
            this.localAudio.play();
        }
        input.value = "";
      },

      handleLyricSelect: function(input) {
        if (input.files && input.files[0]) {
            const reader = new FileReader();
            reader.onload = (e) => { this.parseAndSetLyrics(e.target.result); };
            reader.readAsText(input.files[0]);
        }
        input.value = "";
      },

      updateVisuals: function (src, isVideo) {
        this.isVideoBg = isVideo;
        const bgImg = document.getElementById("vg-bg-img"), bgVid = document.getElementById("vg-bg-video");
        const cvImg = document.getElementById("vg-cover-img"), cvVid = document.getElementById("vg-cover-video");
        bgImg.style.display = "none"; bgVid.style.display = "none"; cvImg.style.display = "none"; cvVid.style.display = "none";
        bgVid.pause(); cvVid.pause();
        if (isVideo) {
          bgVid.src = src; bgVid.style.display = "block"; bgVid.play().catch(() => {});
          cvVid.src = src; cvVid.style.display = "block"; cvVid.play().catch(() => {});
        } else {
          bgImg.src = src; bgImg.style.display = "block"; cvImg.src = src; cvImg.style.display = "block";
        }
      },

      attachEvents: function(audioEl) {
        if (!audioEl) return;
        this.detachEvents(this._currentAudioEl);
        this._currentAudioEl = audioEl;

        if (!this.apTimeHandler) {
            this.apTimeHandler = () => this.syncLyrics();
            this.apPlayHandler = () => { 
                if (!this.isLocalAudio && window.currentPlayingId !== this.data.id) return;
                this.isPlaying = true; this.updatePlayUI(); this.initAudioContext(); this.startRealtimeVisualizer(); this.startLyricsLoop();
                const b = document.getElementById("vg-bg-video"), c = document.getElementById("vg-cover-video");
                if(b?.style.display !== 'none') b.play().catch(()=>{}); if(c?.style.display !== 'none') c.play().catch(()=>{});
                document.getElementById("vg-bg-img")?.classList.add("playing");
            };
            this.apPauseHandler = () => { 
                this.isPlaying = false; this.updatePlayUI(); this.stopRealtimeVisualizer(); this.stopLyricsLoop(); this.syncLyrics();
                const b = document.getElementById("vg-bg-video"), c = document.getElementById("vg-cover-video");
                if(b?.style.display !== 'none') b.pause(); if(c?.style.display !== 'none') c.pause();
                document.getElementById("vg-bg-img")?.classList.remove("playing");
            };
            this.apEndHandler = () => { this.isPlaying = false; this.updatePlayUI(); this.stopRealtimeVisualizer(); this.stopLyricsLoop(); this.syncLyrics(); };
        }

        audioEl.addEventListener('timeupdate', this.apTimeHandler);
        audioEl.addEventListener('play', this.apPlayHandler);
        audioEl.addEventListener('pause', this.apPauseHandler);
        audioEl.addEventListener('ended', this.apEndHandler);
      },

      detachEvents: function(audioEl) {
          if (!audioEl) return;
          audioEl.removeEventListener('timeupdate', this.apTimeHandler);
          audioEl.removeEventListener('play', this.apPlayHandler);
          audioEl.removeEventListener('pause', this.apPauseHandler);
          audioEl.removeEventListener('ended', this.apEndHandler);
      },

      open: async function (songData) {
        this.data = songData; this.customVisual = null;
        this.isLocalAudio = false; this._currentLocalAudioFile = null;
        if(this.localAudio) { this.localAudio.pause(); this.localAudio.removeAttribute('src'); this.localAudio.load(); }

        const modal = document.getElementById("vg-modal");
        this.updateVisuals(songData.cover || "https://via.placeholder.com/600", false);
        document.getElementById("vg-title").textContent = songData.name;
        document.getElementById("vg-artist").textContent = songData.artist;

        if (window.ap && window.ap.audio) {
            this.isPlaying = (window.currentPlayingId === songData.id) && !window.ap.audio.paused;
            this.updatePlayUI();
            this.attachEvents(window.ap.audio);
        }
        
        const sb = document.getElementById('vg-seek-bar');
        sb.oninput = (e) => {
            this.isDraggingProgress = true;
            let audioEl = this.isLocalAudio ? this.localAudio : (window.ap && window.ap.audio);
            if (audioEl && audioEl.duration > 0) {
                document.getElementById('vg-time-current').textContent = this.formatTime((e.target.value / 100) * audioEl.duration);
            }
        };
        sb.onchange = (e) => {
            this.isDraggingProgress = false;
            let targetPercent = e.target.value / 100;

            if (this.isLocalAudio) {
                if (this.localAudio.duration) this.localAudio.currentTime = targetPercent * this.localAudio.duration;
                this.syncLyrics();
                if (this.localAudio.paused) this.localAudio.play();
            } else {
                if (window.currentPlayingId === this.data.id) {
                    if (window.ap && window.ap.audio && window.ap.audio.duration) {
                        window.ap.seek(targetPercent * window.ap.audio.duration);
                        this.syncLyrics();
                        if (window.ap.audio.paused) window.ap.play();
                    }
                } else {
                    if (typeof window.playAllAndJumpToId === 'function') {
                        window.playAllAndJumpToId(this.data.id);
                        const onCanPlay = () => {
                            if (window.ap && window.ap.audio && window.ap.audio.duration) window.ap.seek(targetPercent * window.ap.audio.duration);
                            window.ap.audio.removeEventListener('canplay', onCanPlay);
                        };
                        window.ap.audio.addEventListener('canplay', onCanPlay);
                    }
                }
            }
        };

        // 👇 新增：初始化并绑定音量条事件
        const vb = document.getElementById('vg-volume-bar');
        if (vb) {
            let initialVol = 0.7;
            if (this.isLocalAudio && this.localAudio) {
                initialVol = this.localAudio.volume;
            } else if (window.ap && window.ap.audio) {
                initialVol = window.ap.audio.volume || initialVol;
            }
            vb.value = initialVol * 100;
            const vt = document.getElementById('vg-vol-text'); if (vt) vt.textContent = Math.round(initialVol * 100) + '%';
            this.updateVolIcon(initialVol);
            vb.oninput = (e) => { this.setVolume(e.target.value / 100); };
        }

        this.reset(); 
        
        fetch(buildVideoGenLyricURL(songData))
            .then(r => r.text())
            .then(text => this.parseAndSetLyrics(text));

        modal.style.display = "flex";
        requestAnimationFrame(() => { 
            modal.classList.add("active"); this.initRealtimeCanvas(); 
            if (this.isPlaying) this.apPlayHandler(); 
        });
      },

      close: function () {
        this.stopRealtimeVisualizer();
        this.stopLyricsLoop();
        this.detachEvents(this._currentAudioEl);
        if (this.isLocalAudio && this.localAudio) this.localAudio.pause();
        if (this.resizeObserver) { this.resizeObserver.disconnect(); this.resizeObserver = null; }
        const m = document.getElementById("vg-modal"); m.classList.remove("active");
        setTimeout(() => { m.style.display = "none"; this.reset(); }, 500);
      },

      reset: function () {
        this.stopLyricsLoop();
        document.getElementById("vg-status-loading").style.display = "none";
        document.getElementById("vg-status-success").style.display = "none";
        document.getElementById("vg-ui-container").classList.remove("vg-rendering-hide");
      },

      parseAndSetLyrics: function (text) {
        const box = document.getElementById("vg-lyrics"); box.innerHTML = "";
        this.lyricTimes = []; this.lyricRaw = []; this.lyricGroups = []; this.lyricMode = 'line'; this.lastActiveIndex = -1;

        const parsed = parseLyrics(text);
        this.lyricGroups = parsed.groups;
        this.lyricMode = parsed.type;

        parsed.groups.forEach((group, index) => {
            const { orig, roma, trans } = splitLyricGroupLines(group.lines);
            if (!orig) return;

            const t = group.time;
            this.lyricTimes.push({ time: t, content: orig.text });
            this.lyricRaw.push({ time: t, text: orig.text });

            const d = document.createElement("div");
            d.className = parsed.type === 'karaoke' ? "vg-line vg-line-karaoke" : "vg-line";
            d.dataset.index = String(index);

            if (parsed.type === 'karaoke') {
                d.innerHTML = [
                    renderKaraokeLineHTML(orig, 'vg-line-orig', group, true),
                    renderKaraokeLineHTML(roma, 'vg-line-roma', group, !!roma?.verbatim),
                    renderKaraokeLineHTML(trans, 'vg-line-trans', group, !!trans?.verbatim)
                ].join('');
            } else {
                d.textContent = orig.text;
            }

            d.onclick = () => { 
                if (this.isLocalAudio) {
                    this.localAudio.currentTime = t;
                    if (this.localAudio.paused) this.localAudio.play();
                } else {
                    if (window.currentPlayingId === this.data.id) {
                        if (window.ap && window.ap.audio) { window.ap.seek(t); if (window.ap.audio.paused) window.ap.play(); }
                    } else {
                        if (typeof window.playAllAndJumpToId === 'function') {
                            window.playAllAndJumpToId(this.data.id);
                            const onCanPlay = () => {
                                if (window.ap && window.ap.audio) window.ap.seek(t);
                                window.ap.audio.removeEventListener('canplay', onCanPlay);
                            };
                            window.ap.audio.addEventListener('canplay', onCanPlay);
                        }
                    }
                }
            };
            box.appendChild(d);
        });
        if (this.lyricTimes.length === 0) box.innerHTML = '<p style="padding-top:100px; color:rgba(255,255,255,0.5);">纯音乐 / 无歌词</p>';
      },
  
      initRealtimeCanvas: function() {
          const c = document.getElementById('vg-visualizer'); c.innerHTML = '<canvas id="vg-rt-canvas"></canvas>';
          this.rtCanvas = document.getElementById('vg-rt-canvas'); this.rtCtx = this.rtCanvas.getContext('2d');
          const rz = () => { this.rtCanvas.width = c.offsetWidth; this.rtCanvas.height = c.offsetHeight; }; rz();
          if(!this.resizeObserver) { this.resizeObserver = new ResizeObserver(rz); this.resizeObserver.observe(c); }
      },

      initAudioContext: function() {
          if (!this.audioCtx) this.audioCtx = new (window.AudioContext || window.webkitAudioContext)();
          if (this.audioCtx.state === 'suspended') this.audioCtx.resume();
          if (!this.analyser) { 
              this.analyser = this.audioCtx.createAnalyser(); 
              this.analyser.fftSize = 2048; 
              this.analyser.smoothingTimeConstant = 0.65; 
          }
      },
      connectAudioSource: function() {
          if (!this.analyser) return; 
          try {
              if (this.isLocalAudio) {
                  if (!this.localSourceNode && this.localAudio) {
                      this.localSourceNode = this.audioCtx.createMediaElementSource(this.localAudio);
                  }
                  if (this.localSourceNode) this.localSourceNode.connect(this.analyser);
              } else {
                  if (!window.ap || !window.ap.audio) return;
                  if (!this.sourceNode) {
                      this.sourceNode = this.audioCtx.createMediaElementSource(window.ap.audio);
                  }
                  if (this.sourceNode) this.sourceNode.connect(this.analyser);
              }
              this.analyser.connect(this.audioCtx.destination);
          } catch(e) { console.warn("Audio Routing Warning:", e); }
      },
      startRealtimeVisualizer: function() {
          this.initAudioContext(); this.connectAudioSource();
          const dataArray = new Uint8Array(this.analyser.frequencyBinCount);
          const animate = () => {
              if (!this.isPlaying) return; this.animationId = requestAnimationFrame(animate);
              this.analyser.getByteFrequencyData(dataArray);
              const visResult = processVisualizerBars(dataArray);
              this.rtCtx.clearRect(0, 0, this.rtCanvas.width, this.rtCanvas.height);
              const dw = document.querySelector('.vg-disc-wrap');
              const radius = dw ? (dw.offsetWidth / 2 + 2) : (this.rtCanvas.width / 2 * 0.25);
              drawVisualizerRings(this.rtCtx, this.rtCanvas.width/2, this.rtCanvas.height/2, radius, visResult.heights);
          };
          animate();
      },
      stopRealtimeVisualizer: function() { if (this.animationId) cancelAnimationFrame(this.animationId); },
      startLyricsLoop: function() {
          if (this.lyricsAnimationId) return;
          const tick = () => {
              this.lyricsAnimationId = null;
              this.syncLyrics();
              if (!this.isPlaying) return;
              this.lyricsAnimationId = requestAnimationFrame(tick);
          };
          this.lyricsAnimationId = requestAnimationFrame(tick);
      },
      stopLyricsLoop: function() {
          if (this.lyricsAnimationId) {
              cancelAnimationFrame(this.lyricsAnimationId);
              this.lyricsAnimationId = null;
          }
      },
  
      startRendering: function () {
        if (!this.data) return;
        
        // 【关键改动】附加上你本地选择的文件 Blob，给到新开的渲染页面
        window.__vgRenderData = {
            id: this.data.id,
            source: this.data.source,
            name: this.data.name,
            artist: this.data.artist,
            rawCover: this.customVisual || this.data.cover || "https://via.placeholder.com/600",
            isVideoBg: this.isVideoBg,
            lyricRaw: this.lyricRaw,
            lyricGroups: this.lyricGroups,
            lyricMode: this.lyricMode,
            customAudioFile: this.isLocalAudio ? this._currentLocalAudioFile : null,
            apiRoot: window.API_ROOT
        };

        const renderWin = window.open(window.API_ROOT + '/render', '_blank');
        if (!renderWin) {
            alert("渲染页面被浏览器拦截，请允许弹出窗口！"); return;
        }
      },
  
            // === 新增：上一首 / 下一首 ===
            playPrev: function() {
                    if (this.isLocalAudio) return alert("本地预览模式下无法切换歌曲");
                    if (window.ap && window.ap.list && window.ap.list.audios && window.ap.list.audios.length > 0) {
                            window.ap.skipBack && window.ap.skipBack();
                            if (window.ap.audio && window.ap.audio.paused) window.ap.play();
                    }
            },
            playNext: function() {
                    if (this.isLocalAudio) return alert("本地预览模式下无法切换歌曲");
                    if (window.ap && window.ap.list && window.ap.list.audios && window.ap.list.audios.length > 0) {
                            window.ap.skipForward && window.ap.skipForward();
                            if (window.ap.audio && window.ap.audio.paused) window.ap.play();
                    }
            },

            togglePlay: function () {
                if (this.isLocalAudio && this.localAudio) {
                        this.localAudio.paused ? this.localAudio.play() : this.localAudio.pause();
                } else {
                        if (!this.data || !window.ap) return;
                        if (window.currentPlayingId === this.data.id) window.ap.toggle();
                        else if (typeof window.playAllAndJumpToId === 'function') window.playAllAndJumpToId(this.data.id);
                }
            },

            updatePlayUI: function () {
                const i = document.getElementById("vg-play-toggle-icon"), m = document.getElementById("vg-modal");
                if (i) i.className = this.isPlaying ? "fa-solid fa-pause" : "fa-solid fa-play";
                if (m) this.isPlaying ? m.classList.add("playing") : m.classList.remove("playing");
            },

      syncLyrics: function () {
        const audioEl = this.isLocalAudio ? this.localAudio : (window.ap && window.ap.audio);
        if (!audioEl) return;
        if (!this.isLocalAudio && window.currentPlayingId !== this.data.id) return;
        
        const ct = audioEl.currentTime, dur = audioEl.duration || 0;
        
        if (!this.isDraggingProgress && dur > 0) { 
            const sb = document.getElementById('vg-seek-bar'); 
            sb.value = (ct/dur)*100; 
            document.getElementById('vg-time-current').textContent = this.formatTime(ct); 
            document.getElementById('vg-time-total').textContent = this.formatTime(dur); 
        }
        
        if (this.lyricTimes.length === 0 || this.isUserScrolling) return;
        let active = -1; for (let i = 0; i < this.lyricTimes.length; i++) { if (ct >= this.lyricTimes[i].time) active = i; else break; }
        if (active === -1) return;

        const ls = document.querySelectorAll(".vg-line");
        if (active !== this.lastActiveIndex) {
          ls.forEach(l => {
            l.classList.remove("active");
          });
          const al = ls[active]; if (al) { al.classList.add("active"); if (!this.isUserScrolling) al.scrollIntoView({ behavior: "smooth", block: "center" }); }
          this.lastActiveIndex = active;
        }

        if (this.lyricMode === 'karaoke') {
          const ms = ct * 1000;
          document.querySelectorAll(".vg-word").forEach(word => {
            const start = Number(word.dataset.start || 0);
            const end = Number(word.dataset.end || start + fallbackLineDuration);
            const progress = lyricProgress(ms, start, end);
            // 核心修复：100% 进度时直接加上安全距离，防止被裁切掉右侧描边
            word.style.setProperty("--karaoke-progress", progress === 1 ? "calc(100% + 8px)" : `${(progress * 100).toFixed(3)}%`);
            word.classList.toggle("is-active", progress > 0 && progress < 1);
          });
        }
      },
    };
})();
