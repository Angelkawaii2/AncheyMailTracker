package services

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mileusna/useragent"
)

type HistoryRecord struct {
	Time  time.Time           `json:"time"`
	UA    string              `json:"ua"`
	IP    string              `json:"ip"`
	UAObj useragent.UserAgent `json:"-"`
	IPObj *IPInfo             `json:"-"`
}

func (s *EntriesService) RecorduaNewlinejson(key string, rec HistoryRecord) error {
	hp := s.historyPath(key)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(hp), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(hp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	return enc.Encode(rec) // 每条一行
}
func (s *EntriesService) ReadUARecords(key string) ([]HistoryRecord, error) {
	hp := s.historyPath(key)

	// 文件不存在时返回 nil
	f, err := os.Open(hp)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var records []HistoryRecord
	dec := json.NewDecoder(f)

	for {
		var rec HistoryRecord
		if err := dec.Decode(&rec); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		records = append(records, rec)
	}
	// 倒序排列
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}
	return records, nil
}
