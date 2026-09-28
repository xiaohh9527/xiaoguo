package hongguo

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type StreamManager struct {
	cacheDir string
	client   *Client
	mu       sync.Mutex
	inFlight map[string]*streamJob
}

type streamJob struct {
	done chan struct{}
	err  error
	path string
}

func isHEIC(data []byte) bool {
	if len(data) >= 12 && string(data[4:8]) == "ftyp" {
		brand := string(data[8:12])
		switch brand {
		case "heic", "heix", "hevc", "hevx", "mif1", "msf1":
			return true
		}
	}
	return false
}

func NewStreamManager(cacheDir string, client *Client) *StreamManager {
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "hongguo_video_cache")
	}
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.MkdirAll(filepath.Join(cacheDir, "images"), 0755)
	_ = os.MkdirAll(filepath.Join(cacheDir, "videos"), 0755)

	sm := &StreamManager{
		cacheDir: cacheDir,
		client:   client,
		inFlight: make(map[string]*streamJob),
	}

	sm.cleanHEICCache()
	go sm.periodicCleanup()
	return sm
}

func (sm *StreamManager) cleanHEICCache() {
	dir := filepath.Join(sm.cacheDir, "images")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		hdr := make([]byte, 16)
		n, _ := f.Read(hdr)
		f.Close()
		if isHEIC(hdr[:n]) {
			_ = os.Remove(path)
		}
	}
}

func (sm *StreamManager) GetDecryptedVideo(ctx context.Context, seriesID, videoID string, variant MediaVariant) (string, error) {
	cacheFileName := fmt.Sprintf("%s_%s_%d.mp4", seriesID, videoID, variant.Quality)
	cacheFilePath := filepath.Join(sm.cacheDir, "videos", cacheFileName)

	// Check if already cached and valid
	if fi, err := os.Stat(cacheFilePath); err == nil && fi.Size() > 1024 {
		return cacheFilePath, nil
	}

	jobKey := cacheFileName
	sm.mu.Lock()
	if job, ok := sm.inFlight[jobKey]; ok {
		sm.mu.Unlock()
		select {
		case <-job.done:
			return job.path, job.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	job := &streamJob{done: make(chan struct{})}
	sm.inFlight[jobKey] = job
	sm.mu.Unlock()

	defer func() {
		sm.mu.Lock()
		delete(sm.inFlight, jobKey)
		close(job.done)
		sm.mu.Unlock()
	}()

	// Download encrypted MP4
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, variant.URL, nil)
	if err != nil {
		job.err = err
		return "", err
	}
	req.Header.Set("User-Agent", AppUserAgent)
	req.Header.Set("Referer", "https://novel.snssdk.com/")

	resp, err := sm.client.DoHTTP(req)
	if err != nil {
		job.err = err
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		job.err = fmt.Errorf("CDN HTTP %d", resp.StatusCode)
		return "", job.err
	}

	rawBytes, err := io.ReadAll(io.LimitReader(resp.Body, 500<<20)) // Max 500MB
	if err != nil {
		job.err = err
		return "", err
	}

	var finalBytes []byte
	if variant.Encrypted && variant.CENCKeyHex != "" {
		key, err := hex.DecodeString(variant.CENCKeyHex)
		if err != nil || len(key) != 16 {
			job.err = errors.New("无效解密密钥")
			return "", job.err
		}

		decrypted, err := DecryptCENCMP4(rawBytes, key)
		if err != nil {
			job.err = fmt.Errorf("解密失败: %w", err)
			return "", job.err
		}
		finalBytes = decrypted
	} else {
		finalBytes = rawBytes
	}

	tempPath := cacheFilePath + ".tmp"
	if err := os.WriteFile(tempPath, finalBytes, 0644); err != nil {
		job.err = err
		return "", err
	}
	_ = os.Rename(tempPath, cacheFilePath)

	job.path = cacheFilePath
	return cacheFilePath, nil
}

func (sm *StreamManager) ProxyImage(ctx context.Context, imgURL string, w http.ResponseWriter, r *http.Request) error {
	if imgURL == "" {
		return errors.New("empty url")
	}

	h := md5.Sum([]byte(imgURL))
	cacheKey := hex.EncodeToString(h[:])
	cacheFile := filepath.Join(sm.cacheDir, "images", cacheKey)

	if fi, err := os.Stat(cacheFile); err == nil && fi.Size() > 0 {
		f, err := os.Open(cacheFile)
		if err == nil {
			hdr := make([]byte, 512)
			n, _ := f.Read(hdr)
			f.Close()
			if isHEIC(hdr[:n]) {
				_ = os.Remove(cacheFile)
			} else {
				ct := http.DetectContentType(hdr[:n])
				if ct != "" && ct != "application/octet-stream" {
					w.Header().Set("Content-Type", ct)
				} else {
					w.Header().Set("Content-Type", "image/jpeg")
				}
				w.Header().Set("Cache-Control", "public, max-age=864000")
				http.ServeFile(w, r, cacheFile)
				return nil
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imgURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", AppUserAgent)
	req.Header.Set("Accept", "image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	if strings.Contains(imgURL, "byteimg.com") || strings.Contains(imgURL, "fqnovel") || strings.Contains(imgURL, "snssdk.com") {
		req.Header.Set("Referer", "https://novel.snssdk.com/")
	} else {
		req.Header.Set("Referer", "https://hongguoduanju.com/")
	}
	req.Header.Set("Sec-Fetch-Dest", "image")
	req.Header.Set("Sec-Fetch-Mode", "no-cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")

	resp, err := sm.client.DoHTTP(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("image fetch failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return err
	}

	if isHEIC(data) {
		http.Error(w, "unsupported image format", http.StatusUnsupportedMediaType)
		return nil
	}

	_ = os.WriteFile(cacheFile, data, 0644)

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = "image/jpeg"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=864000")
	_, _ = w.Write(data)
	return nil
}

func (sm *StreamManager) periodicCleanup() {
	ticker := time.NewTicker(2 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		sm.cleanOldFiles(filepath.Join(sm.cacheDir, "videos"), 7*24*time.Hour)
		sm.cleanOldFiles(filepath.Join(sm.cacheDir, "images"), 14*24*time.Hour)
	}
}

func (sm *StreamManager) cleanOldFiles(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > maxAge {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
