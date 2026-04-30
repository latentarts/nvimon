package history

import (
	"testing"
	"time"
)

func TestRingPreservesOrderAcrossWrap(t *testing.T) {
	ring := NewRing(3)
	base := time.Unix(0, 0)

	ring.Push(Point{Time: base.Add(1 * time.Second), Value: 1})
	ring.Push(Point{Time: base.Add(2 * time.Second), Value: 2})
	ring.Push(Point{Time: base.Add(3 * time.Second), Value: 3})
	ring.Push(Point{Time: base.Add(4 * time.Second), Value: 4})

	got := ring.Values()
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}

	if got[0].Value != 2 || got[1].Value != 3 || got[2].Value != 4 {
		t.Fatalf("values = %+v, want [2 3 4]", got)
	}
}

func TestStoreCreatesSeriesOnAppend(t *testing.T) {
	store := NewStore(2)
	store.Append("gpu/0/util", time.Unix(1, 0), 70)

	series := store.Series("gpu/0/util")
	if len(series) != 1 || series[0].Value != 70 {
		t.Fatalf("series = %+v, want one point 70", series)
	}
}
