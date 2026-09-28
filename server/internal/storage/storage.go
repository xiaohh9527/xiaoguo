package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type FavoriteItem struct {
	ID        string    `json:"id"`
	SourceID  string    `json:"sourceId"`
	Title     string    `json:"title"`
	Cover     string    `json:"cover"`
	Episodes  int       `json:"episodes"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"createdAt"`
}

type HistoryItem struct {
	SeriesID     string    `json:"seriesId"`
	SeriesTitle  string    `json:"seriesTitle"`
	Cover        string    `json:"cover"`
	EpisodeIndex int       `json:"episodeIndex"`
	EpisodeTitle string    `json:"episodeTitle"`
	PositionSec  float64   `json:"positionSec"`
	DurationSec  float64   `json:"durationSec"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type UserData struct {
	Favorites []FavoriteItem `json:"favorites"`
	History   []HistoryItem  `json:"history"`
}

type Storage struct {
	mu       sync.RWMutex
	filePath string
	data     UserData
}

func NewStorage(dir string) (*Storage, error) {
	if dir == "" {
		dir = "data"
	}
	_ = os.MkdirAll(dir, 0755)
	file := filepath.Join(dir, "user_data.json")

	s := &Storage{
		filePath: file,
		data: UserData{
			Favorites: []FavoriteItem{},
			History:   []HistoryItem{},
		},
	}

	s.load()
	return s, nil
}

func (s *Storage) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.filePath)
	if err == nil {
		_ = json.Unmarshal(b, &s.data)
	}
	if s.data.Favorites == nil {
		s.data.Favorites = []FavoriteItem{}
	}
	if s.data.History == nil {
		s.data.History = []HistoryItem{}
	}
}

func (s *Storage) saveLocked() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
}

func (s *Storage) GetFavorites() []FavoriteItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.data.Favorites) == 0 {
		return []FavoriteItem{}
	}
	out := append([]FavoriteItem(nil), s.data.Favorites...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *Storage) AddFavorite(item FavoriteItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.data.Favorites {
		if existing.SourceID == item.SourceID {
			item.CreatedAt = existing.CreatedAt
			s.data.Favorites[i] = item
			return s.saveLocked()
		}
	}

	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	s.data.Favorites = append(s.data.Favorites, item)
	return s.saveLocked()
}

func (s *Storage) RemoveFavorite(sourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var updated []FavoriteItem
	for _, it := range s.data.Favorites {
		if it.SourceID != sourceID {
			updated = append(updated, it)
		}
	}
	s.data.Favorites = updated
	return s.saveLocked()
}

func (s *Storage) IsFavorite(sourceID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, it := range s.data.Favorites {
		if it.SourceID == sourceID {
			return true
		}
	}
	return false
}

func (s *Storage) GetHistory() []HistoryItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.data.History) == 0 {
		return []HistoryItem{}
	}
	out := append([]HistoryItem(nil), s.data.History...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func (s *Storage) SaveHistory(item HistoryItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item.UpdatedAt = time.Now()
	found := false
	for i, existing := range s.data.History {
		if existing.SeriesID == item.SeriesID {
			s.data.History[i] = item
			found = true
			break
		}
	}
	if !found {
		s.data.History = append(s.data.History, item)
	}

	// Keep max 200 history items
	if len(s.data.History) > 200 {
		sort.Slice(s.data.History, func(i, j int) bool {
			return s.data.History[i].UpdatedAt.After(s.data.History[j].UpdatedAt)
		})
		s.data.History = s.data.History[:200]
	}

	return s.saveLocked()
}

func (s *Storage) ClearHistory() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.History = []HistoryItem{}
	return s.saveLocked()
}
