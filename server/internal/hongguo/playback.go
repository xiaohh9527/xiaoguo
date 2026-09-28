package hongguo

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/bits"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const backupPlaybackAPI = "https://djapi.999888456.xyz/api/hongguo/play"

func (c *Client) ResolvePlayback(ctx context.Context, seriesID, videoID string) (PlaybackInfo, error) {
	seriesID = strings.TrimPrefix(strings.TrimSpace(seriesID), "hongguo:")
	videoID = strings.TrimPrefix(strings.TrimSpace(videoID), "hongguo-cenc://")

	if !numericID.MatchString(videoID) {
		return PlaybackInfo{}, errors.New("无效的视频 ID")
	}
	if seriesID == "" {
		seriesID = videoID
	}

	// 1. Try App video model
	info, appErr := c.resolveAppMedia(ctx, seriesID, videoID)
	if appErr == nil && len(info.Variants) > 0 {
		return info, nil
	}

	// 2. Try Web player
	info, webErr := c.resolveWebMedia(ctx, seriesID, videoID)
	if webErr == nil && len(info.Variants) > 0 {
		return info, nil
	}

	// 3. Try Backup API
	info, apiErr := c.resolveBackupMedia(ctx, seriesID, videoID)
	if apiErr == nil && len(info.Variants) > 0 {
		return info, nil
	}

	return PlaybackInfo{}, fmt.Errorf("解析播放地址失败: app=%v, web=%v, backup=%w", appErr, webErr, apiErr)
}

func (c *Client) resolveAppMedia(ctx context.Context, seriesID, videoID string) (PlaybackInfo, error) {
	payload := map[string]any{
		"video_id":     videoID,
		"content_type": 1,
		"biz_param": map[string]any{
			"need_all_video_definition": true,
			"video_platform":            3,
		},
	}

	result, err := c.AppRequest(ctx, http.MethodPost, "/novel/player/video_model/v1/", nil, payload, false)
	if err != nil {
		return PlaybackInfo{}, err
	}

	data := nestedMap(result, "data")
	model, _ := data["video_model"].(map[string]any)
	if encoded, ok := data["video_model"].(string); ok {
		var decoded map[string]any
		dec := json.NewDecoder(strings.NewReader(encoded))
		dec.UseNumber()
		if err := dec.Decode(&decoded); err == nil {
			model = decoded
		}
	}

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

	if len(rawVariants) == 0 {
		return PlaybackInfo{}, errors.New("无可用视频流")
	}

	duration, _ := strconv.ParseFloat(mapString(model, "video_duration", "duration"), 64)
	var variants []MediaVariant

	for _, v := range rawVariants {
		meta := nestedMap(v, "video_meta")
		codec := strings.ToLower(firstNonEmpty(mapString(meta, "codec_type"), mapString(v, "codec_type")))
		if codec == "bytevc2" {
			continue // Skip bytevc2 as it's not browser-compatible
		}

		urls := extractMediaURLs(v)
		if len(urls) == 0 {
			continue
		}

		enc := nestedMap(v, "encrypt_info")
		spade := firstNonEmpty(mapString(enc, "spade_a"), mapString(v, "spade_a"))
		isEnc := spade != "" || enc["encrypt"] == true || v["encrypt"] == true || mapString(v, "encryption_method") == "cenc-aes-ctr"

		var keyHex string
		if spade != "" {
			k, err := DecodeHongguoContentKey(spade)
			if err == nil && len(k) == 16 {
				keyHex = hex.EncodeToString(k)
			}
		}

		def := firstNonEmpty(mapString(meta, "definition"), mapString(v, "definition"))
		height, _ := strconv.Atoi(firstNonEmpty(mapString(meta, "vheight"), mapString(v, "vheight")))
		width, _ := strconv.Atoi(firstNonEmpty(mapString(meta, "vwidth"), mapString(v, "vwidth")))
		sz, _ := strconv.ParseInt(firstNonEmpty(mapString(meta, "size"), mapString(v, "size")), 10, 64)

		quality := height
		if q, err := strconv.Atoi(qualityNumber.FindString(def)); err == nil && q > 0 {
			quality = q
		}
		if quality == 0 {
			quality = 720
		}

		for _, u := range urls {
			variants = append(variants, MediaVariant{
				URL:        u,
				Codec:      codec,
				Definition: def,
				Quality:    quality,
				Width:      width,
				Height:     height,
				Size:       sz,
				Encrypted:  isEnc,
				CENCKeyHex: keyHex,
			})
		}
	}

	if len(variants) == 0 {
		return PlaybackInfo{}, errors.New("无兼容的视频编码")
	}

	// Sort by quality descending
	sort.SliceStable(variants, func(i, j int) bool {
		// Prefer H264 if quality is same
		if variants[i].Quality == variants[j].Quality {
			iH264 := variants[i].Codec == "h264" || variants[i].Codec == "avc1"
			jH264 := variants[j].Codec == "h264" || variants[j].Codec == "avc1"
			if iH264 != jH264 {
				return iH264
			}
		}
		return variants[i].Quality > variants[j].Quality
	})

	return PlaybackInfo{
		SeriesID: seriesID,
		VideoID:  videoID,
		Duration: duration,
		Selected: variants[0],
		Variants: variants,
	}, nil
}

func (c *Client) resolveWebMedia(ctx context.Context, seriesID, videoID string) (PlaybackInfo, error) {
	pageURL := fmt.Sprintf("%s/player/%s/%s", WebBaseURL, url.PathEscape(seriesID), url.PathEscape(videoID))
	body, err := c.FetchWebText(ctx, pageURL, WebBaseURL+"/")
	if err != nil {
		return PlaybackInfo{}, err
	}

	data := parseRouterData(body)
	page := routerLoaderMap(data, "player_", "player_page")
	info, _ := page["video_player_info"].(map[string]any)
	urls := extractMediaURLs(info)
	if len(urls) == 0 {
		return PlaybackInfo{}, errors.New("网页该集未提供播放地址")
	}

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

