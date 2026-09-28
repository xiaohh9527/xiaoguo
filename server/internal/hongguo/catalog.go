package hongguo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var Genres = []struct {
	Key   string `json:"key"`
	Scene string `json:"scene"`
	Name  string `json:"name"`
}{
	{Key: "short_play", Scene: "default", Name: "真人剧"},
	{Key: "comic_series", Scene: "comic_series", Name: "漫剧"},
	{Key: "ai_series", Scene: "ai_series", Name: "AI剧"},
	{Key: "comic", Scene: "comic", Name: "动漫"},
}

func (c *Client) FetchRecommendations(ctx context.Context, query RecommendationQuery) (RecommendationPage, error) {
	scene, category := "default", "真人剧"
	if query.Genre != "" {
		for _, g := range Genres {
			if g.Key == query.Genre {
				scene, category = g.Scene, g.Name
				break
			}
		}
	} else {
		query.Genre = "short_play"
	}

	seen := make(map[string]bool, len(query.Seen))
	var filterIDs []string
	for _, id := range query.Seen {
		clean := strings.TrimPrefix(id, "hongguo:")
		if !seen[clean] {
			seen[clean] = true
			filterIDs = append(filterIDs, clean)
		}
	}

	payload := map[string]any{
		"req_scene":          scene,
		"offset":             query.Offset,
		"limit":              18,
		"req_type":           "only_content",
		"need_selector_panel": false,
		"client_req_type":    3,
		"session_id":         query.SessionID,
		"filter_ids":         strings.Join(filterIDs, ","),
		"select_items": map[string]any{
			"genre":              []string{query.Genre},
			"sort":               []string{},
			"gender":             []string{},
			"category_dim_theme": []string{},
			"category_dim_role":  []string{},
			"category_dim_epoch": []string{},
			"online_time":        []string{},
			"creation_status":    []string{},
		},
	}
	if query.Offset > 0 {
		payload["client_req_type"] = 2
	}

	result, err := c.AppRequest(ctx, http.MethodPost, "/reading/distribution/category/landpage/v/", nil, payload, false)
	if err != nil {
		// Fallback to web category
		return c.fetchWebCatalogFallback(ctx, query.Genre, category, query.Offset/18+1)
	}

	data := nestedMap(result, "data")
	rows, valid := data["video_data"].([]any)
	hasMore, _ := data["has_more"].(bool)
	next, parseErr := strconv.Atoi(mapString(data, "next_offset"))
	if !valid {
		return c.fetchWebCatalogFallback(ctx, query.Genre, category, query.Offset/18+1)
	}

	var items []Drama
	for _, row := range rows {
		d := dramaFromAny(row, category)
		if d.ID != "" && !seen[d.SourceID] {
			seen[d.SourceID] = true
			items = append(items, d)
		}
	}

	if parseErr != nil {
		next = query.Offset + len(rows)
	}

	return RecommendationPage{
		Dramas:     items,
		NextOffset: next,
		SessionID:  mapString(data, "session_id"),
		HasMore:    hasMore,
	}, nil
}

func (c *Client) fetchWebCatalogFallback(ctx context.Context, genreKey, category string, page int) (RecommendationPage, error) {
	route := "real-drama"
	switch genreKey {
	case "comic_series":
		route = "comic-drama"
	case "ai_series":
		route = "ai-drama"
	case "comic":
		route = "comic"
	}

	raw, err := c.FetchWebText(ctx, fmt.Sprintf("%s/category/%s?page=%d", WebBaseURL, route, page), WebBaseURL+"/")
	if err != nil {
		return RecommendationPage{}, err
	}

	data := parseRouterData(raw)
	pageData := routerLoaderMap(data, "category_page", "category_$")
	if len(pageData) == 0 {
		return RecommendationPage{}, errors.New("网页分类数据不可用")
	}

	items := anyList(pageData["recommendList"])
	var dramas []Drama
	for _, item := range items {
		if d := dramaFromAny(item, category); d.ID != "" {
			dramas = append(dramas, d)
		}
	}

	totalPages, _ := strconv.Atoi(mapString(nestedMap(pageData, "pagination"), "totalPages"))
	hasMore := page < totalPages

	return RecommendationPage{
		Dramas:     dramas,
		NextOffset: (page) * 18,
		SessionID:  "",
		HasMore:    hasMore,
	}, nil
}

func (c *Client) FetchWebCategoryRoute(ctx context.Context, route, category string, page int) ([]Drama, int, error) {
	body, err := c.FetchWebText(ctx, fmt.Sprintf("%s/category/%s?page=%d", WebBaseURL, route, page), WebBaseURL+"/")
	if err != nil {
		return nil, 0, err
	}
	data := parseRouterData(body)
	pageMap := routerLoaderMap(data, "category_page", "category_$")
	if len(pageMap) == 0 {
		return nil, 0, errors.New("红果分类数据不可用")
	}
	items := anyList(pageMap["recommendList"])
	out := make([]Drama, 0, len(items))
	for _, item := range items {
		if dr := dramaFromAny(item, category); dr.ID != "" {
			out = append(out, dr)
		}
	}
	pages, _ := strconv.Atoi(mapString(nestedMap(pageMap, "pagination"), "totalPages"))
	return out, pages, nil
}
