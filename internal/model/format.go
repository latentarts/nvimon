package model

import "fmt"

func FormatBytes(v uint64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
		tib = 1024 * gib
	)

	switch {
	case v >= tib:
		return fmt.Sprintf("%.1f TiB", float64(v)/float64(tib))
	case v >= gib:
		return fmt.Sprintf("%.1f GiB", float64(v)/float64(gib))
	case v >= mib:
		return fmt.Sprintf("%.1f MiB", float64(v)/float64(mib))
	case v >= kib:
		return fmt.Sprintf("%.1f KiB", float64(v)/float64(kib))
	default:
		return fmt.Sprintf("%d B", v)
	}
}

func FormatPercent(m MetricValue) string {
	return m.String("%", 1)
}
