package hongguo

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type Drama struct {
	ID            string   `json:"id"`
	SourceID      string   `json:"sourceId"`
	Title         string   `json:"title"`
	Name          string   `json:"name"`
	Desc          string   `json:"desc"`
	Cover         string   `json:"cover"`
	TotalEpisodes int      `json:"totalEpisodes"`
	EpisodeCount  string   `json:"episodeCount"`
	CategoryName  string   `json:"categoryName"`
	ChannelName   string   `json:"channelName"`
	Remark        string   `json:"remark,omitempty"`
	Score         string   `json:"score,omitempty"`
	Views         string   `json:"views,omitempty"`
	Heat          string   `json:"heat,omitempty"`
	OnlineDate    string   `json:"onlineDate,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	ReleaseStatus string   `json:"releaseStatus,omitempty"`
}

func (d Drama) DisplayTitle() string {
	if strings.TrimSpace(d.Title) != "" {
		return d.Title
	}
	if strings.TrimSpace(d.Name) != "" {
		return d.Name
	}
	return "短剧"
}

type Chapter struct {
	ID             string `json:"id"`
	SeriesID       string `json:"seriesId"`
	VideoID        string `json:"videoId"`
	Index          int    `json:"index"`
	Title          string `json:"title"`
	VideoURL       string `json:"videoUrl"`
	CurrentEpisode string `json:"currentEpisode"`
}

type RankingBoard struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

type RankingItem struct {
	Rank   int    `json:"rank"`
	Drama  Drama  `json:"drama"`
	Metric string `json:"metric"`
}

type RankingPage struct {
	Items       []RankingItem `json:"items"`
	TotalPages  int           `json:"totalPages"`
	HasMore     bool          `json:"hasMore"`
	UpdatedText string        `json:"updatedText"`
}

type SearchEntry struct {
	Dramas    []Drama   `json:"dramas"`
	Total     int       `json:"total"`
	Limited   bool      `json:"limited"`
	Warning   string    `json:"warning,omitempty"`
	ExpiresAt time.Time `json:"-"`
}

type SearchSuggestion struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type DanmakuItem struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	TimeMS int64  `json:"timeMs"`
}

type DanmakuPage struct {
	EpisodeID string        `json:"episodeId"`
	Items     []DanmakuItem `json:"items"`
	StartMS   int64         `json:"startMs"`
	NextMS    int64         `json:"nextMs"`
	Total     int64         `json:"total"`
}

type MediaVariant struct {
	URL        string `json:"url"`
	Codec      string `json:"codec"`
	Definition string `json:"definition"`
	Quality    int    `json:"quality"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Size       int64  `json:"size"`
	Encrypted  bool   `json:"encrypted"`
	CENCKeyHex string `json:"cencKeyHex,omitempty"`
}

type PlaybackInfo struct {
	SeriesID string         `json:"seriesId"`
	VideoID  string         `json:"videoId"`
	Title    string         `json:"title"`
	Duration float64        `json:"duration"`
	Selected MediaVariant   `json:"selected"`
	Variants []MediaVariant `json:"variants"`
	StreamURL string        `json:"streamUrl"` // Local clean decrypted stream proxy URL
}

type RecommendationQuery struct {
	Genre     string   `json:"genre"`
	Offset    int      `json:"offset"`
	SessionID string   `json:"sessionId"`
	Seen      []string `json:"seen"`
}

type RecommendationPage struct {
	Dramas     []Drama `json:"data"`
	NextOffset int     `json:"nextOffset"`
	SessionID  string  `json:"sessionId"`
	HasMore    bool    `json:"hasMore"`
}

func mapString(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch val := v.(type) {
			case string:
				return strings.TrimSpace(val)
			case json.Number:
				return val.String()
			case float64:
				return strconv.FormatFloat(val, 'f', -1, 64)
			case int:
				return strconv.Itoa(val)
			case int64:
				return strconv.FormatInt(val, 10)
			default:
				s := strings.TrimSpace(strings.Trim(string(mustJSON(val)), `"`))
				if s != "" && s != "null" {
					return s
				}
			}
		}
	}
	return ""
}

func nestedMap(v any, keys ...string) map[string]any {
	cur, _ := v.(map[string]any)
	for _, key := range keys {
		if cur == nil {
			return nil
		}
		cur, _ = cur[key].(map[string]any)
	}
	return cur
}

func anyList(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case map[string]any:
		for _, key := range []string{"list", "items", "data"} {
			if out := anyList(x[key]); len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func mapStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return nil
	}
	var out []string
	switch list := raw.(type) {
	case []any:
		for _, item := range list {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	case []string:
		out = append(out, list...)
	case string:
		if strings.TrimSpace(list) != "" {
			out = append(out, strings.TrimSpace(list))
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
