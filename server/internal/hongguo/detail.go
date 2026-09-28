package hongguo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (c *Client) FetchDetail(ctx context.Context, seriesID string) (Drama, []Chapter, error) {
	seriesID = strings.TrimPrefix(strings.TrimSpace(seriesID), "hongguo:")
	seriesID = strings.TrimPrefix(seriesID, "hg-series-v1:")
	if !numericID.MatchString(seriesID) {
		return Drama{}, nil, errors.New("无效的短剧 ID")
	}

	c.mu.Lock()
	if cached, found := c.details[seriesID]; found && time.Now().Before(cached.expiresAt) {
		c.mu.Unlock()
		return cached.drama, append([]Chapter(nil), cached.chapters...), nil
	}
	c.mu.Unlock()

	// Try App detail first
	drama, chapters, appErr := c.fetchAppDetail(ctx, seriesID)
	if appErr == nil && len(chapters) > 0 {
		c.mu.Lock()
		c.details[seriesID] = detailCacheEntry{
			drama:     drama,
			chapters:  chapters,
			expiresAt: time.Now().Add(10 * time.Minute),
		}
		c.mu.Unlock()
		return drama, chapters, nil
	}

	// Fallback to Web detail
	drama, chapters, webErr := c.fetchWebDetail(ctx, seriesID)
	if webErr == nil && len(chapters) > 0 {
		c.mu.Lock()
		c.details[seriesID] = detailCacheEntry{
			drama:     drama,
			chapters:  chapters,
			expiresAt: time.Now().Add(10 * time.Minute),
		}
		c.mu.Unlock()
		return drama, chapters, nil
	}

	return Drama{}, nil, fmt.Errorf("获取短剧详情失败: app=%v, web=%w", appErr, webErr)
}

func (c *Client) fetchAppDetail(ctx context.Context, seriesID string) (Drama, []Chapter, error) {
	result, err := c.AppRequest(ctx, http.MethodPost, "/novel/player/video_detail/v1/", nil, map[string]any{"series_id": seriesID}, false)
	if err != nil {
		return Drama{}, nil, err
	}

	detail := nestedMap(result, "data", "video_data")
	if len(detail) == 0 {
		return Drama{}, nil, errors.New("video_data is empty")
	}

	drama := dramaFromAny(detail, "短剧")
	rows := anyList(detail["video_list"])
	if len(rows) == 0 {
		return drama, nil, errors.New("分集列表为空")
	}

	chapters := make([]Chapter, 0, len(rows))
	seen := make(map[string]bool)
	for _, row := range rows {
		v, _ := row.(map[string]any)
		vid := mapString(v, "vid")
		if !numericID.MatchString(vid) || seen[vid] {
			continue
		}
		seen[vid] = true

		idx, _ := strconv.Atoi(mapString(v, "vid_index"))
		if idx <= 0 {
			idx = len(chapters) + 1
		}

		chapters = append(chapters, Chapter{
			ID:             fmt.Sprintf("hongguo:%s:%s", seriesID, vid),
			SeriesID:       seriesID,
			VideoID:        vid,
			Index:          idx,
			Title:          fmt.Sprintf("第%d集", idx),
			VideoURL:       "hongguo-cenc://" + vid,
			CurrentEpisode: strconv.Itoa(idx),
		})
	}

	sort.Slice(chapters, func(i, j int) bool {
		return chapters[i].Index < chapters[j].Index
	})

	return drama, chapters, nil
}

func (c *Client) fetchWebDetail(ctx context.Context, seriesID string) (Drama, []Chapter, error) {
	body, err := c.FetchWebText(ctx, WebBaseURL+"/detail?series_id="+url.QueryEscape(seriesID), WebBaseURL+"/")
	if err != nil {
		return Drama{}, nil, err
	}

	data := parseRouterData(body)
	page := routerLoaderMap(data, "detail_page", "detail_")
	detail, _ := page["seriesDetail"].(map[string]any)
	if len(detail) == 0 {
		return Drama{}, nil, errors.New("网页详情数据为空")
	}

	drama := dramaFromAny(detail, "短剧")
	vids := anyList(detail["vid_list"])
	chapters := make([]Chapter, 0, len(vids))
	for i, v := range vids {
		vid := strings.TrimSpace(fmt.Sprint(v))
		if vid == "" || vid == "<nil>" {
			continue
		}
		idx := i + 1
		chapters = append(chapters, Chapter{
			ID:             fmt.Sprintf("hongguo:%s:%s", seriesID, vid),
			SeriesID:       seriesID,
			VideoID:        vid,
			Index:          idx,
			Title:          fmt.Sprintf("第%d集", idx),
			VideoURL:       "hongguo-cenc://" + vid,
			CurrentEpisode: strconv.Itoa(idx),
		})
	}

	return drama, chapters, nil
}
