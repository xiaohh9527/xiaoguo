package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"xiaoguo/server/internal/hongguo"
	"xiaoguo/server/internal/storage"
)

type Server struct {
	client  *hongguo.Client
	stream  *hongguo.StreamManager
	storage *storage.Storage
}

func NewServer(client *hongguo.Client, stream *hongguo.StreamManager, store *storage.Storage) *Server {
	return &Server{
		client:  client,
		stream:  stream,
		storage: store,
	}
}

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(Response{
		Code:    code,
		Message: msg,
	})
}

// GET /api/genres
func (s *Server) HandleGenres(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, hongguo.Genres)
}

// GET /api/catalog?genre=short_play&offset=0&session_id=...
func (s *Server) HandleCatalog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	genre := q.Get("genre")
	offset, _ := strconv.Atoi(q.Get("offset"))
	sessionID := q.Get("session_id")
	var seen []string
	if rawSeen := q.Get("seen"); rawSeen != "" {
		seen = strings.Split(rawSeen, ",")
	}

	page, err := s.client.FetchRecommendations(r.Context(), hongguo.RecommendationQuery{
		Genre:     genre,
		Offset:    offset,
		SessionID: sessionID,
		Seen:      seen,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, page)
}

// GET /api/ranking-boards
func (s *Server) HandleRankingBoards(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, hongguo.RankingBoards)
}

// GET /api/rankings?board=hongguo-hot&page=1
func (s *Server) HandleRankings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	board := q.Get("board")
	if board == "" {
		board = "hongguo-hot"
	}
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}

	resp, err := s.client.FetchRankings(r.Context(), board, page)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, resp)
}

// GET /api/detail?id=7611094547900156990
func (s *Server) HandleDetail(w http.ResponseWriter, r *http.Request) {
	seriesID := r.URL.Query().Get("id")
	if seriesID == "" {
		seriesID = r.URL.Query().Get("series_id")
	}
	if seriesID == "" {
		jsonError(w, http.StatusBadRequest, "缺少剧集 ID")
		return
	}

	drama, chapters, err := s.client.FetchDetail(r.Context(), seriesID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	isFav := s.storage.IsFavorite(drama.SourceID)

	jsonOK(w, map[string]any{
		"drama":      drama,
		"chapters":   chapters,
		"isFavorite": isFav,
	})
}

// GET /api/play?series_id=...&video_id=...
func (s *Server) HandlePlay(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	seriesID := q.Get("series_id")
	videoID := q.Get("video_id")

	if seriesID == "" || videoID == "" {
		jsonError(w, http.StatusBadRequest, "缺少参数")
		return
	}

	info, err := s.client.ResolvePlayback(r.Context(), seriesID, videoID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Attach local streaming proxy URL
	info.StreamURL = "/api/stream?series_id=" + seriesID + "&video_id=" + videoID + "&quality=" + strconv.Itoa(info.Selected.Quality)

	jsonOK(w, info)
}

// GET /api/stream?series_id=...&video_id=...&quality=... or /play?vid=...
func (s *Server) HandleStream(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	seriesID := q.Get("series_id")
	videoID := q.Get("video_id")
	if videoID == "" {
		videoID = q.Get("vid")
	}
	if videoID == "" {
		videoID = q.Get("id")
	}
	if strings.Contains(videoID, "*") {
		parts := strings.SplitN(videoID, "*", 2)
		if seriesID == "" {
			seriesID = parts[0]
		}
		videoID = parts[1]
	} else if strings.Contains(videoID, "|") {
		parts := strings.SplitN(videoID, "|", 2)
		if seriesID == "" {
			seriesID = parts[0]
		}
		videoID = parts[1]
	}
	if seriesID == "" {
		seriesID = videoID
	}
	quality, _ := strconv.Atoi(q.Get("quality"))

	if videoID == "" {
		http.Error(w, "missing video_id or vid", http.StatusBadRequest)
		return
	}

	info, err := s.client.ResolvePlayback(r.Context(), seriesID, videoID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Match quality variant
	selected := info.Selected
	if quality > 0 {
		for _, v := range info.Variants {
			if v.Quality == quality {
				selected = v
				break
			}
		}
	}

	filePath, err := s.stream.GetDecryptedVideo(r.Context(), seriesID, videoID, selected)
	if err != nil {
		http.Error(w, "stream preparation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", "video/mp4")
	if q.Get("download") == "1" {
		fileName := seriesID + "_" + videoID + ".mp4"
		w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	}

	http.ServeFile(w, r, filePath)
}

// GET /api/danmaku?series_id=...&video_id=...&start_ms=0&duration_ms=600000
func (s *Server) HandleDanmaku(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	seriesID := q.Get("series_id")
	videoID := q.Get("video_id")
	startMS, _ := strconv.ParseInt(q.Get("start_ms"), 10, 64)
	durationMS, _ := strconv.ParseInt(q.Get("duration_ms"), 10, 64)

	if seriesID == "" || videoID == "" {
		jsonError(w, http.StatusBadRequest, "缺少参数")
		return
	}

	page, err := s.client.FetchDanmaku(r.Context(), seriesID, videoID, startMS, durationMS)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, page)
}

// GET /api/search?keyword=...
func (s *Server) HandleSearch(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	if keyword == "" {
		keyword = r.URL.Query().Get("key")
	}
	if keyword == "" {
		keyword = r.URL.Query().Get("wd")
	}
	if keyword == "" {
		jsonError(w, http.StatusBadRequest, "请输入搜索关键词")
		return
	}

	entry, err := s.client.Search(r.Context(), keyword)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, entry)
}

// GET /api/search/suggestions?query=...
func (s *Server) HandleSuggestions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	items, err := s.client.FetchSuggestions(r.Context(), query)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, items)
}

// GET /api/proxy/image?url=...
func (s *Server) HandleProxyImage(w http.ResponseWriter, r *http.Request) {
	imgURL := r.URL.Query().Get("url")
	if imgURL == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}

	if err := s.stream.ProxyImage(r.Context(), imgURL, w, r); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
}

// GET /api/favorites
func (s *Server) HandleGetFavorites(w http.ResponseWriter, r *http.Request) {
	items := s.storage.GetFavorites()
	jsonOK(w, items)
}

// POST /api/favorites
func (s *Server) HandleAddFavorite(w http.ResponseWriter, r *http.Request) {
	var item storage.FavoriteItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		jsonError(w, http.StatusBadRequest, "无效参数")
		return
	}

	if item.SourceID == "" {
		jsonError(w, http.StatusBadRequest, "缺少剧集 ID")
		return
	}

	_ = s.storage.AddFavorite(item)
	jsonOK(w, true)
}

// DELETE /api/favorites?id=...
func (s *Server) HandleDeleteFavorite(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		jsonError(w, http.StatusBadRequest, "缺少剧集 ID")
		return
	}

	_ = s.storage.RemoveFavorite(id)
	jsonOK(w, true)
}

// GET /api/history
func (s *Server) HandleGetHistory(w http.ResponseWriter, r *http.Request) {
	items := s.storage.GetHistory()
	jsonOK(w, items)
}

// POST /api/history
func (s *Server) HandleSaveHistory(w http.ResponseWriter, r *http.Request) {
	var item storage.HistoryItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		jsonError(w, http.StatusBadRequest, "无效参数")
		return
	}

	if item.SeriesID == "" {
		jsonError(w, http.StatusBadRequest, "缺少剧集 ID")
		return
	}

	_ = s.storage.SaveHistory(item)
	jsonOK(w, true)
}

// DELETE /api/history
func (s *Server) HandleClearHistory(w http.ResponseWriter, r *http.Request) {
	_ = s.storage.ClearHistory()
	jsonOK(w, true)
}




