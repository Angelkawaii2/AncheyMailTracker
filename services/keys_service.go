package services

import (
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type KeyInfo struct {
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
}

type KeysService struct {
	filePath string
	mu       sync.RWMutex
	keys     map[string]KeyInfo
}

func NewKeysService(filePath string) *KeysService {
	return &KeysService{filePath: filePath, keys: map[string]KeyInfo{}}
}

func (s *KeysService) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.flushLocked()
		}
		return err
	}
	var list []KeyInfo
	if len(b) > 0 {
		if err := json.Unmarshal(b, &list); err != nil {
			return err
		}
	}
	s.keys = make(map[string]KeyInfo, len(list))
	for _, ki := range list {
		s.keys[ki.Key] = ki
	}
	return nil
}

func (s *KeysService) flushLocked() error {
	list := make([]KeyInfo, 0, len(s.keys))
	for _, ki := range s.keys {
		list = append(list, ki)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.Before(list[j].CreatedAt) })
	b, _ := json.MarshalIndent(list, "", "  ")
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.filePath, b, 0o644)
}

func (s *KeysService) Generate(n int, length int) ([]KeyInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]KeyInfo, 0, n)
	for i := 0; i < n; i++ {
		var k string
		for {
			k = randKey(length)
			if _, exists := s.keys[k]; !exists {
				break
			}
		}
		ki := KeyInfo{Key: k, CreatedAt: time.Now()}
		s.keys[k] = ki
		out = append(out, ki)
	}
	if err := s.flushLocked(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *KeysService) Get(k string) (KeyInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ki, ok := s.keys[k]
	return ki, ok
}

func (s *KeysService) List() []KeyInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]KeyInfo, 0, len(s.keys))
	for _, ki := range s.keys {
		list = append(list, ki)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.Before(list[j].CreatedAt) })
	return list
}

func randKey(n int) string {
	const al = "ABCDEFHJKLMNPQRSTWXY123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = al[rand.Intn(len(al))]
	}
	return string(b)
}
