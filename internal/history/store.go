package history

import (
	"sync"
	"time"
)

type Store struct {
	mu       sync.RWMutex
	capacity int
	series   map[string]*Ring
}

func NewStore(capacity int) *Store {
	return &Store{
		capacity: capacity,
		series:   make(map[string]*Ring),
	}
}

func (s *Store) Append(key string, ts time.Time, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ring, ok := s.series[key]
	if !ok {
		ring = NewRing(s.capacity)
		s.series[key] = ring
	}

	ring.Push(Point{Time: ts, Value: value})
}

func (s *Store) Series(key string) []Point {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ring, ok := s.series[key]
	if !ok {
		return nil
	}

	return ring.Values()
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.series)
}
