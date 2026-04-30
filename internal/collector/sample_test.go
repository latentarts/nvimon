package collector

import (
	"context"
	"testing"
	"time"
)

func TestSampleCollectorProducesSnapshot(t *testing.T) {
	c := NewSampleCollector("host-a", "gpu-a", time.Second)
	c.Now = func() time.Time { return time.Unix(1714474800, 0) }

	snapshot, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	if snapshot.HostID != "host-a" || snapshot.Hostname != "gpu-a" {
		t.Fatalf("snapshot identity = %+v", snapshot)
	}

	if len(snapshot.GPUs) != 2 {
		t.Fatalf("gpu count = %d, want 2", len(snapshot.GPUs))
	}

	if len(snapshot.GPUProcesses) == 0 {
		t.Fatal("expected sample GPU processes")
	}
}
