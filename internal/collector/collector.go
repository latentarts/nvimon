package collector

import (
	"context"
	"time"

	"github.com/prods/nvimon/internal/model"
)

type Collector interface {
	Collect(ctx context.Context) (model.HostSnapshot, error)
}

type TimedCollector struct {
	HostID         string
	Hostname       string
	SampleInterval time.Duration
	Now            func() time.Time
}

func NewTimedCollector(hostID, hostname string, sampleInterval time.Duration) TimedCollector {
	return TimedCollector{
		HostID:         hostID,
		Hostname:       hostname,
		SampleInterval: sampleInterval,
		Now:            time.Now,
	}
}
