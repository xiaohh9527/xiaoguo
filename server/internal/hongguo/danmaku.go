package hongguo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const DanmakuMaxDurationMS = 24 * 60 * 60 * 1000

func (c *Client) FetchDanmaku(ctx context.Context, seriesID, videoID string, startMS, durationMS int64) (DanmakuPage, error) {
	seriesID = strings.TrimPrefix(strings.TrimSpace(seriesID), "hongguo:")
	videoID = strings.TrimPrefix(strings.TrimSpace(videoID), "hongguo-cenc://")

	if !numericID.MatchString(seriesID) || !numericID.MatchString(videoID) {
		return DanmakuPage{}, errors.New("弹幕请求参数无效")
	}

	if durationMS <= 0 {
		durationMS = 10 * 60 * 1000 // 10 minutes default
	}

	key := fmt.Sprintf("%s:%s:%d:%d", seriesID, videoID, startMS, durationMS)
	c.mu.Lock()
	if entry, found := c.danmaku[key]; found && time.Now().Before(entry.expiresAt) {
		c.mu.Unlock()
		return entry.page, nil
	}
	c.mu.Unlock()

	body := map[string]any{
		"comment_source":    601,
		"server_channel":    1000,
		"group_id":          videoID,
		"group_type":        30,
		"comment_type":      20,
		"sort":              1,
		"count":             90,
		"cursor":            "",
		"aid":               8662,
		"compliance_status": 0,
		"business_param": map[string]any{
			"book_id":                seriesID,
			"start_offset_time":      startMS,
			"playlet_item_duration":  durationMS,
			"need_danmaku_guide_type": []int{1, 3, 4, 2},
		},
	}

	result, err := c.AppRequest(ctx, http.MethodPost, "/novel/commentapi/comment/list/"+videoID+"/v1/", nil, body, true)
	if err != nil {
		return DanmakuPage{}, err
	}

	page, err := parseDanmakuResult(result, videoID, startMS, durationMS)
	if err != nil {
		return DanmakuPage{}, err
	}

	c.mu.Lock()
	if len(c.danmaku) >= 256 {
		c.danmaku = make(map[string]danmakuCacheEntry)
	}
	c.danmaku[key] = danmakuCacheEntry{
		page:      page,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	c.mu.Unlock()

	return page, nil
}

func parseDanmakuResult(result map[string]any, videoID string, start, duration int64) (DanmakuPage, error) {
	data := nestedMap(result, "data")
	rows, valid := data["data_list"].([]any)
	next, _ := strconv.ParseInt(mapString(nestedMap(data, "extra"), "next_query_danmaku_list_time"), 10, 64)
	if !valid {
		return DanmakuPage{}, errors.New("弹幕格式异常")
	}

	page := DanmakuPage{
		EpisodeID: videoID,
		Items:     []DanmakuItem{},
		StartMS:   start,
		NextMS:    next,
	}
	if page.NextMS <= start {
		page.NextMS = duration
	}

	var cursor struct {
		Total int64 `json:"danmaku_count"`
	}
	if raw := mapString(nestedMap(data, "common_list_info"), "cursor"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &cursor)
		page.Total = cursor.Total
	}

	seen := make(map[string]bool)
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		comment := nestedMap(row, "comment")
		common := nestedMap(comment, "common")
		if mapString(common, "group_id") != videoID || mapString(common, "status") != "1" {
			continue
		}

		offsetTime, err := strconv.ParseInt(mapString(nestedMap(comment, "expand"), "offset_time"), 10, 64)
		if err != nil {
			continue
		}

		text := strings.TrimSpace(strings.Map(func(r rune) rune {
			if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
				return ' '
			}
			return r
		}, mapString(nestedMap(common, "content"), "text")))

		id := mapString(comment, "comment_id")
		if text == "" || id == "" || seen[id] {
			continue
		}
		seen[id] = true

		if runes := []rune(text); len(runes) > 180 {
			text = string(runes[:180]) + "…"
		}

		page.Items = append(page.Items, DanmakuItem{
			ID:     id,
			Text:   text,
			TimeMS: offsetTime,
		})
	}

	sort.SliceStable(page.Items, func(i, j int) bool {
		return page.Items[i].TimeMS < page.Items[j].TimeMS
	})

	return page, nil
}
