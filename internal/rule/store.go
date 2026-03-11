package rule

import (
	"sync"
	"time"
)

type Entry struct {
	Host string
	Port int
}

type RuleSet struct {
	Entries   []Entry
	UpdatedAt time.Time
}

type Store struct {
	mu    sync.RWMutex
	rules map[string]*RuleSet
}

func NewStore() *Store {
	return &Store{rules: make(map[string]*RuleSet)}
}

func (s *Store) Get(sourceIP string) (*RuleSet, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rs, ok := s.rules[sourceIP]
	if !ok {
		return nil, false
	}
	entries := make([]Entry, len(rs.Entries))
	copy(entries, rs.Entries)
	return &RuleSet{Entries: entries, UpdatedAt: rs.UpdatedAt}, true
}

func (s *Store) Set(sourceIP string, entries []Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]Entry, len(entries))
	copy(copied, entries)
	s.rules[sourceIP] = &RuleSet{
		Entries:   copied,
		UpdatedAt: time.Now().UTC(),
	}
}

func (s *Store) Delete(sourceIP string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.rules[sourceIP]
	if ok {
		delete(s.rules, sourceIP)
	}
	return ok
}

func (s *Store) DeleteAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules = make(map[string]*RuleSet)
}

func (s *Store) All() map[string]*RuleSet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := make(map[string]*RuleSet, len(s.rules))
	for k, v := range s.rules {
		entries := make([]Entry, len(v.Entries))
		copy(entries, v.Entries)
		snap[k] = &RuleSet{Entries: entries, UpdatedAt: v.UpdatedAt}
	}
	return snap
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.rules)
}
