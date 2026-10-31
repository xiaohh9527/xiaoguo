package hongguo_test

import (
	"context"
	"testing"
	"time"

	"xiaoguo/server/internal/hongguo"
)

func TestSearchTargetTitle(t *testing.T) {
	client := hongguo.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	keyword := "秃头老汉和驴的十万年"
	res, err := client.Search(ctx, keyword)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	t.Logf("Search result for '%s': Total=%d, DramasCount=%d", keyword, res.Total, len(res.Dramas))
	if len(res.Dramas) == 0 {
		t.Fatalf("expected search results, got 0")
	}

	found := false
	for i, d := range res.Dramas {
		t.Logf("[%d] ID=%s Title=%s Name=%s Category=%s Episodes=%d Cover=%s",
			i, d.ID, d.Title, d.Name, d.CategoryName, d.TotalEpisodes, d.Cover)
		if d.Title == "秃头老汉和驴的十万年" {
			found = true
			if i != 0 {
				t.Errorf("expected target title to be ranked first, but got index %d", i)
			}
		}
	}

	if !found {
		t.Fatalf("target drama '秃头老汉和驴的十万年' not found in search results")
	}

	// Also verify detail fetch for target drama
	targetID := res.Dramas[0].SourceID
	drama, chapters, err := client.FetchDetail(ctx, targetID)
	if err != nil {
		t.Fatalf("FetchDetail failed for %s: %v", targetID, err)
	}
	t.Logf("Detail for %s: Title=%s, ChaptersCount=%d", targetID, drama.Title, len(chapters))
	if len(chapters) == 0 {
		t.Fatalf("expected chapters, got 0")
	}

	// Verify playback resolve for episode 1
	playInfo, err := client.ResolvePlayback(ctx, targetID, chapters[0].VideoID)
	if err != nil {
		t.Fatalf("ResolvePlayback failed: %v", err)
	}
	t.Logf("Playback for episode 1 (video %s): definition=%s quality=%d variants=%d",
		chapters[0].VideoID, playInfo.Selected.Definition, playInfo.Selected.Quality, len(playInfo.Variants))
}
