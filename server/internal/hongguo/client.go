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
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	AppBaseURL   = "https://api5-normal-sinfonlineb.fqnovel.com"
	WebBaseURL   = "https://hongguoduanju.com"
	AppUserAgent = "com.phoenix.read/73532 (Linux; U; Android 7.1.2; zh_CN; 25053RT47C; Build/N2G47H; Cronet/TTNetVersion:04657795 2026-01-23 QuicVersion:c67e9834 2025-09-08)"
	WebUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

var numericID = regexp.MustCompile(`^[0-9]{1,32}$`)
var qualityNumber = regexp.MustCompile(`[0-9]+`)

type Client struct {
	mu          sync.Mutex
	httpClient  *http.Client
	deviceID    string
	installID   string
	details     map[string]detailCacheEntry
	searches    map[string]SearchEntry
	suggestions map[string]suggestionCacheEntry
	danmaku     map[string]danmakuCacheEntry
}

type detailCacheEntry struct {
	drama     Drama
	chapters  []Chapter
	expiresAt time.Time
}

type suggestionCacheEntry struct {
	items     []SearchSuggestion
	expiresAt time.Time
}

type danmakuCacheEntry struct {
	page      DanmakuPage
	expiresAt time.Time
}

func NewClient() *Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 32,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression: false,
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   35 * time.Second,
		},
		deviceID:    newHongguoDeviceID(),
		installID:   newHongguoDeviceID(),
		details:     make(map[string]detailCacheEntry),
		searches:    make(map[string]SearchEntry),
		suggestions: make(map[string]suggestionCacheEntry),
		danmaku:     make(map[string]danmakuCacheEntry),
	}
}

func (c *Client) AppRequest(ctx context.Context, method, path string, extra url.Values, payload any, isComment bool) (map[string]any, error) {
	c.mu.Lock()
	deviceID, installID := c.deviceID, c.installID
	c.mu.Unlock()

	query := url.Values{
		"aid":                   {"8662"},
		"app_name":              {"novelread"},
		"version_code":          {"73532"},
		"version_name":          {"7.3.5.32"},
		"manifest_version_code": {"73532"},
		"update_version_code":   {"73532"},
		"channel":               {"update_64"},
		"device_platform":       {"android"},
		"os":                    {"android"},
		"ssmix":                 {"a"},
		"device_type":           {"25053RT47C"},
		"device_brand":          {"Redmi"},
		"language":              {"zh"},
		"os_api":                {"25"},
		"os_version":            {"7.1.2"},
		"resolution":            {"1280*2772"},
		"dpi":                   {"520"},
		"ac":                    {"wifi"},
		"device_id":             {deviceID},
		"iid":                   {installID},
	}
	for k, v := range extra {
		query[k] = append([]string(nil), v...)
	}

	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		now := time.Now()
		query.Set("_rticket", strconv.FormatInt(now.UnixMilli(), 10))

		fullURL := strings.TrimRight(AppBaseURL, "/") + path + "?" + query.Encode()
		req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}

		req.Header.Set("User-Agent", AppUserAgent)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-XS-From-Web", "0")
		req.Header.Set("Sdk-Version", "2")

		if isComment {
			nonce, nonceErr := newHongguoCommentNonce()
			if nonceErr != nil {
				return nil, errors.New("无法初始化评论签名")
			}
			req.Header.Set("Comment-Source", "601")
			req.Header.Set("Server-Channel", "1000")
			signHongguoCommentRequest(req, nonce, now)
		} else {
			signHongguoRequest(req, body, now)
		}

		if payload != nil {
			req.Header.Set("Content-Type", "application/json; charset=utf-8")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("红果 App 接口 HTTP %d", resp.StatusCode)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return nil, lastErr
			}
			continue
		}

		var result map[string]any
		dec := json.NewDecoder(bytes.NewReader(respBody))
		dec.UseNumber()
		if err := dec.Decode(&result); err != nil {
			lastErr = errors.New("红果 App 接口返回格式异常")
			continue
		}

		code := firstNonEmpty(mapString(result, "code", "Code", "status_code"), mapString(nestedMap(result, "BaseResp"), "StatusCode"))
		if code != "" && code != "0" {
			return nil, fmt.Errorf("红果接口提示: %s", code)
		}

		return result, nil
	}

	return nil, lastErr
}

func (c *Client) FetchWebText(ctx context.Context, targetURL, referer string) (string, error) {
	if referer == "" {
		referer = WebBaseURL + "/"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", WebUserAgent)
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (c *Client) DoHTTP(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

func parseRouterData(raw string) map[string]any {
	idx := regexp.MustCompile(`(?s)(?:window\.)?_ROUTER_DATA\s*=\s*`).FindStringIndex(raw)
	if idx == nil {
		return nil
	}
	var data map[string]any
	dec := json.NewDecoder(strings.NewReader(raw[idx[1]:]))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		return nil
	}
	return data
}

func routerLoaderMap(data map[string]any, names ...string) map[string]any {
	loader, _ := data["loaderData"].(map[string]any)
	for _, name := range names {
		if page, _ := loader[name].(map[string]any); len(page) > 0 {
			return page
		}
	}
	for key, value := range loader {
		for _, name := range names {
			if strings.TrimSuffix(name, "$") != "" && strings.HasPrefix(key, strings.TrimSuffix(name, "$")) {
				if page, _ := value.(map[string]any); len(page) > 0 {
					return page
				}
			}
		}
	}
	return nil
}

func dramaFromAny(v any, defaultCategory string) Drama {
	m, ok := v.(map[string]any)
	if !ok {
		return Drama{}
	}
	vd, _ := m["video_data"].(map[string]any)
	if len(vd) == 0 {
		vd = m
	}
	sourceID := firstNonEmpty(
		mapString(vd, "series_id_str", "series_id"),
		mapString(m, "series_id_str", "series_id"),
		mapString(vd, "keyword"),
		mapString(m, "keyword"),
	)
	if !numericID.MatchString(sourceID) {
		return Drama{}
	}

	title := firstNonEmpty(mapString(vd, "series_title", "series_name", "title"), mapString(m, "series_name", "name"), sourceID)
	cover := firstNonEmpty(mapString(vd, "series_cover", "cover"), mapString(m, "series_cover"))
	intro := firstNonEmpty(mapString(vd, "series_intro", "video_desc"), mapString(m, "series_intro"))
	count := firstNonEmpty(mapString(vd, "episode_cnt"), mapString(m, "episode_cnt"))
	totalEpisodes, _ := strconv.Atoi(count)
	remark := firstNonEmpty(mapString(vd, "episode_right_text"), mapString(m, "episode_right_text"))
	if remark == "" && count != "" {
		remark = "共" + count + "集"
	}

	releaseStatus := "ongoing"
	if mapString(vd, "series_status") == "1" {
		releaseStatus = "finished"
	}

	tags := mapStringSlice(vd, "tags")
	for _, value := range anyList(vd["category_list"]) {
		item, _ := value.(map[string]any)
		if name := mapString(item, "name"); name != "" {
			found := false
			for _, t := range tags {
				if t == name {
					found = true
					break
				}
			}
			if !found {
				tags = append(tags, name)
			}
		}
	}

	genre := mapString(vd, "category_name", "categoryName", "category")
	if genre == "" && len(tags) > 0 {
		genre = tags[0]
	}
	if genre == "" {
		genre = defaultCategory
	}

	heat := firstNonEmpty(mapString(vd, "hot_count"), mapString(vd, "heat"))
	if heat == "" {
		heat = mapString(vd, "series_play_cnt", "play_cnt")
	}

	return Drama{
		ID:            "hongguo:" + sourceID,
		SourceID:      sourceID,
		Title:         title,
		Name:          title,
		Desc:          intro,
		Cover:         cover,
		TotalEpisodes: totalEpisodes,
		EpisodeCount:  count,
		CategoryName:  genre,
		ChannelName:   "红果短剧",
		Remark:        remark,
		Score:         mapString(vd, "score"),
		Views:         mapString(vd, "series_play_cnt", "play_cnt"),
		Heat:          heat,
		Tags:          tags,
		ReleaseStatus: releaseStatus,
	}
}

