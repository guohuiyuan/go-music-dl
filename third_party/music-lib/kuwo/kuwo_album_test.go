package kuwo

import (
	"strings"
	"testing"
)

func TestKuwoAlbumDetailURLKeepsLegacyParameterOrder(t *testing.T) {
	got := kuwoAlbumDetailURL("87758985", 0, 100)
	wantPrefix := "http://search.kuwo.cn/r.s?pn=0&rn=100&stype=albuminfo&albumid=87758985"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("url = %q, want prefix %q", got, wantPrefix)
	}
	if strings.Contains(got, "?albumid=") {
		t.Fatalf("url uses sorted query order that Kuwo legacy album API does not accept: %q", got)
	}
}
