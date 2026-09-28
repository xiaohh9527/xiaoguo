package hongguo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var RankingBoards = []RankingBoard{
	{ID: "hongguo-hot", Name: "热播榜", Description: "全站综合热度短剧排行", Path: "hot-drama"},
	{ID: "hongguo-real", Name: "真人剧榜", Description: "热门真人实拍短剧排行", Path: "hot-real-drama"},
	{ID: "hongguo-comic", Name: "漫剧榜", Description: "热门漫剧动态漫画排行", Path: "hot-comic-drama"},
	{ID: "hongguo-ai", Name: "AI剧榜", Description: "精选 AI 创作短剧排行", Path: "hot-ai-drama"},
}

var rankingJSONLD = regexp.MustCompile(`(?s)<script[^>]+type="application/ld\+json"[^>]*>(.*?)</script>`)
var rankingMetricRe = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?[万亿]?热度$`)

func (c *Client) FetchRankings(ctx context.Context, boardID string, page int) (RankingPage, error) {
	if page < 1 {
		page = 1
	}

	var targetBoard *RankingBoard
	for _, b := range RankingBoards {
		if b.ID == boardID {
			targetBoard = &b
			break
		}
	}
	if targetBoard == nil {
		targetBoard = &RankingBoards[0]
	}

	address := fmt.Sprintf("%s/rank/%s", WebBaseURL, targetBoard.Path)
	if page > 1 {
		address += fmt.Sprintf("?page=%d", page)
	}

	body, err := c.FetchWebText(ctx, address, WebBaseURL+"/")
	if err != nil {
		return RankingPage{}, err
	}

	// First try JSON-LD
	if items, ok := parseRankingJSONLD(body, *targetBoard, page); ok && len(items) > 0 {
		return RankingPage{
			Items:      items,
			TotalPages: page,
			HasMore:    len(items) >= 20,
		}, nil
	}

	// Next try HTML DOM parsing
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return RankingPage{}, err
	}

	data := parseRouterData(body)
	loader := routerLoaderMap(data, "rank_page", "rank_$")

	return parseRankingHTML(doc, *targetBoard, page, loader)
}

func parseRankingJSONLD(body string, board RankingBoard, page int) ([]RankingItem, bool) {
	for _, match := range rankingJSONLD.FindAllStringSubmatch(body, -1) {
		var val struct {
			Type  string `json:"@type"`
			URL   string `json:"url"`
			Items []struct {
				Position int    `json:"position"`
				Name     string `json:"name"`
				URL      string `json:"url"`
			} `json:"itemListElement"`
		}
		if json.Unmarshal([]byte(match[1]), &val) != nil || val.Type != "ItemList" {
			continue
		}
		if len(val.Items) == 0 {
			continue
		}

		items := make([]RankingItem, 0, len(val.Items))
		for _, row := range val.Items {
			id := parseRankingDramaID(row.URL)
			title := strings.TrimSpace(row.Name)
			items = append(items, RankingItem{
				Rank: row.Position,
				Drama: Drama{
					ID:          "hongguo:" + id,
					SourceID:    id,
					Title:       title,
					Name:        title,
					ChannelName: "红果短剧",
				},
				Metric: "",
			})
		}
		return items, true
	}
	return nil, false
}

func parseRankingDramaID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	id := u.Query().Get("series_id")
	if numericID.MatchString(id) {
		return id
	}
	return ""
}

func parseRankingHTML(doc *html.Node, board RankingBoard, page int, loader map[string]any) (RankingPage, error) {
	var result RankingPage
	result.UpdatedText = mapString(loader, "updatedText")

	var articles []*html.Node
	var findArticles func(*html.Node)
	findArticles = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "article" {
			for _, attr := range n.Attr {
				if attr.Key == "aria-labelledby" && strings.HasPrefix(attr.Val, "rank-title-") {
					articles = append(articles, n)
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findArticles(c)
		}
	}
	findArticles(doc)

	seen := make(map[string]bool)
	for _, art := range articles {
		item, ok := parseArticleItem(art)
		if ok && !seen[item.Drama.SourceID] {
			seen[item.Drama.SourceID] = true
			result.Items = append(result.Items, item)
		}
	}

	result.HasMore = len(result.Items) >= 20
	result.TotalPages = page
	if result.HasMore {
		result.TotalPages = page + 1
	}

	return result, nil
}

func parseArticleItem(art *html.Node) (RankingItem, bool) {
	label := ""
	for _, attr := range art.Attr {
		if attr.Key == "aria-labelledby" {
			label = attr.Val
			break
		}
	}
	id := strings.TrimPrefix(label, "rank-title-")
	if !numericID.MatchString(id) {
		return RankingItem{}, false
	}

	var title string
	var rank int
	var cover string
	var metric string
	var desc string
	var score string
	var tags []string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if (n.Data == "h2" || n.Data == "h3") && title == "" {
				for _, attr := range n.Attr {
					if attr.Key == "id" && attr.Val == label {
						title = nodeText(n)
						break
					}
				}
			}
			if n.Data == "img" && cover == "" {
				for _, attr := range n.Attr {
					if attr.Key == "src" {
						cover = attr.Val
						break
					}
				}
			}
			if n.Data == "a" && rank == 0 {
				txt := nodeText(n)
				if val, err := strconv.Atoi(txt); err == nil && val > 0 {
					rank = val
				}
			}
		}
		if n.Type == html.TextNode {
			t := strings.TrimSpace(n.Data)
			if rankingMetricRe.MatchString(t) {
				metric = t
			}
			if strings.HasPrefix(t, "评分") {
				score = strings.TrimSpace(strings.TrimPrefix(t, "评分"))
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(art)

	if title == "" {
		title = id
	}

	return RankingItem{
		Rank: rank,
		Drama: Drama{
			ID:            "hongguo:" + id,
			SourceID:      id,
			Title:         title,
			Name:          title,
			Cover:         cover,
			Desc:          desc,
			Score:         score,
			Heat:          metric,
			Tags:          tags,
			ChannelName:   "红果短剧",
			ReleaseStatus: "finished",
		},
		Metric: metric,
	}, true
}

func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(curr *html.Node) {
		if curr.Type == html.TextNode {
			sb.WriteString(curr.Data)
		}
		for c := curr.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(sb.String())
}
