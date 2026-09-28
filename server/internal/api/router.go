package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func NewRouter(s *Server, webDistDir string) http.Handler {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/genres", s.HandleGenres)
	mux.HandleFunc("GET /api/catalog", s.HandleCatalog)
	mux.HandleFunc("GET /api/ranking-boards", s.HandleRankingBoards)
	mux.HandleFunc("GET /api/rankings", s.HandleRankings)
	mux.HandleFunc("GET /api/detail", s.HandleDetail)
	mux.HandleFunc("GET /api/play", s.HandlePlay)
	mux.HandleFunc("GET /api/stream", s.HandleStream)
	mux.HandleFunc("GET /play", s.HandleStream)
	mux.HandleFunc("GET /stream", s.HandleStream)
	mux.HandleFunc("GET /api/danmaku", s.HandleDanmaku)
	mux.HandleFunc("GET /api/search", s.HandleSearch)
	mux.HandleFunc("GET /api/search/suggestions", s.HandleSuggestions)
	mux.HandleFunc("GET /api/proxy/image", s.HandleProxyImage)

	mux.HandleFunc("GET /api/favorites", s.HandleGetFavorites)
	mux.HandleFunc("POST /api/favorites", s.HandleAddFavorite)
	mux.HandleFunc("DELETE /api/favorites", s.HandleDeleteFavorite)

	mux.HandleFunc("GET /api/history", s.HandleGetHistory)
	mux.HandleFunc("POST /api/history", s.HandleSaveHistory)
	mux.HandleFunc("DELETE /api/history", s.HandleClearHistory)

	// Static web assets or fallback to index.html for SPA
	if webDistDir != "" {
		if fi, err := os.Stat(webDistDir); err == nil && fi.IsDir() {
			fileServer := http.FileServer(http.Dir(webDistDir))
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/") {
					http.NotFound(w, r)
					return
				}

				// Check if file exists directly
				path := filepath.Join(webDistDir, filepath.Clean(r.URL.Path))
				if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
					fileServer.ServeHTTP(w, r)
					return
				}

				// SPA fallback to index.html
				indexPath := filepath.Join(webDistDir, "index.html")
				if _, err := os.Stat(indexPath); err == nil {
					http.ServeFile(w, r, indexPath)
					return
				}

				fileServer.ServeHTTP(w, r)
			})
		}
	}

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Range")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

