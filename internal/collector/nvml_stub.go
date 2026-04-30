//go:build !cgo

package collector

import (
	"context"
	"errors"

	"github.com/prods/nvimon/internal/model"
)

type nvmlCollector struct{}

func newNVMLCollector() *nvmlCollector {
	return &nvmlCollector{}
}

func (c *nvmlCollector) Collect(context.Context) (string, []model.GPUSnapshot, []model.GPUProcess, []model.CollectorError, error) {
	return "nvml", nil, nil, nil, errors.New("nvml unavailable in CGO_DISABLED build")
}
