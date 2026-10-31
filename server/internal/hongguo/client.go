package hongguo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	AppBaseURL   = "https://api5-normal-sinfonlineb.fqnovel.com"
	WebBaseURL   = "https://hongguoduanju.com"
	AppUserAgent = "com.phoenix.read/73932 (Linux; U; Android 7.1.2; zh_CN; Xiaomi 14; Build/N2G47H; Cronet/TTNetVersion:8d40f833 QuicVersion:462f352c 2026-08-31)"
	WebUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

var numericID = regexp.MustCompile(`^[0-9]{1,32}$`)
var qualityNumber = regexp.MustCompile(`[0-9]+`)

type DeviceProfile struct {
	DeviceID    string `json:"device_id"`
	InstallID   string `json:"install_id"`
	IID         string `json:"iid,omitempty"`
	CDID        string `json:"cdid"`
	OpenUDID    string `json:"openudid"`
	DeviceToken string `json:"device_token,omitempty"`
	TTReq       string `json:"ttreq,omitempty"`
	DeviceBrand string `json:"device_brand"`
	DeviceType  string `json:"device_type"`
	OSVersion   string `json:"os_version"`
	OSAPI       string `json:"os_api"`
	Resolution  string `json:"resolution"`
	DPI         string `json:"dpi"`
	Channel     string `json:"channel"`
}

func defaultDeviceProfile() DeviceProfile {
	return DeviceProfile{
		DeviceID:    "797614684305320",
		InstallID:   "797614683457464",
		CDID:        "f222479a-ffa6-4262-bab8-279d9f701ee8",
		OpenUDID:    "892bdf31d465060d",
		DeviceToken: "AAARG6S7APMJXJ3HIBZZGM3YMBLRS2TOFDNYTTHYSS7WQXDM4MPDYOJQYRS3M3YVHVE3NEQD3CAQBHF47A22CD5A4RGK72QM2VRDETBDGDUD4ZOPEWIUAVV23AEYA",
		TTReq:       "1$df51b637fb4664e066427b792fa89b0ee9ae9eac",
		DeviceBrand: "xiaomi",
		DeviceType:  "23127PN0CC",
		OSVersion:   "7.1.2",
		OSAPI:       "25",
		Resolution:  "1200*2670",
		DPI:         "460",
		Channel:     "xiaomi_8662_64",
	}
}

func loadDeviceProfile() DeviceProfile {
	dev := defaultDeviceProfile()
	candidates := []string{
		"data/device.json",
		"device.json",
	}
	if appData := os.Getenv("APPDATA"); appData != "" {
		candidates = append(candidates, filepath.Join(appData, "hgplayer", "device.json"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".config", "hgplayer", "device.json"))
	}

	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
			var loaded DeviceProfile
			if err := json.Unmarshal(data, &loaded); err == nil && loaded.DeviceID != "" {
				if loaded.CDID != "" {
					dev.CDID = loaded.CDID
				}
				if loaded.OpenUDID != "" {
					dev.OpenUDID = loaded.OpenUDID
				}
				if loaded.DeviceID != "" {
					dev.DeviceID = loaded.DeviceID
				}
				if loaded.InstallID != "" {
					dev.InstallID = loaded.InstallID
				} else if loaded.IID != "" {
					dev.InstallID = loaded.IID
				}
				if loaded.Channel != "" {
					dev.Channel = loaded.Channel
				}
				break
			}
		}
	}
	return dev
}

type Client struct {
	mu           sync.Mutex
	httpClient   *http.Client
	device       DeviceProfile
	details      map[string]detailCacheEntry
	searches     map[string]SearchEntry
	suggestions  map[string]suggestionCacheEntry
	danmaku      map[string]danmakuCacheEntry
	playbacks    map[string]playbackCacheEntry
	playbackJobs map[string]*playbackJob
}

type playbackCacheEntry struct {
	info      PlaybackInfo
	expiresAt time.Time
}

type playbackJob struct {
	done chan struct{}
	info PlaybackInfo
	err  error
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
		MaxIdleConns:        120,
		MaxIdleConnsPerHost: 40,
		IdleConnTimeout:     120 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	}

	jar, _ := cookiejar.New(nil)

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Jar:       jar,
			Timeout:   35 * time.Second,
		},
		device:       loadDeviceProfile(),
		details:      make(map[string]detailCacheEntry),
		searches:     make(map[string]SearchEntry),
		suggestions:  make(map[string]suggestionCacheEntry),
		danmaku:      make(map[string]danmakuCacheEntry),
		playbacks:    make(map[string]playbackCacheEntry),
		playbackJobs: make(map[string]*playbackJob),
	}
}

func (c *Client) AppRequest(ctx context.Context, method, path string, extra url.Values, payload any, isComment bool) (map[string]any, error) {
	c.mu.Lock()
	dev := c.device
	c.mu.Unlock()

	query := url.Values{
		"aid":                   {"8662"},
		"app_name":              {"novelread"},
		"version_code":          {"73932"},
		"version_name":          {"7.3.9.32"},
		"manifest_version_code": {"73932"},
		"update_version_code":   {"73932"},
		"channel":               {dev.Channel},
		"device_platform":       {"android"},
		"os":                    {"android"},
		"ssmix":                 {"a"},
		"device_type":           {dev.DeviceType},
		"device_brand":          {dev.DeviceBrand},
		"language":              {"zh"},
		"os_api":                {"25"},
		"os_version":            {"7.1.2"},
		"resolution":            {dev.Resolution},
		"dpi":                   {dev.DPI},
		"ac":                    {"wifi"},
		"device_id":             {dev.DeviceID},
		"iid":                   {dev.InstallID},
		"cdid":                  {dev.CDID},
		"openudid":              {dev.OpenUDID},
	}
	for k, v := range extra {
		if len(v) == 0 || (len(v) == 1 && v[0] == "") {
			delete(query, k)
		} else {
			query[k] = append([]string(nil), v...)
		}
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
		} else if strings.Contains(path, "video_detail") {
			nonce, nonceErr := newHongguoCommentNonce()
			if nonceErr == nil {
				req.Header.Set("Comment-Source", "601")
				req.Header.Set("Server-Channel", "1000")
				signHongguoCommentRequest(req, nonce, now)
			}
		} else if !strings.Contains(path, "video_model") && attempt == 0 {
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

		if len(respBody) == 0 {
			lastErr = errors.New("红果 App 接口返回空内容")
			if attempt >= 1 {
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
