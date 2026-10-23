package hongguo_test

import (
	"context"
	"testing"
	"time"

	"xiaoguo/server/internal/hongguo"
)

func TestResolvePlayback(t *testing.T) {
	client := hongguo.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Use known popular series ID and video ID
	seriesID := "7611094547900156990"
	videoID := "7611097540447980606"

	info, err := client.ResolvePlayback(ctx, seriesID, videoID)
	if err != nil {
		t.Fatalf("ResolvePlayback failed: %v", err)
	}

	if info.VideoID != videoID {
		t.Errorf("expected videoID %s, got %s", videoID, info.VideoID)
	}

	if len(info.Variants) == 0 {
		t.Fatalf("expected variants, got 0")
	}

	t.Logf("Successfully resolved playback for video %s:", videoID)
	t.Logf("Selected: Definition=%s, Quality=%d, Codec=%s, Encrypted=%v, CENCKeyHex=%s",
		info.Selected.Definition, info.Selected.Quality, info.Selected.Codec, info.Selected.Encrypted, info.Selected.CENCKeyHex)
	t.Logf("Total variants: %d", len(info.Variants))
	for i, v := range info.Variants {
		t.Logf("  [%d] Definition=%s, Quality=%d, Codec=%s, Encrypted=%v, Size=%d, URL=%s",
			i, v.Definition, v.Quality, v.Codec, v.Encrypted, v.Size, v.URL[:40]+"...")
	}
}

func TestStreamDecryption(t *testing.T) {
	client := hongguo.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	seriesID := "7611094547900156990"
	videoID := "7611097540447980606"

	info, err := client.ResolvePlayback(ctx, seriesID, videoID)
	if err != nil {
		t.Fatalf("ResolvePlayback failed: %v", err)
	}

	// Pick lowest quality variant for fast test
	var targetVariant hongguo.MediaVariant
	for i := len(info.Variants) - 1; i >= 0; i-- {
		if info.Variants[i].Quality <= 480 {
			targetVariant = info.Variants[i]
			break
		}
	}
	if targetVariant.URL == "" {
		targetVariant = info.Variants[len(info.Variants)-1]
	}

	tempDir := t.TempDir()
	sm := hongguo.NewStreamManager(tempDir, client)

	path, err := sm.GetDecryptedVideo(ctx, seriesID, videoID, targetVariant)
	if err != nil {
		t.Fatalf("GetDecryptedVideo failed: %v", err)
	}

	t.Logf("Successfully downloaded and decrypted video to: %s", path)
}

func TestDetailAndMultipleEpisodesPlayback(t *testing.T) {
	client := hongguo.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	seriesID := "7611094547900156990"
	detail, chapters, err := client.FetchDetail(ctx, seriesID)
	if err != nil {
		t.Fatalf("FetchDetail failed: %v", err)
	}

	t.Logf("Drama: %s, Total Chapters: %d", detail.Title, len(chapters))
	if len(chapters) == 0 {
		t.Fatalf("expected chapters, got 0")
	}

	// Test episode 1 and episode 10
	testIndices := []int{0}
	if len(chapters) >= 10 {
		testIndices = append(testIndices, 9)
	}

	for _, idx := range testIndices {
		ch := chapters[idx]
		t.Logf("Resolving chapter [%d]: Title=%s, VideoID=%s", idx+1, ch.Title, ch.VideoID)
		info, err := client.ResolvePlayback(ctx, seriesID, ch.VideoID)
		if err != nil {
			t.Fatalf("ResolvePlayback failed for chapter %d (%s): %v", idx+1, ch.VideoID, err)
		}
		if len(info.Variants) == 0 {
			t.Fatalf("No variants for chapter %d (%s)", idx+1, ch.VideoID)
		}
		t.Logf("Chapter [%d] resolved successfully: %d variants, Best Quality=%s", idx+1, len(info.Variants), info.Selected.Definition)
	}
}
