package hongguo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var seasonSuffix = regexp.MustCompile(`第\s*([0-9零〇一二两兩三四五六七八九十百]+)\s*([季部])[\p{P}\s]*$`)

func (c *Client) Search(ctx context.Context, keyword string) (SearchEntry, error) {
	keyword = norm.NFKC.String(strings.TrimSpace(keyword))
	if keyword == "" || utf8.RuneCountInString(keyword) > 80 {
		return SearchEntry{}, errors.New("请输入有效搜索词")
	}

	c.mu.Lock()
	if cached, found := c.searches[keyword]; found && time.Now().Before(cached.ExpiresAt) {
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	// 1. Fetch suggestions names
	names, _ := c.fetchSearchNames(ctx, keyword)

	// 2. Fetch search page from web
	pageURL := fmt.Sprintf("%s/search/%s", WebBaseURL, url.PathEscape(keyword))
	body, err := c.FetchWebText(ctx, pageURL, WebBaseURL+"/")

	var dramas []Drama
	seenID := make(map[string]int)
	seenTitle := make(map[string]int)

	addOrMerge := func(d Drama) {
		if d.ID == "" || d.SourceID == "" {
			return
		}
		normTitle := normalizeSearchText(d.DisplayTitle())
		if idx, found := seenID[d.SourceID]; found {
			dramas[idx] = mergeDrama(dramas[idx], d)
			return
		}
		if normTitle != "" {
			if idx, found := seenTitle[normTitle]; found {
				dramas[idx] = mergeDrama(dramas[idx], d)
				return
			}
		}
		seenID[d.SourceID] = len(dramas)
		if normTitle != "" {
			seenTitle[normTitle] = len(dramas)
		}
		dramas = append(dramas, d)
	}

	total := 0
	// First add official web search page results (richer metadata: tags, desc, episodes)
	if err == nil {
		pageData := parseRouterData(body)
		page := routerLoaderMap(pageData, "search_(keyword)/page", "search_")
		rows, ok := page["searchList"].([]any)
		if ok {
			for _, row := range rows {
				d := dramaFromAny(row, "短剧")
				addOrMerge(d)
			}
			t, _ := strconv.Atoi(mapString(page, "totalCount"))
			if t > total {
				total = t
			}
		}
	}

	// Then merge suggestion results
	for _, d := range names {
		addOrMerge(d)
	}

	// Sort results by relevance to query
	queryNorm := normalizeSearchText(keyword)
	sort.SliceStable(dramas, func(i, j int) bool {
		titleI := normalizeSearchText(dramas[i].DisplayTitle())
		titleJ := normalizeSearchText(dramas[j].DisplayTitle())
		if titleI == queryNorm && titleJ != queryNorm {
			return true
		}
		if titleJ == queryNorm && titleI != queryNorm {
			return false
		}
		if strings.HasPrefix(titleI, queryNorm) && !strings.HasPrefix(titleJ, queryNorm) {
			return true
		}
		if strings.HasPrefix(titleJ, queryNorm) && !strings.HasPrefix(titleI, queryNorm) {
			return false
		}
		return false
	})

	entry := SearchEntry{
		Dramas:    dramas,
		Total:     max(total, len(dramas)),
		Limited:   false,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	c.mu.Lock()
	if len(c.searches) >= 64 {
		c.searches = make(map[string]SearchEntry)
	}
	c.searches[keyword] = entry
	c.mu.Unlock()

	return entry, nil
}

func mergeDrama(base, extra Drama) Drama {
	if base.Title == "" || base.Title == base.SourceID {
		base.Title = extra.Title
		base.Name = extra.Name
	}
	if base.Cover == "" {
		base.Cover = extra.Cover
	}
	if base.Desc == "" {
		base.Desc = extra.Desc
	}
	if base.TotalEpisodes == 0 {
		base.TotalEpisodes = extra.TotalEpisodes
		base.EpisodeCount = extra.EpisodeCount
	}
	if base.Remark == "" {
		base.Remark = extra.Remark
	}
	if base.Score == "" {
		base.Score = extra.Score
	}
	if base.CategoryName == "" || base.CategoryName == "短剧" {
		if extra.CategoryName != "" && extra.CategoryName != "短剧" {
			base.CategoryName = extra.CategoryName
		}
	}
	if len(base.Tags) == 0 {
		base.Tags = extra.Tags
	}
	if base.Views == "" {
		base.Views = extra.Views
	}
	if base.Heat == "" {
		base.Heat = extra.Heat
	}
	return base
}

func (c *Client) FetchSuggestions(ctx context.Context, query string) ([]SearchSuggestion, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	c.mu.Lock()
	if cached, found := c.suggestions[query]; found && time.Now().Before(cached.expiresAt) {
		c.mu.Unlock()
		return append([]SearchSuggestion(nil), cached.items...), nil
	}
	c.mu.Unlock()

	params := url.Values{"app_id": {"8662"}, "query": {query}, "count": {"10"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, WebBaseURL+"/incent_resource/suggestion?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", WebUserAgent)
	req.Header.Set("Referer", WebBaseURL+"/")
	req.Header.Set("Accept", "application/json")

	resp, err := c.DoHTTP(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Items []struct {
			Name     string `json:"name"`
			WordType string `json:"word_type"`
		} `json:"suggest_list"`
		Data struct {
			Items []struct {
				Name     string `json:"name"`
				WordType string `json:"word_type"`
			} `json:"suggest_list"`
		} `json:"data"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	_ = dec.Decode(&result)

	rawItems := result.Items
	if len(rawItems) == 0 {
		rawItems = result.Data.Items
	}

	items := make([]SearchSuggestion, 0, len(rawItems))
	seen := make(map[string]bool)
	for _, item := range rawItems {
		name := strings.TrimSpace(item.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		items = append(items, SearchSuggestion{
			Name: name,
			Type: item.WordType,
		})
	}

	c.mu.Lock()
	if len(c.suggestions) >= 128 {
		c.suggestions = make(map[string]suggestionCacheEntry)
	}
	c.suggestions[query] = suggestionCacheEntry{
		items:     items,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	c.mu.Unlock()

	return items, nil
}

func (c *Client) fetchSearchNames(ctx context.Context, keyword string) ([]Drama, error) {
	params := url.Values{"app_id": {"8662"}, "query": {keyword}, "count": {"50"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, WebBaseURL+"/incent_resource/suggestion?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", WebUserAgent)
	req.Header.Set("Referer", WebBaseURL+"/")

	resp, err := c.DoHTTP(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res struct {
		Items []struct {
			Name      string         `json:"name"`
			WordType  string         `json:"word_type"`
			VideoData map[string]any `json:"video_data"`
		} `json:"suggest_list"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	_ = dec.Decode(&res)

	var list []Drama
	seen := make(map[string]bool)
	for _, it := range res.Items {
		if it.WordType != "short_play_name" || len(it.VideoData) == 0 {
			continue
		}
		d := dramaFromAny(map[string]any{"video_data": it.VideoData, "name": it.Name}, "短剧")
		if d.ID != "" && !seen[d.SourceID] {
			seen[d.SourceID] = true
			list = append(list, d)
		}
	}
	return list, nil
}

func normalizeSearchText(text string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return r
	}, norm.NFKC.String(text)))
}
