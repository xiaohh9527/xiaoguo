package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"xiaoguo/server/internal/api"
	"xiaoguo/server/internal/hongguo"
	"xiaoguo/server/internal/storage"
)

func main() {
	defaultPort := 8080
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
			defaultPort = p
		}
	}
	defaultData := "./data"
	if envData := os.Getenv("DATA_DIR"); envData != "" {
		defaultData = envData
	}
	defaultWeb := "../web/dist"
	if envWeb := os.Getenv("WEB_DIR"); envWeb != "" {
		defaultWeb = envWeb
	}

	port := flag.Int("port", defaultPort, "server listening port")
	dataDir := flag.String("data", defaultData, "data directory for user data and stream cache")
	webDir := flag.String("web", defaultWeb, "frontend static assets directory")
	flag.Parse()

	absDataDir, err := filepath.Abs(*dataDir)
	if err != nil {
		absDataDir = *dataDir
	}
	_ = os.MkdirAll(absDataDir, 0755)

	cacheDir := filepath.Join(absDataDir, "cache")
	_ = os.MkdirAll(cacheDir, 0755)

	client := hongguo.NewClient()
	stream := hongguo.NewStreamManager(cacheDir, client)
	store, err := storage.NewStorage(absDataDir)
	if err != nil {
		log.Fatalf("failed to initialize storage: %v", err)
	}

	server := api.NewServer(client, stream, store)
	handler := api.NewRouter(server, *webDir)

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 300 * time.Second, // Allow large video streams
		IdleTimeout:  120 * time.Second,
	}

	fmt.Printf("====================================================\n")
	fmt.Printf("  小果短剧 (Xiaoguo) Web 后端服务已启动\n")
	fmt.Printf("  监听地址: http://127.0.0.1:%d\n", *port)
	fmt.Printf("  数据目录: %s\n", absDataDir)
	fmt.Printf("  静态目录: %s\n", *webDir)
	fmt.Printf("====================================================\n")

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server listen failed: %v", err)
	}
}

