package services

import (
	"encoding/json"
	"errors"
	"log"
	"mailtrackerProject/models"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type EntryEnvelope struct {
	Data      EntryData `json:"data"`
	CreatedAt time.Time `json:"created_at"`
}

type Encrypt struct {
	Method   *string `json:"method"`
	Password *string `json:"password"`
}
type LookupLimit struct {
	Type            *string `json:"type"`
	AvailableAfter  *string `json:"availableAfter"`
	AvailableBefore *string `json:"availableBefore"`
}

type EntryData struct {
	Images         *[]string    `json:"images,omitempty"`         // 可选数组
	OriginLocation *string      `json:"originLocation,omitempty"` // 可选字符串
	PostDate       *string      `json:"postDate,omitempty"`       // 用 *string 保存原始日期，再转 time.Time
	LookupLimit    *LookupLimit `json:"lookupLimit,omitempty"`
	Encrypt        *Encrypt     `json:"encrypt,omitempty"`
	RecipientName  *string      `json:"recipientName,omitempty"`
	Remarks        *string      `json:"remarks,omitempty"`
}

type EntriesService struct {
	dataDir string
	keys    *KeysService
	mu      sync.RWMutex // protects write operations per entry file (coarse-grained)
}

func NewEntriesService(dataDir string, ks *KeysService) *EntriesService {
	return &EntriesService{dataDir: dataDir, keys: ks}
}

func (s *EntriesService) entryDir(key string) string { return filepath.Join(s.dataDir, "entries", key) }
func (s *EntriesService) entryPath(key string) string {
	return filepath.Join(s.entryDir(key), "entry.json")
}
func (s *EntriesService) historyPath(key string) string {
	return filepath.Join(s.entryDir(key), "history.ndjson")
}

func (s *EntriesService) SaveData(key string, data EntryData) error {
	if !models.ValidKey(key) {
		return errors.New("invalid key")
	}
	if _, ok := s.keys.Get(key); !ok {
		return errors.New("key not found")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// prepare envelope
	now := time.Now()

	env := EntryEnvelope{Data: data, CreatedAt: now}

	if err := os.MkdirAll(s.entryDir(key), 0o755); err != nil {
		return err
	}

	// If exists, keep CreatedAt
	if b, err := os.ReadFile(s.entryPath(key)); err == nil && len(b) > 0 {
		var old EntryEnvelope
		if json.Unmarshal(b, &old) == nil && !old.CreatedAt.IsZero() {
			env.CreatedAt = old.CreatedAt
		}
	}

	b, _ := json.MarshalIndent(&env, "", "  ")
	return os.WriteFile(s.entryPath(key), b, 0o644)
}

func (s *EntriesService) LoadData(key string) (*EntryEnvelope, error) {
	p := s.entryPath(key)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var env EntryEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		log.Printf("error unmarshalling %s", b)
		return nil, err
	}
	return &env, nil
}

// HasData returns true if entry.json exists for the key
func (s *EntriesService) HasData(key string) bool {
	if !models.ValidKey(key) {
		return false
	}
	_, err := os.Stat(s.entryPath(key))
	return err == nil
}
