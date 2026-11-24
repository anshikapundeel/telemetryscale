package stats

import (
	"testing"
	"time"
)

func TestPercentile(t *testing.T) {
	var d []time.Duration
	for i := 1; i <= 100; i++ {
		d = append(d, time.Duration(i)*time.Millisecond)
	}
	if got := Percentile(d, 50); got != 50*time.Millisecond {
		t.Fatalf("p50=%v", got)
	}
	if got := Percentile(d, 99); got != 99*time.Millisecond {
		t.Fatalf("p99=%v", got)
	}
	if Percentile(nil, 99) != 0 {
		t.Fatal("empty should be 0")
	}
}
