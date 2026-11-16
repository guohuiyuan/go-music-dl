package core

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"

	"github.com/guohuiyuan/music-lib/apple"
	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
	"github.com/guohuiyuan/music-lib/spotify"
)

type audioQualityCacheEntry struct {
	options []model.AudioQualityOption
}

type audioQualityInflight struct {
	done    chan struct{}
	options []model.AudioQualityOption
	err     error
}

var audioQualityOptionsCache = struct {
	sync.Mutex
	entries  map[string]audioQualityCacheEntry
	inflight map[string]*audioQualityInflight
}{
	entries:  make(map[string]audioQualityCacheEntry),
	inflight: make(map[string]*audioQualityInflight),
}

type accountAudioQualityProvider interface {
	AvailableQualityOptions() ([]model.AudioQualityOption, error)
	GetDownloadURLWithQuality(s *model.Song, quality string) (string, error)
}

type audioQualitySourceDefinition struct {
	choices  func() []model.AudioQualityOption
	provider func(cookie string) accountAudioQualityProvider
}

var audioQualitySourceNames = []string{
	"qq",
	"netease",
	"kugou",
	"kuwo",
	"migu",
	"soda",
	"qianqian",
	"apple",
	"spotify",
	"fivesing",
	"jamendo",
	"joox",
	"bilibili",
}

var audioQualitySources = map[string]audioQualitySourceDefinition{
	"qq": {
		choices:  qq.ListQualityOptions,
		provider: func(cookie string) accountAudioQualityProvider { return qq.New(cookie) },
	},
	"netease": {
		choices:  netease.ListQualityOptions,
		provider: func(cookie string) accountAudioQualityProvider { return netease.New(cookie) },
	},
	"kugou": {
		choices:  kugou.ListQualityOptions,
		provider: func(cookie string) accountAudioQualityProvider { return kugou.New(cookie) },
	},
	"kuwo": {
		choices:  kuwo.ListQualityOptions,
		provider: func(cookie string) accountAudioQualityProvider { return kuwo.New(cookie) },
	},
	"migu": {
		choices:  migu.ListQualityOptions,
		provider: func(cookie string) accountAudioQualityProvider { return migu.New(cookie) },
	},
	"soda": {
		choices:  soda.ListQualityOptions,
		provider: func(cookie string) accountAudioQualityProvider { return NewSodaClient(cookie) },
	},
	"qianqian": {
		choices:  qianqian.ListQualityOptions,
		provider: func(cookie string) accountAudioQualityProvider { return qianqian.New(cookie) },
	},
	"apple": {
		choices:  apple.ListQualityOptions,
		provider: func(cookie string) accountAudioQualityProvider { return apple.New(cookie) },
	},
	"spotify": {
		choices:  spotify.ListQualityOptions,
		provider: func(cookie string) accountAudioQualityProvider { return spotify.New(cookie) },
	},
	"fivesing": {
		choices:  fivesing.ListQualityOptions,
		provider: func(cookie string) accountAudioQualityProvider { return fivesing.New(cookie) },
	},
	"jamendo": {
		choices:  jamendo.ListQualityOptions,
		provider: func(cookie string) accountAudioQualityProvider { return jamendo.New(cookie) },
	},
	"joox": {
		choices:  joox.ListQualityOptions,
		provider: func(cookie string) accountAudioQualityProvider { return joox.New(cookie) },
	},
	"bilibili": {
		choices:  bilibili.ListQualityOptions,
		provider: func(cookie string) accountAudioQualityProvider { return bilibili.New(cookie) },
	},
}

func audioQualitySourceFor(source string) (audioQualitySourceDefinition, string, bool) {
	source = strings.ToLower(strings.TrimSpace(source))
	definition, ok := audioQualitySources[source]
	return definition, source, ok
}

func audioQualityChoicesForSource(source string) ([]model.AudioQualityOption, bool) {
	definition, _, ok := audioQualitySourceFor(source)
	if !ok || definition.choices == nil {
		return nil, false
	}
	return append([]model.AudioQualityOption(nil), definition.choices()...), true
}

// GetAllAudioQualityChoices returns the complete quality taxonomy exposed by
// every supported source. These choices are independent of the current cookie;
// GetAudioQualityOptions reports which of them the cookie can actually use.
func GetAllAudioQualityChoices() map[string][]model.AudioQualityOption {
	choices := make(map[string][]model.AudioQualityOption, len(audioQualitySourceNames))
	for _, source := range audioQualitySourceNames {
		if options, ok := audioQualityChoicesForSource(source); ok {
			choices[source] = options
		}
	}
	return choices
}

func audioQualityProviderForSource(source, cookie string) (accountAudioQualityProvider, bool) {
	definition, _, ok := audioQualitySourceFor(source)
	if !ok || definition.provider == nil {
		return nil, false
	}
	return definition.provider(cookie), true
}

func resolveSongAudioQuality(song *model.Song, requestedQuality, configuredQuality string) string {
	if song != nil && song.Extra != nil {
		if override := strings.TrimSpace(song.Extra["quality"]); override != "" {
			return override
		}
	}
	if requested := strings.TrimSpace(requestedQuality); requested != "" {
		return requested
	}
	return strings.TrimSpace(configuredQuality)
}

// ResolveSongAudioQuality applies the request, per-song override, and
// source-specific Web setting in a stable precedence order.
func ResolveSongAudioQuality(song *model.Song, requestedQuality string) string {
	source := ""
	if song != nil {
		source = song.Source
	}
	return resolveSongAudioQuality(
		song,
		requestedQuality,
		AudioQualityDefaultForSource(source),
	)
}

// GetAudioQualityOptions returns the selectable qualities for a source using
// the currently configured cookie. It probes the provider once per cookie and
// reuses that measured range until the cookie changes.
func GetAudioQualityOptions(source string) ([]model.AudioQualityOption, error) {
	source = strings.ToLower(strings.TrimSpace(source))
	cookie := strings.TrimSpace(CM.Get(source))
	provider, ok := audioQualityProviderForSource(source, cookie)
	if !ok {
		return nil, nil
	}
	return getAudioQualityOptionsWithProvider(source, cookie, provider)
}

func getAudioQualityOptionsWithProvider(
	source string,
	cookie string,
	provider accountAudioQualityProvider,
) ([]model.AudioQualityOption, error) {
	cacheKey := audioQualityCacheKey(source, cookie)

	audioQualityOptionsCache.Lock()
	if cached, exists := audioQualityOptionsCache.entries[cacheKey]; exists {
		audioQualityOptionsCache.Unlock()
		return append([]model.AudioQualityOption(nil), cached.options...), nil
	}
	if pending := audioQualityOptionsCache.inflight[cacheKey]; pending != nil {
		audioQualityOptionsCache.Unlock()
		<-pending.done
		return append([]model.AudioQualityOption(nil), pending.options...), pending.err
	}
	pending := &audioQualityInflight{done: make(chan struct{})}
	audioQualityOptionsCache.inflight[cacheKey] = pending
	audioQualityOptionsCache.Unlock()

	options, err := provider.AvailableQualityOptions()

	audioQualityOptionsCache.Lock()
	if err == nil {
		audioQualityOptionsCache.entries[cacheKey] = audioQualityCacheEntry{
			options: append([]model.AudioQualityOption(nil), options...),
		}
	}
	pending.options = append([]model.AudioQualityOption(nil), options...)
	pending.err = err
	delete(audioQualityOptionsCache.inflight, cacheKey)
	close(pending.done)
	audioQualityOptionsCache.Unlock()
	return append([]model.AudioQualityOption(nil), options...), err
}

func audioQualityCacheKey(source, cookie string) string {
	sum := sha256.Sum256([]byte(cookie))
	return source + "\x00" + fmt.Sprintf("%x", sum)
}

// GetAllAudioQualityOptions probes supported sources in parallel and returns
// the source-specific options and non-fatal warnings.
func GetAllAudioQualityOptions() (map[string][]model.AudioQualityOption, map[string]string) {
	optionsBySource := make(map[string][]model.AudioQualityOption, len(audioQualitySourceNames))
	warnings := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, source := range audioQualitySourceNames {
		source := source
		wg.Add(1)
		go func() {
			defer wg.Done()
			options, err := GetAudioQualityOptions(source)
			mu.Lock()
			defer mu.Unlock()
			optionsBySource[source] = options
			if err != nil {
				warnings[source] = err.Error()
			}
		}()
	}
	wg.Wait()
	return optionsBySource, warnings
}

// ResolveAvailableSongAudioQuality returns the requested tier when it is
// usable, otherwise the best advertised tier for that source.
func ResolveAvailableSongAudioQuality(song *model.Song, requestedQuality string) (string, error) {
	quality := ResolveSongAudioQuality(song, requestedQuality)
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return "", fmt.Errorf("unsupported audio quality: %s", quality)
	}
	if normalized == model.AudioQualityAuto || song == nil {
		return string(normalized), nil
	}

	options, err := GetAudioQualityOptions(song.Source)
	if err != nil {
		return "", err
	}
	for _, option := range options {
		if option.Value == normalized {
			return string(normalized), nil
		}
	}
	if len(options) == 0 {
		return string(model.AudioQualityAuto), nil
	}
	return string(options[0].Value), nil
}

func audioQualityFallbackOrder(song *model.Song, requestedQuality string) ([]model.AudioQuality, error) {
	quality := ResolveSongAudioQuality(song, requestedQuality)
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return nil, fmt.Errorf("unsupported audio quality: %s", quality)
	}
	if normalized == model.AudioQualityAuto {
		return []model.AudioQuality{model.AudioQualityAuto}, nil
	}
	if song == nil {
		return nil, fmt.Errorf("song is nil")
	}

	choices, ok := audioQualityChoicesForSource(song.Source)
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("audio quality choices are unavailable for source %s", song.Source)
	}
	for index, option := range choices {
		if option.Value != normalized {
			continue
		}
		order := make([]model.AudioQuality, 0, len(choices)-index)
		for _, fallback := range choices[index:] {
			order = append(order, fallback.Value)
		}
		return order, nil
	}
	return nil, fmt.Errorf(
		"audio quality %s is not supported by source %s",
		normalized,
		song.Source,
	)
}

func resolveWithAudioQualityFallback[T any](
	song *model.Song,
	requestedQuality string,
	resolve func(string) (T, error),
	valid func(T) bool,
) (T, error) {
	var zero T
	order, err := audioQualityFallbackOrder(song, requestedQuality)
	if err != nil {
		return zero, err
	}

	var lastErr error
	for _, quality := range order {
		value, resolveErr := resolve(string(quality))
		if resolveErr == nil && valid(value) {
			return value, nil
		}
		if resolveErr != nil {
			lastErr = resolveErr
		} else {
			lastErr = fmt.Errorf("source returned an empty result for quality %s", quality)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no playable quality was found")
	}
	return zero, fmt.Errorf(
		"audio unavailable from quality %s downward: %w",
		order[0],
		lastErr,
	)
}

func downloadURLWithQualityFallback(
	provider accountAudioQualityProvider,
	song *model.Song,
	requestedQuality string,
) (string, error) {
	return resolveWithAudioQualityFallback(
		song,
		requestedQuality,
		func(quality string) (string, error) {
			return provider.GetDownloadURLWithQuality(song, quality)
		},
		func(downloadURL string) bool {
			return strings.TrimSpace(downloadURL) != ""
		},
	)
}
