package hongguo

import (
	"bytes"
	"context"
	"crypto/tls"
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
	KlinkEgdi   string `json:"klink_egdi,omitempty"`
	DeviceToken string `json:"device_token,omitempty"`
	TTReq       string `json:"ttreq,omitempty"`
	DeviceBrand string `json:"device_brand"`
	DeviceType  string `json:"device_type"`
	OSVersion   string `json:"os_version"`
	OSAPI       string `json:"os_api"`
	Resolution  string `json:"resolution"`
	DPI         string `json:"dpi"`
	Channel     string `json:"channel"`
	UserAgent   string `json:"user_agent,omitempty"`
}

func defaultDeviceProfiles() []DeviceProfile {
	return []DeviceProfile{
		{
			DeviceID:    "1342980729399856",
			InstallID:   "1342980729403952",
			CDID:        "a8f4c210-7b1e-4d89-9c3d-1e5a8b7c9d02",
			OpenUDID:    "9774d56d682e549c",
			KlinkEgdi:   "AALJr8OiZ78N5ncFnZGybSZAt52ItCaM990IsL1EdLhL-FeS6K3MmOTh",
			DeviceToken: "AAAUZ5HHSUIKDRTA27ODDDEPZFMNSYT6W5PZJG4W2YW46L73MQHSVJCMC47WOGQW2NYW2TMKUE4FBDI5EGFRAL3ZQ3BMAMI7VHL7R5PFDOQ4XBEB5ZMWIVHJ4FOJ4",
			TTReq:       "1$c95a148c5eef4a4c0aad53fde7c02cd3c2514c53",
			DeviceBrand: "Redmi",
			DeviceType:  "25053RT47C",
			OSVersion:   "12",
			OSAPI:       "25",
			Resolution:  "1280*2772",
			DPI:         "520",
			Channel:     "69310258a",
			UserAgent:   "com.phoenix.read/73932 (Linux; U; Android 12; zh_CN; 25053RT47C; Build/W528JS; Cronet/TTNetVersion:8d40f833 2026-03-03 QuicVersion:21ac1950 2025-11-18)",
		},
		{
			DeviceID:    "1255017861116115",
			InstallID:   "1255017860317411",
			CDID:        "5261d52a-eb52-4d97-90ae-928238d0d215",
			OpenUDID:    "c0dd9fd7e7f37a6c",
			DeviceToken: "AAA7DT6HQUKXVIDQQY4QBRTFQLZOWWCFVQXPMRDTUG2ND45JRLNKSJZLYOYFKIXDCZFQRPFVP7MRJZBHPHUVAUCSLI3BKSCEKLMJZK776KW2QHVRZJWCAUPNOR754",
			TTReq:       "1$3fce9d6c5a359b9e4bf2292e29f8bb393dc1ce66",
			DeviceBrand: "xiaomi",
			DeviceType:  "23127PN0CC",
			OSVersion:   "14",
			OSAPI:       "25",
			Resolution:  "1200*2670",
			DPI:         "460",
			Channel:     "xiaomi_8662_64",
			UserAgent:   "com.phoenix.read/73932 (Linux; U; Android 14; zh_CN; Xiaomi 14; Build/UKQ1.230804.001; Cronet/TTNetVersion:8d40f833 QuicVersion:462f352c 2026-08-31)",
		},
		{
			DeviceID:    "1993888415882919",
			InstallID:   "1993888415887015",
			CDID:        "42b5f13c-db08-40c0-ae3e-6005559ccf37",
			OpenUDID:    "b28a0664c59d011d",
			DeviceToken: "AAA3IECUXUYIRTQFDSLMK3GHQP3XLGW5CFW32MR3CH3U7OMIUDCA45LNTUJQJR3FQL4Q4O3C4OX4BDMAJWAKDREVQJ3NCF6XKHHPA3UUGMWOCEWQILR3OSA6JSLYA",
			TTReq:       "1$7ff3a20288efb6f0a77d4e0c5316b8d56ffddad1",
			DeviceBrand: "xiaomi",
			DeviceType:  "23127PN0CC",
			OSVersion:   "14",
			OSAPI:       "25",
			Resolution:  "1200*2670",
			DPI:         "460",
			Channel:     "xiaomi_8662_64",
			UserAgent:   "com.phoenix.read/73932 (Linux; U; Android 14; zh_CN; Xiaomi 14; Build/UKQ1.230804.001; Cronet/TTNetVersion:8d40f833 QuicVersion:462f352c 2026-08-31)",
		},
		{
			DeviceID:    "3190157066913784",
			InstallID:   "3190157066917880",
			CDID:        "2766eb71-cfb9-459c-ad65-f18992710f81",
			OpenUDID:    "39e48263544334b9",
			DeviceToken: "AAA34LSTPJCHN37ZUR6CKGZTEC4IVTBFXCPJPZ7DAV57ZIRXC6NLV364U6YHQPQNDD34WESPQZCGGWUYPAO7HQZPKSGU3P4M5BSGLZHXY6NAIFURTPROB47LK6LR4",
			TTReq:       "1$3a7d9a34dc7d473a93c32e0c9145ab3589b23d25",
			DeviceBrand: "xiaomi",
			DeviceType:  "23127PN0CC",
			OSVersion:   "14",
			OSAPI:       "25",
			Resolution:  "1200*2670",
			DPI:         "460",
			Channel:     "xiaomi_8662_64",
			UserAgent:   "com.phoenix.read/73932 (Linux; U; Android 14; zh_CN; Xiaomi 14; Build/UKQ1.230804.001; Cronet/TTNetVersion:8d40f833 QuicVersion:462f352c 2026-08-31)",
		},
		{
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
			UserAgent:   "com.phoenix.read/73932 (Linux; U; Android 7.1.2; zh_CN; Xiaomi 14; Build/N2G47H; Cronet/TTNetVersion:8d40f833 QuicVersion:462f352c 2026-08-31)",
		},
	}
}

func loadDeviceProfiles() []DeviceProfile {
	devices := defaultDeviceProfiles()
	seen := make(map[string]bool)
	for _, d := range devices {
		seen[d.DeviceID] = true
	}

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
				if loaded.OSAPI == "" {
					loaded.OSAPI = "25"
				}
				if loaded.Resolution == "" {
					loaded.Resolution = "1200*2670"
				}
				if loaded.DPI == "" {
					loaded.DPI = "460"
				}
				if loaded.Channel == "" {
					loaded.Channel = "xiaomi_8662_64"
				}
				if !seen[loaded.DeviceID] {
					seen[loaded.DeviceID] = true
					devices = append([]DeviceProfile{loaded}, devices...)
				}
			}
		}
	}

	// Try reading spare devices from hgplayer.db if available
	var dbCandidates []string
	if appData := os.Getenv("APPDATA"); appData != "" {
		dbCandidates = append(dbCandidates, filepath.Join(appData, "hgplayer", "hgplayer.db"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dbCandidates = append(dbCandidates, filepath.Join(home, ".config", "hgplayer", "hgplayer.db"))
	}
	for _, dbPath := range dbCandidates {
		if dbData, err := os.ReadFile(dbPath); err == nil && len(dbData) > 0 {
			needle := []byte("device:spares")
			idx := bytes.LastIndex(dbData, needle)
			if idx != -1 {
				jsonStart := idx + len(needle)
				jsonEnd := bytes.Index(dbData[jsonStart:], []byte(`],"current"`))
				if jsonEnd != -1 {
					fullJSON := string(dbData[jsonStart:jsonStart+jsonEnd+len(`],"current"`)]) + ":0}"
					var res struct {
						Devices []DeviceProfile `json:"devices"`
					}
					if err := json.Unmarshal([]byte(fullJSON), &res); err == nil {
						for _, d := range res.Devices {
							if d.DeviceID != "" && !seen[d.DeviceID] {
								seen[d.DeviceID] = true
								if d.OSAPI == "" {
									d.OSAPI = "25"
								}
								devices = append([]DeviceProfile{d}, devices...)
							}
						}
					}
				}
			}
		}
	}

	return devices
}

type Client struct {
	mu             sync.Mutex
	httpClient     *http.Client
	deviceProfiles []DeviceProfile
	deviceIndex    int
	details        map[string]detailCacheEntry
	searches       map[string]SearchEntry
	suggestions    map[string]suggestionCacheEntry
	danmaku        map[string]danmakuCacheEntry
	playbacks      map[string]playbackCacheEntry
	playbackJobs   map[string]*playbackJob
}

func (c *Client) CurrentDevice() DeviceProfile {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.deviceProfiles) == 0 {
		return defaultDeviceProfiles()[0]
	}
	return c.deviceProfiles[c.deviceIndex%len(c.deviceProfiles)]
}

func (c *Client) RotateDevice() DeviceProfile {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.deviceProfiles) <= 1 {
		if len(c.deviceProfiles) == 0 {
			return defaultDeviceProfiles()[0]
		}
		return c.deviceProfiles[0]
	}
	c.deviceIndex = (c.deviceIndex + 1) % len(c.deviceProfiles)
	return c.deviceProfiles[c.deviceIndex]
}

func (c *Client) DevicePool() []DeviceProfile {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]DeviceProfile(nil), c.deviceProfiles...)
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
		ForceAttemptHTTP2:   false,
		TLSNextProto:        make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}

	jar, _ := cookiejar.New(nil)

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Jar:       jar,
			Timeout:   35 * time.Second,
		},
		deviceProfiles: loadDeviceProfiles(),
		details:        make(map[string]detailCacheEntry),
		searches:       make(map[string]SearchEntry),
		suggestions:    make(map[string]suggestionCacheEntry),
		danmaku:        make(map[string]danmakuCacheEntry),
		playbacks:      make(map[string]playbackCacheEntry),
		playbackJobs:   make(map[string]*playbackJob),
	}
}

func (c *Client) AppRequest(ctx context.Context, method, path string, extra url.Values, payload any, isComment bool) (map[string]any, error) {
	dev := c.CurrentDevice()

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
		} else {
			signHongguoAppPlayRequest(req, body, now)
			cookie := fmt.Sprintf("install_id=%s; store-region=cn-hk; store-region-src=did", dev.InstallID)
			if dev.TTReq != "" {
				cookie += "; ttreq=" + dev.TTReq
			}
			req.Header.Set("Cookie", cookie)
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

	heat := firstNonEmpty(mapString(vd, "hot_count"), mapString(vd, "heat"), mapString(nestedMap(vd, "hot_score_data"), "text"))
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
