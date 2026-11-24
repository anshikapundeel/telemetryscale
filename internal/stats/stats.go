// Package stats has small helpers for benchmark reporting.
package stats

import (
	"sort"
	"time"
)

// Percentile returns the p-th percentile (0-100) using nearest-rank.
func Percentile(d []time.Duration, p float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	c := append([]time.Duration(nil), d...)
	sort.Slice(c, func(i, j int) bool { return c[i] < c[j] })
	rank := int(p/100*float64(len(c))+0.9999999) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(c) {
		rank = len(c) - 1
	}
	return c[rank]
}
