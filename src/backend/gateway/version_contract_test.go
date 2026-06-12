package main

import (
	"context"
	"fmt"
	"sync"
)

type memoryVersionStore struct {
	mu   sync.Mutex
	data map[string]clientVersionRecord
	gets int
	sets int
}

func newMemoryVersionStore(seed map[string]clientVersionRecord) *memoryVersionStore {
	data := make(map[string]clientVersionRecord, len(seed))
	for platform, record := range seed {
		data[platform] = record
	}
	return &memoryVersionStore{data: data}
}

func (s *memoryVersionStore) Get(_ context.Context, platform string) (clientVersionRecord, error) {
	s.mu.Lock()
	s.gets++
	s.mu.Unlock()
	record, ok := s.data[platform]
	if !ok {
		return clientVersionRecord{}, fmt.Errorf("%w: %s", errUnknownPlatform, platform)
	}
	return record, nil
}

func (s *memoryVersionStore) Set(_ context.Context, record clientVersionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sets++
	s.data[record.Platform] = record
	return nil
}

func (s *memoryVersionStore) getCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gets
}
