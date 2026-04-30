package history

import "time"

type Point struct {
	Time  time.Time
	Value float64
}

type Ring struct {
	points []Point
	head   int
	size   int
}

func NewRing(capacity int) *Ring {
	if capacity <= 0 {
		panic("ring capacity must be positive")
	}

	return &Ring{
		points: make([]Point, capacity),
	}
}

func (r *Ring) Capacity() int {
	return len(r.points)
}

func (r *Ring) Len() int {
	return r.size
}

func (r *Ring) Push(p Point) {
	if len(r.points) == 0 {
		return
	}

	if r.size < len(r.points) {
		idx := (r.head + r.size) % len(r.points)
		r.points[idx] = p
		r.size++
		return
	}

	r.points[r.head] = p
	r.head = (r.head + 1) % len(r.points)
}

func (r *Ring) Values() []Point {
	out := make([]Point, 0, r.size)
	for i := 0; i < r.size; i++ {
		idx := (r.head + i) % len(r.points)
		out = append(out, r.points[idx])
	}
	return out
}
