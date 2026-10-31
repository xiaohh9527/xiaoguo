package hongguo

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"math/bits"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const backupPlaybackAPI = "https://djapi.999888456.xyz/api/hongguo/play"

var playHosts = []string{
	"https://api5-normal-lq.fqnovel.com",
	"https://api5-normal-hl.fqnovel.com",
	"https://api5-normal-lf.fqnovel.com",
	"https://api.fqnovel.com",
	"https://api3-normal-lq.fqnovel.com",
	"https://api3-normal-hl.fqnovel.com",
}

func (c *Client) ResolvePlayback(ctx context.Context, seriesID, videoID string) (PlaybackInfo, error) {
	seriesID = strings.TrimPrefix(strings.TrimSpace(seriesID), "hongguo:")
	videoID = strings.TrimPrefix(strings.TrimSpace(videoID), "hongguo-cenc://")

	if !numericID.MatchString(videoID) {
		return PlaybackInfo{}, errors.New("无效的视频 ID")
	}
	if seriesID == "" {
		seriesID = videoID
	}

	cacheKey := seriesID + ":" + videoID

	// 1. Check memory cache (0ms instant hit)
	c.mu.Lock()
	if entry, ok := c.playbacks[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		c.mu.Unlock()
		return entry.info, nil
	}

	// 2. Singleflight: deduplicate in-flight requests for the same video
	if job, ok := c.playbackJobs[cacheKey]; ok {
		c.mu.Unlock()
		select {
		case <-job.done:
			return job.info, job.err
		case <-ctx.Done():
			return PlaybackInfo{}, ctx.Err()
		}
	}

	job := &playbackJob{done: make(chan struct{})}
	c.playbackJobs[cacheKey] = job
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.playbackJobs, cacheKey)
		close(job.done)
		c.mu.Unlock()
	}()

	info, err := c.doResolvePlayback(ctx, seriesID, videoID)
	if err != nil {
		job.err = err
		return PlaybackInfo{}, err
	}

	c.mu.Lock()
	if len(c.playbacks) >= 2048 {
		c.playbacks = make(map[string]playbackCacheEntry)
	}
	c.playbacks[cacheKey] = playbackCacheEntry{
		info:      info,
		expiresAt: time.Now().Add(45 * time.Minute),
	}
	c.mu.Unlock()

	job.info = info
	return info, nil
}

func (c *Client) doResolvePlayback(ctx context.Context, seriesID, videoID string) (PlaybackInfo, error) {
	// 1. Try Web player first (uses official unencrypted fallback-play-info: 100% reliable, zero risk of WAF block)
	info, webErr := c.resolveWebMedia(ctx, seriesID, videoID)
	if webErr == nil && len(info.Variants) > 0 {
		return info, nil
	}

	// 2. Try App video model
	info, appErr := c.resolveAppMedia(ctx, seriesID, videoID)
	if appErr == nil && len(info.Variants) > 0 {
		return info, nil
	}

	// 3. Try Backup API
	info, apiErr := c.resolveBackupMedia(ctx, seriesID, videoID)
	if apiErr == nil && len(info.Variants) > 0 {
		return info, nil
	}

	return PlaybackInfo{}, fmt.Errorf("解析播放地址失败: web=%v, app=%v, backup=%w", webErr, appErr, apiErr)
}

func parseVideoModelMap(result map[string]any, videoID string) map[string]any {
	data := nestedMap(result, "data")
	var rawModel any
	if videoID != "" {
		if item := nestedMap(data, videoID); len(item) > 0 {
			rawModel = item["video_model"]
		}
	}
	if rawModel == nil {
		rawModel = data["video_model"]
	}
	if rawModel == nil {
		for _, v := range data {
			if item, ok := v.(map[string]any); ok {
				if m, ok := item["video_model"]; ok && m != nil {
					rawModel = m
					break
				}
			}
		}
	}

	var model map[string]any
	if m, ok := rawModel.(map[string]any); ok {
		model = m
	} else if encoded, ok := rawModel.(string); ok && encoded != "" {
		var decoded map[string]any
		dec := json.NewDecoder(strings.NewReader(encoded))
		dec.UseNumber()
		if err := dec.Decode(&decoded); err == nil {
			model = decoded
		}
	}
	return model
}

func parseRawVariants(model map[string]any) []map[string]any {
	var rawVariants []map[string]any
	if list, ok := model["video_list"].([]any); ok {
		for _, v := range list {
			if m, ok := v.(map[string]any); ok {
				rawVariants = append(rawVariants, m)
			}
		}
	} else if m, ok := model["video_list"].(map[string]any); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if item, ok := m[k].(map[string]any); ok {
				rawVariants = append(rawVariants, item)
			}
		}
	}
	return rawVariants
}

func (c *Client) resolveAppMedia(ctx context.Context, seriesID, videoID string) (PlaybackInfo, error) {
	c.mu.Lock()
	dev := c.device
	c.mu.Unlock()

	devID := dev.DeviceID
	iid := dev.InstallID
	cdid := dev.CDID
	openudid := dev.OpenUDID
	if devID == "" || devID == "797614684305320" {
		devID = "3190157066913784"
		iid = "3190157066917880"
		cdid = "2766eb71-cfb9-459c-ad65-f18992710f81"
		openudid = "39e48263544334b9"
	}

	bizParam := map[string]any{
		"detail_page_version":       0,
		"device_level":              4,
		"disable_digg_stat":         false,
		"need_all_video_definition": true,
		"need_mp4_align":            false,
		"use_os_player":             false,
		"use_server_dns":            false,
		"video_platform":            1024,
	}

	contentTypes := []string{"1004", "1001"}
	var lastErr error

	for _, ct := range contentTypes {
		payload := map[string]any{
			"biz_param": bizParam,
			"mixed_video_id_map": map[string]any{
				ct: []string{videoID},
			},
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return PlaybackInfo{}, err
		}

		now := time.Now()
		rnd, _ := rand.Int(rand.Reader, big.NewInt(0x35a4e900))
		reqID := fmt.Sprintf("%d-%d", now.UnixMilli(), rnd.Int64()+0x5f5e100)

		query := url.Values{
			"aid":                       {"8662"},
			"app_name":                  {"novelread"},
			"version_code":              {"73932"},
			"version_name":              {"7.3.9.32"},
			"manifest_version_code":     {"73932"},
			"update_version_code":       {"73932"},
			"channel":                   {dev.Channel},
			"device_platform":           {"android"},
			"os":                        {"android"},
			"ssmix":                     {"a"},
			"device_type":               {dev.DeviceType},
			"device_brand":              {dev.DeviceBrand},
			"language":                  {"zh"},
			"os_api":                    {"25"},
			"os_version":                {"7.1.2"},
			"resolution":                {dev.Resolution},
			"dpi":                       {dev.DPI},
			"ac":                        {"wifi"},
			"host_abi":                  {"arm64-v8a"},
			"dragon_device_type":        {"phone"},
			"compliance_status":         {"0"},
			"need_personal_recommend":   {"1"},
			"player_so_load":            {"1"},
			"is_android_pad_screen":     {"0"},
			"pv_player":                 {"1"},
			"passport-sdk-version":      {"50514"},
			"use_new_token_expire_rule": {"1"},
			"cronet_version":            {"8d40f833_2026-03-03"},
				"ttnet_version":             {"4.2.243.31-douyin"},
				"use_store_region_cookie":   {"1"},
				"device_id":                 {devID},
				"iid":                       {iid},
				"cdid":                      {cdid},
				"openudid":                  {openudid},
				"_rticket":                  {strconv.FormatInt(now.UnixMilli(), 10)},
			}

			h := md5.Sum(body)
			stub := strings.ToUpper(hex.EncodeToString(h[:]))

			for _, baseURL := range playHosts {
				fullURL := fmt.Sprintf("%s/novel/player/multi_video_model/v1/?%s", baseURL, query.Encode())
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(body))
				if err != nil {
					lastErr = err
					continue
				}

				req.Header.Set("User-Agent", "com.phoenix.read/73932 (Linux; U; Android 11; zh_CN; M2012K11AC; Build/RKQ1.200826.002; Cronet/TTNetVersion:8d40f833 2026-03-03 QuicVersion:21ac1950 2025-11-18)")
				req.Header.Set("Accept", "application/json; charset=utf-8,application/x-protobuf")
				req.Header.Set("Content-Type", "application/json; charset=utf-8")
				req.Header.Set("X-XS-From-Web", "0")
				req.Header.Set("X-VC-Bdturing-Sdk-Version", "4.0.3.cn")
				req.Header.Set("Sdk-Version", "2")
				req.Header.Set("Passport-Sdk-Version", "50514")
				req.Header.Set("X-TT-Store-Region", "cn-gd")
				req.Header.Set("X-TT-Store-Region-Src", "did")
				req.Header.Set("X-SS-DP", "8662")
				req.Header.Set("X-Reading-Request-Id", reqID)
				req.Header.Set("X-SS-Req-Ticket", strconv.FormatInt(now.UnixMilli(), 10))
				req.Header.Set("X-SS-STUB", stub)
				req.Header.Set("Cookie", fmt.Sprintf("install_id=%s; store-region=cn-hk; store-region-src=did", iid))

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

			if resp.StatusCode != http.StatusOK || len(respBody) == 0 {
				lastErr = fmt.Errorf("HTTP %d, len %d", resp.StatusCode, len(respBody))
				continue
			}

			var result struct {
				Code int `json:"code"`
				Data map[string]struct {
					VideoModel string `json:"video_model"`
				} `json:"data"`
			}
			if err := json.Unmarshal(respBody, &result); err != nil {
				lastErr = err
				continue
			}

			item, ok := result.Data[videoID]
			if !ok || item.VideoModel == "" {
				lastErr = fmt.Errorf("未找到分集 %s 播放模型", videoID)
				continue
			}

			var model struct {
				VideoDuration float64 `json:"video_duration"`
				VideoList     []struct {
					MainURL     string `json:"main_url"`
					BackupURL   string `json:"backup_url_1"`
					BackupURL2  string `json:"backup_url"`
					SpadeA      string `json:"spade_a"`
					EncryptInfo struct {
						Encrypt bool   `json:"encrypt"`
						Method  string `json:"encryption_method"`
						KID     string `json:"kid"`
						SpadeA  string `json:"spade_a"`
					} `json:"encrypt_info"`
					VideoMeta struct {
						Definition string `json:"definition"`
						Quality    string `json:"quality"`
						CodecType  string `json:"codec_type"`
						VHeight    int    `json:"vheight"`
						VWidth     int    `json:"vwidth"`
						Size       int64  `json:"size"`
					} `json:"video_meta"`
				} `json:"video_list"`
			}
			if err := json.Unmarshal([]byte(item.VideoModel), &model); err != nil {
				lastErr = err
				continue
			}

			var variants []MediaVariant
			for _, v := range model.VideoList {
				codec := strings.ToLower(v.VideoMeta.CodecType)
				if codec == "bytevc2" {
					continue // 跳过第三方播放器无法解码画面黑屏的 bytevc2
				}
				u := strings.TrimSpace(v.MainURL)
				if u == "" {
					u = strings.TrimSpace(v.BackupURL)
				}
				if u == "" {
					u = strings.TrimSpace(v.BackupURL2)
				}
				if u == "" {
					continue
				}

				spadeA := strings.TrimSpace(v.EncryptInfo.SpadeA)
				if spadeA == "" {
					spadeA = strings.TrimSpace(v.SpadeA)
				}

				var keyHex string
				isEnc := spadeA != "" || v.EncryptInfo.Encrypt || v.EncryptInfo.Method == "cenc-aes-ctr"
				if spadeA != "" {
					k, err := DecodeHongguoContentKey(spadeA)
					if err == nil && len(k) == 16 {
						keyHex = hex.EncodeToString(k)
						isEnc = true
					}
				}

				def := v.VideoMeta.Definition
				height := v.VideoMeta.VHeight
				width := v.VideoMeta.VWidth
				quality := height
				if strings.Contains(def, "1080") {
					quality = 1080
				} else if strings.Contains(def, "720") {
					quality = 720
				} else if strings.Contains(def, "540") {
					quality = 540
				} else if strings.Contains(def, "480") {
					quality = 480
				} else if strings.Contains(def, "360") {
					quality = 360
				}
				if quality == 0 {
					quality = 720
				}

				variants = append(variants, MediaVariant{
					URL:        u,
					Codec:      codec,
					Definition: def,
					Quality:    quality,
					Width:      width,
					Height:     height,
					Size:       v.VideoMeta.Size,
					Encrypted:  isEnc,
					CENCKeyHex: keyHex,
				})
			}

			if len(variants) == 0 {
				lastErr = fmt.Errorf("分集 %s 无可用兼容编码视频流", videoID)
				continue
			}

			// Sort by quality descending: 1080p > 720p > 540p > 480p > 360p
			sort.SliceStable(variants, func(i, j int) bool {
				if variants[i].Quality != variants[j].Quality {
					return variants[i].Quality > variants[j].Quality
				}
				if variants[i].Codec == "h264" && variants[j].Codec != "h264" {
					return true
				}
				return false
			})

			return PlaybackInfo{
				SeriesID: seriesID,
				VideoID:  videoID,
				Duration: model.VideoDuration,
				Selected: variants[0],
				Variants: variants,
			}, nil
		}
	}

	return PlaybackInfo{}, lastErr
}

func (c *Client) fetchWebProtectedFallbackPlayInfo(ctx context.Context, seriesID, videoID string) (PlaybackInfo, error) {
	payload := map[string]string{
		"series_id": seriesID,
		"video_id":  videoID,
		"item_id":   videoID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return PlaybackInfo{}, err
	}

	apiURL := WebBaseURL + "/api/video/protected/fallback-play-info?aid=8662"

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 250 * time.Millisecond):
			case <-ctx.Done():
				return PlaybackInfo{}, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
		if err != nil {
			return PlaybackInfo{}, err
		}
		req.Header.Set("User-Agent", WebUserAgent)
		req.Header.Set("Referer", WebBaseURL+"/")
		req.Header.Set("Origin", WebBaseURL)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Sec-Fetch-Dest", "empty")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Site", "same-origin")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
			continue
		}

		var res struct {
			MainURL   string `json:"main_url"`
			URLExpire int64  `json:"url_expire"`
			BaseResp  struct {
				StatusCode    int    `json:"StatusCode"`
				StatusMessage string `json:"StatusMessage"`
			} `json:"base_resp"`
		}
		if err := json.Unmarshal(respBody, &res); err != nil {
			lastErr = err
			continue
		}

		mainURL := strings.TrimSpace(res.MainURL)
		if res.BaseResp.StatusCode != 0 || mainURL == "" {
			lastErr = fmt.Errorf("fallback-play-info code %d: %s", res.BaseResp.StatusCode, res.BaseResp.StatusMessage)
			continue
		}

		// Provide standard quality definition lines (1080p, 720p, 540p, 480p) pointing to this high-quality unencrypted stream
		qualities := []struct {
			def string
			q   int
		}{
			{"1080p", 1080},
			{"720p", 720},
			{"540p", 540},
			{"480p", 480},
		}
		var variants []MediaVariant
		for _, item := range qualities {
			variants = append(variants, MediaVariant{
				URL:        mainURL,
				Definition: item.def,
				Quality:    item.q,
				Codec:      "h264",
				Encrypted:  false,
			})
		}

		return PlaybackInfo{
			SeriesID: seriesID,
			VideoID:  videoID,
			Selected: variants[0],
			Variants: variants,
		}, nil
	}

	return PlaybackInfo{}, lastErr
}

func (c *Client) resolveWebMedia(ctx context.Context, seriesID, videoID string) (PlaybackInfo, error) {
	// 1. Try official unencrypted fallback-play-info API first (high quality, full episode, zero WAF risk)
	if info, err := c.fetchWebProtectedFallbackPlayInfo(ctx, seriesID, videoID); err == nil && len(info.Variants) > 0 {
		return info, nil
	}

	// 2. Try router page data
	pageURL := fmt.Sprintf("%s/player/%s/%s", WebBaseURL, url.PathEscape(seriesID), url.PathEscape(videoID))
	body, err := c.FetchWebText(ctx, pageURL, WebBaseURL+"/")
	if err == nil {
		data := parseRouterData(body)
		page := routerLoaderMap(data, "player_", "player_page")
		info, _ := page["video_player_info"].(map[string]any)
		urls := extractMediaURLs(info)
		if len(urls) > 0 {
			duration, _ := strconv.ParseFloat(mapString(info, "duration"), 64)
			var variants []MediaVariant
			for _, u := range urls {
				variants = append(variants, MediaVariant{
					URL:        u,
					Definition: "1080p",
					Quality:    1080,
					Encrypted:  false,
				})
			}
			return PlaybackInfo{
				SeriesID: seriesID,
				VideoID:  videoID,
				Duration: duration,
				Selected: variants[0],
				Variants: variants,
			}, nil
		}
	}

	// 3. Fallback: check if official web public preview MP4 is accessible (free episodes)
	publicURL := fmt.Sprintf("%s/video/public/%s/360p.mp4", WebBaseURL, url.PathEscape(videoID))
	req, headErr := http.NewRequestWithContext(ctx, http.MethodHead, publicURL, nil)
	if headErr == nil {
		req.Header.Set("User-Agent", WebUserAgent)
		req.Header.Set("Referer", WebBaseURL+"/")
		if resp, err := c.httpClient.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
				variant := MediaVariant{
					URL:        publicURL,
					Definition: "360p",
					Quality:    360,
					Codec:      "h264",
					Encrypted:  false,
				}
				return PlaybackInfo{
					SeriesID: seriesID,
					VideoID:  videoID,
					Selected: variant,
					Variants: []MediaVariant{variant},
				}, nil
			}
		}
	}

	return PlaybackInfo{}, errors.New("网页该集未提供播放地址")
}

func (c *Client) resolveBackupMedia(ctx context.Context, seriesID, videoID string) (PlaybackInfo, error) {
	ref, _ := json.Marshal(map[string]any{
		"content_type":   1004,
		"series_id":      seriesID,
		"vid":            videoID,
		"video_platform": 3,
	})
	query := url.Values{"id": {base64.StdEncoding.EncodeToString(ref)}}
	body, err := c.FetchWebText(ctx, backupPlaybackAPI+"?"+query.Encode(), WebBaseURL+"/")
	if err != nil {
		return PlaybackInfo{}, err
	}

	decoded, err := decodeBackupPlaybackResponse(body)
	if err != nil {
		return PlaybackInfo{}, err
	}

	var resp struct {
		KeyURLs []struct {
			Name   string `json:"name"`
			URL    string `json:"src"`
			KeyID  string `json:"kid"`
			SpadeA string `json:"spade_a"`
		} `json:"key_urls"`
	}
	if err := json.Unmarshal(decoded, &resp); err != nil {
		return PlaybackInfo{}, errors.New("解析备用接口响应失败")
	}

	var variants []MediaVariant
	for _, opt := range resp.KeyURLs {
		u := strings.TrimSpace(opt.URL)
		if !strings.HasPrefix(u, "http") {
			continue
		}
		k, err := DecodeHongguoContentKey(opt.SpadeA)
		if err != nil {
			continue
		}
		q, _ := strconv.Atoi(qualityNumber.FindString(opt.Name))
		if q == 0 {
			q = 720
		}
		variants = append(variants, MediaVariant{
			URL:        u,
			Definition: opt.Name,
			Quality:    q,
			Encrypted:  true,
			CENCKeyHex: hex.EncodeToString(k),
		})
	}

	if len(variants) == 0 {
		return PlaybackInfo{}, errors.New("备用接口无可用地址")
	}

	sort.SliceStable(variants, func(i, j int) bool {
		return variants[i].Quality > variants[j].Quality
	})

	return PlaybackInfo{
		SeriesID: seriesID,
		VideoID:  videoID,
		Selected: variants[0],
		Variants: variants,
	}, nil
}

func extractMediaURLs(info map[string]any) []string {
	var out []string
	seen := make(map[string]bool)

	var add func(any)
	add = func(v any) {
		switch val := v.(type) {
		case string:
			addr := strings.TrimSpace(val)
			if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
				dec, err := base64.StdEncoding.DecodeString(addr)
				if err == nil {
					addr = strings.TrimSpace(string(dec))
				}
			}
			if (strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://")) && !seen[addr] {
				seen[addr] = true
				out = append(out, addr)
			}
		case []any:
			for _, item := range val {
				add(item)
			}
		}
	}

	for _, k := range []string{"main_url", "backup_url", "backup_url_1", "backup_url_2", "backup_urls", "url_list"} {
		add(info[k])
	}
	return out
}

func decodeBackupPlaybackResponse(body string) ([]byte, error) {
	text := strings.TrimSpace(body)
	if !strings.HasPrefix(text, "v2.") {
		return []byte(text), nil
	}
	parts := strings.SplitN(text, ".", 3)
	if len(parts) != 3 || len(parts[1]) <= 4 {
		return nil, errors.New("invalid v2 header")
	}
	encoded, err := hex.DecodeString(parts[1][4:])
	if err != nil || len(encoded) < 32 {
		return nil, errors.New("invalid key hex")
	}
	mask := [...]byte{104, 64, 70, 166, 190, 168, 143, 130, 225, 254, 251, 217, 196, 34, 45, 60, 29, 20, 103, 105}
	material := make([]byte, len(encoded))
	for index, current := range encoded {
		previous := byte(109)
		if index > 0 {
			previous = encoded[index-1]
		}
		slot := index % len(mask)
		salt := mask[slot] ^ byte(90+13*slot) ^ 85
		shifted := byte(int(current) + 215 - 11*index)
		material[index] = previous ^ salt ^ bits.RotateLeft8(shifted, 3)
	}

	rawCipher, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		rawCipher, err = base64.RawStdEncoding.DecodeString(parts[2])
		if err != nil {
			return nil, err
		}
	}
	block, err := aes.NewCipher(material[:16])
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(rawCipher))
	cipher.NewCBCDecrypter(block, material[16:32]).CryptBlocks(plain, rawCipher)
	pad := int(plain[len(plain)-1])
	if pad <= 0 || pad > 16 || pad > len(plain) {
		return plain, nil
	}
	return plain[:len(plain)-pad], nil
}
