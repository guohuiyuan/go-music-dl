package apple

import (
	"fmt"
	"net/url"
	"testing"
)

func TestApplePlaylistCategoriesIntegration(t *testing.T) {
	client := New("storefront=cn")
	categories, err := client.GetPlaylistCategories()
	if err != nil {
		t.Fatalf("GetPlaylistCategories failed: %v", err)
	}
	t.Logf("Got %d categories", len(categories))
	for i, c := range categories {
		if i >= 5 {
			break
		}
		t.Logf("  [%d] ID=%s Name=%s", i, c.ID, c.Name)
	}
	if len(categories) == 0 {
		t.Fatal("No categories returned")
	}

	// Test GetCategoryPlaylists with first category
	first := categories[0]
	playlists, err := client.GetCategoryPlaylists(first.ID, 1, 5)
	if err != nil {
		t.Fatalf("GetCategoryPlaylists(%s / %s) failed: %v", first.ID, first.Name, err)
	}
	t.Logf("Category %q has %d playlists (page 1, limit 5)", first.Name, len(playlists))
	for i, p := range playlists {
		fmt.Printf("  [%d] %s (ID=%s)\n", i, p.Name, p.ID)
	}
	if len(playlists) == 0 {
		t.Fatal("No playlists returned")
	}

	// Test with limit=120 (what the web UI uses)
	playlists2, err := client.GetCategoryPlaylists(first.ID, 1, 120)
	if err != nil {
		t.Fatalf("GetCategoryPlaylists(%s, 1, 120) failed: %v", first.ID, err)
	}
	t.Logf("Category %q with limit=120: got %d playlists", first.Name, len(playlists2))

	// Test K-Pop (1019399551) which was reported as 404
	kpop, err := client.GetCategoryPlaylists("1019399551", 1, 120)
	if err != nil {
		t.Fatalf("GetCategoryPlaylists(K-Pop) failed: %v", err)
	}
	t.Logf("K-Pop with limit=120: got %d playlists", len(kpop))
}


func TestAppleCuratorPlaylistFields(t *testing.T) {
	client := New("storefront=cn")

	// Fetch raw response to check if trackCount is in the data
	params := url.Values{}
	params.Set("limit", "2")
	params.Set("offset", "0")
	params.Set("l", "zh-Hans-CN")
	body, err := client.ampGet("/v1/catalog/cn/apple-curators/1019399551/playlists", params)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	// Print first 800 chars to see what fields are available
	if len(body) > 800 {
		t.Logf("Raw: %s", string(body[:800]))
	} else {
		t.Logf("Raw: %s", string(body))
	}

	playlists, err := client.GetCategoryPlaylists("1019399551", 1, 3)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	for i, p := range playlists {
		t.Logf("[%d] Name=%q TrackCount=%d Creator=%q", i, p.Name, p.TrackCount, p.Creator)
	}
}
