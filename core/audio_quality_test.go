package core

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

func TestResolveSongAudioQualityPrecedence(t *testing.T) {
	tests := []struct {
		name       string
		song       *model.Song
		requested  string
		configured string
		want       string
	}{
		{
			name:       "per song override wins",
			song:       &model.Song{Extra: map[string]string{"quality": "master"}},
			requested:  "lossless",
			configured: "high",
			want:       "master",
		},
		{
			name:       "explicit request wins over default",
			song:       &model.Song{},
			requested:  "atmos",
			configured: "high",
			want:       "atmos",
		},
		{
			name:       "configured default is used",
			song:       &model.Song{},
			configured: "lossless",
			want:       "lossless",
		},
		{
			name:       "missing song is safe",
			configured: "high",
			want:       "high",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveSongAudioQuality(test.song, test.requested, test.configured); got != test.want {
				t.Fatalf("quality = %q, want %q", got, test.want)
			}
		})
	}
}

func TestGetAudioQualityOptionsIgnoresUnsupportedSources(t *testing.T) {
	options, err := GetAudioQualityOptions("not-a-source")
	if err != nil {
		t.Fatalf("unsupported source returned error: %v", err)
	}
	if len(options) != 0 {
		t.Fatalf("unsupported source options = %#v, want none", options)
	}
}

func TestGetAllAudioQualityChoicesCoversEverySupportedSource(t *testing.T) {
	choices := GetAllAudioQualityChoices()
	if len(choices) != len(audioQualitySourceNames) {
		t.Fatalf("choice source count = %d, want %d", len(choices), len(audioQualitySourceNames))
	}
	for _, source := range audioQualitySourceNames {
		if len(choices[source]) == 0 {
			t.Fatalf("source %q has no quality choices", source)
		}
	}
	if !containsAudioQuality(choices["qq"], model.AudioQualityMaster) {
		t.Fatalf("QQ choices = %#v, want master", choices["qq"])
	}
	if !containsAudioQuality(choices["apple"], model.AudioQualityPreview) {
		t.Fatalf("Apple choices = %#v, want preview", choices["apple"])
	}
	if !containsAudioQuality(choices["spotify"], model.AudioQualityPreview) {
		t.Fatalf("Spotify choices = %#v, want preview", choices["spotify"])
	}
}

func TestAudioQualityRegistryHasProviderForEverySource(t *testing.T) {
	for _, source := range audioQualitySourceNames {
		definition, ok := audioQualitySources[source]
		if !ok {
			t.Fatalf("source %q is missing from the quality registry", source)
		}
		if definition.choices == nil {
			t.Fatalf("source %q has no quality choices", source)
		}
		if definition.provider == nil {
			t.Fatalf("source %q has no quality provider", source)
		}
	}
}

func containsAudioQuality(options []model.AudioQualityOption, quality model.AudioQuality) bool {
	for _, option := range options {
		if option.Value == quality {
			return true
		}
	}
	return false
}

type fallingBackAudioQualityProvider struct {
	calls     []string
	available map[string]string
}

func (p *fallingBackAudioQualityProvider) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	return nil, nil
}

func (p *fallingBackAudioQualityProvider) GetDownloadURLWithQuality(
	_ *model.Song,
	quality string,
) (string, error) {
	p.calls = append(p.calls, quality)
	if downloadURL := p.available[quality]; downloadURL != "" {
		return downloadURL, nil
	}
	return "", errors.New("quality unavailable")
}

func TestDownloadURLWithQualityFallbackDescendsInSourceOrder(t *testing.T) {
	provider := &fallingBackAudioQualityProvider{
		available: map[string]string{
			string(model.AudioQualityLossless): "https://example.com/lossless.flac",
		},
	}
	url, err := downloadURLWithQualityFallback(
		provider,
		&model.Song{ID: "song", Source: "qq"},
		string(model.AudioQualityMaster),
	)
	if err != nil {
		t.Fatalf("fallback download: %v", err)
	}
	if url != "https://example.com/lossless.flac" {
		t.Fatalf("download URL = %q", url)
	}
	wantCalls := []string{
		string(model.AudioQualityMaster),
		string(model.AudioQualityAtmos51),
		string(model.AudioQualityAtmos),
		string(model.AudioQualityLossless),
	}
	if len(provider.calls) != len(wantCalls) {
		t.Fatalf("quality calls = %#v, want %#v", provider.calls, wantCalls)
	}
	for index := range wantCalls {
		if provider.calls[index] != wantCalls[index] {
			t.Fatalf("quality calls = %#v, want %#v", provider.calls, wantCalls)
		}
	}
}

func TestDownloadURLWithQualityFallbackNeverUpgrades(t *testing.T) {
	provider := &fallingBackAudioQualityProvider{
		available: map[string]string{
			string(model.AudioQualityMaster): "https://example.com/master.flac",
			string(model.AudioQualityHigh):   "https://example.com/high.mp3",
		},
	}
	url, err := downloadURLWithQualityFallback(
		provider,
		&model.Song{ID: "song", Source: "qq"},
		string(model.AudioQualityHigh),
	)
	if err != nil {
		t.Fatalf("fallback download: %v", err)
	}
	if url != "https://example.com/high.mp3" {
		t.Fatalf("download URL = %q", url)
	}
	if len(provider.calls) != 1 || provider.calls[0] != string(model.AudioQualityHigh) {
		t.Fatalf("quality calls = %#v, want only high", provider.calls)
	}
}

type blockingAudioQualityProvider struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	options []model.AudioQualityOption
}

func (p *blockingAudioQualityProvider) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	p.calls.Add(1)
	select {
	case p.started <- struct{}{}:
	default:
	}
	<-p.release
	return append([]model.AudioQualityOption(nil), p.options...), nil
}

func (p *blockingAudioQualityProvider) GetDownloadURLWithQuality(
	_ *model.Song,
	_ string,
) (string, error) {
	return "", nil
}

func TestGetAudioQualityOptionsCoalescesConcurrentProbes(t *testing.T) {
	const source = "audio-quality-coalesce-test"
	const cookie = "test-cookie"
	t.Cleanup(func() {
		audioQualityOptionsCache.Lock()
		defer audioQualityOptionsCache.Unlock()
		cacheKey := audioQualityCacheKey(source, cookie)
		delete(audioQualityOptionsCache.entries, cacheKey)
		delete(audioQualityOptionsCache.inflight, cacheKey)
	})

	provider := &blockingAudioQualityProvider{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
		options: []model.AudioQualityOption{{
			Value: model.AudioQualityHigh,
			Label: "High",
		}},
	}
	type result struct {
		options []model.AudioQualityOption
		err     error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		options, err := getAudioQualityOptionsWithProvider(source, cookie, provider)
		results <- result{options: options, err: err}
	}()
	<-provider.started
	go func() {
		defer wg.Done()
		options, err := getAudioQualityOptionsWithProvider(source, cookie, provider)
		results <- result{options: options, err: err}
	}()

	time.Sleep(50 * time.Millisecond)
	close(provider.release)
	wg.Wait()
	close(results)

	if got := provider.calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
	for result := range results {
		if result.err != nil {
			t.Fatalf("quality probe returned error: %v", result.err)
		}
		if len(result.options) != 1 || result.options[0].Value != model.AudioQualityHigh {
			t.Fatalf("quality options = %#v, want one high option", result.options)
		}
	}

	options, err := getAudioQualityOptionsWithProvider(source, cookie, provider)
	if err != nil || len(options) != 1 {
		t.Fatalf("cached options = %#v, err = %v", options, err)
	}
	if got := provider.calls.Load(); got != 1 {
		t.Fatalf("cached provider calls = %d, want 1", got)
	}
}

type countingAudioQualityProvider struct {
	calls atomic.Int32
}

func (p *countingAudioQualityProvider) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	p.calls.Add(1)
	return []model.AudioQualityOption{{
		Value: model.AudioQualityHigh,
		Label: "High",
	}}, nil
}

func (p *countingAudioQualityProvider) GetDownloadURLWithQuality(
	_ *model.Song,
	_ string,
) (string, error) {
	return "", nil
}

func TestGetAudioQualityOptionsReusesProbeUntilCookieChanges(t *testing.T) {
	const source = "audio-quality-cookie-cache-test"
	t.Cleanup(func() {
		audioQualityOptionsCache.Lock()
		defer audioQualityOptionsCache.Unlock()
		for _, cookie := range []string{"cookie-a", "cookie-b"} {
			cacheKey := audioQualityCacheKey(source, cookie)
			delete(audioQualityOptionsCache.entries, cacheKey)
			delete(audioQualityOptionsCache.inflight, cacheKey)
		}
	})

	provider := &countingAudioQualityProvider{}
	for i := 0; i < 3; i++ {
		if _, err := getAudioQualityOptionsWithProvider(source, "cookie-a", provider); err != nil {
			t.Fatalf("first cookie probe %d: %v", i, err)
		}
	}
	if got := provider.calls.Load(); got != 1 {
		t.Fatalf("unchanged cookie provider calls = %d, want 1", got)
	}

	if _, err := getAudioQualityOptionsWithProvider(source, "cookie-b", provider); err != nil {
		t.Fatalf("changed cookie probe: %v", err)
	}
	if got := provider.calls.Load(); got != 2 {
		t.Fatalf("changed cookie provider calls = %d, want 2", got)
	}

	if _, err := getAudioQualityOptionsWithProvider(source, "cookie-a", provider); err != nil {
		t.Fatalf("restored cookie probe: %v", err)
	}
	if got := provider.calls.Load(); got != 2 {
		t.Fatalf("restored cached cookie provider calls = %d, want 2", got)
	}
}
