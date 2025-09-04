package services

import (
	"encoding/json"
	"errors"
	"mailtrackerProject/helper"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type KeyInfo struct {
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
	Comment   string    `json:"comment"`
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

// 原子落盘：写入临时文件后 Rename 覆盖
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

	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	// Rename 在同一分区上是原子的
	return os.Rename(tmp, s.filePath)
}

func (s *KeysService) Generate(n int, length int, comment string) ([]KeyInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]KeyInfo, 0, n)
	newKeys := make([]string, 0, n)

	// 最多尝试次数，避免极端情况下死循环
	const maxAttemptsPerKey = 10_000

	for i := 0; i < n; i++ {
		var k string
		var err error
		ok := false
		for attempt := 0; attempt < maxAttemptsPerKey; attempt++ {
			k, err = helper.RandKey(length)
			if err != nil {
				return nil, err
			}
			if _, exists := s.keys[k]; !exists {
				// 也避免本批次内重复
				dup := false
				for _, nk := range newKeys {
					if nk == k {
						dup = true
						break
					}
				}
				if !dup {
					ok = true
					break
				}
			}
		}
		if !ok {
			return nil, errors.New("failed to generate unique key without collision")
		}

		ki := KeyInfo{Key: k, CreatedAt: time.Now(), Comment: comment}
		// 先写入内存；若 flush 失败我们会回滚
		s.keys[k] = ki
		newKeys = append(newKeys, k)
		out = append(out, ki)
	}

	// 持久化；失败则回滚新加的键，确保状态一致
	if err := s.flushLocked(); err != nil {
		for _, k := range newKeys {
			delete(s.keys, k)
		}
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
		//ki.HasData = entriesSrvc.HasData(ki.Key)
		list = append(list, ki)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
	return list
}
