// logquery benchmarks the same log queries against Elasticsearch and ClickHouse.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anshikapundeel/telemetryscale/internal/stats"
)

type q struct{ name, es, ch string }

func main() {
	es := flag.String("es", "http://localhost:9200", "Elasticsearch URL")
	ch := flag.String("ch", "http://localhost:8123", "ClickHouse HTTP URL")
	n := flag.Int("n", 30, "runs per query")
	term := flag.String("term", "kerberos", "search term")
	flag.Parse()

	queries := []q{
		{"term-count",
			fmt.Sprintf(`{"size":0,"track_total_hits":true,"query":{"match":{"message":%q}}}`, *term),
			fmt.Sprintf("SELECT count() FROM telemetry.logs WHERE positionCaseInsensitive(body, '%s') > 0", *term)},
		{"errors-by-service",
			`{"size":0,"query":{"term":{"severity.keyword":"ERROR"}},"aggs":{"s":{"terms":{"field":"service.keyword","size":10}}}}`,
			"SELECT service, count() c FROM telemetry.logs WHERE severity = 'ERROR' GROUP BY service ORDER BY c DESC LIMIT 10"},
		{"by-trace-id-point-lookup",
			`{"size":10,"query":{"term":{"trace_id.keyword":"0000000000000000ffffffffffffffff"}}}`,
			"SELECT * FROM telemetry.logs WHERE trace_id = '0000000000000000ffffffffffffffff' LIMIT 10"},
	}
	fmt.Printf("%-26s %-14s %10s %10s %10s\n", "query", "backend", "p50", "p95", "p99")
	for _, qq := range queries {
		var de, dc []time.Duration
		for i := 0; i < *n; i++ {
			de = append(de, timeIt(func() error { return esQuery(*es, qq.es) }))
			dc = append(dc, timeIt(func() error { return chQuery(*ch, qq.ch) }))
		}
		report(qq.name, "elasticsearch", de)
		report(qq.name, "clickhouse", dc)
	}
	fmt.Println()
	fmt.Println("storage:")
	fmt.Println("  elasticsearch:", strings.TrimSpace(get(*es+"/_cat/indices/logs-*?h=index,docs.count,store.size&bytes=b")))
	fmt.Println("  clickhouse   :", strings.TrimSpace(chText(*ch, "SELECT count(), sum(bytes_on_disk) FROM system.parts WHERE database='telemetry' AND table='logs' AND active")))
}

func timeIt(f func() error) time.Duration {
	s := time.Now()
	if err := f(); err != nil {
		return -1
	}
	return time.Since(s)
}

func report(name, backend string, d []time.Duration) {
	var ok []time.Duration
	for _, x := range d {
		if x >= 0 {
			ok = append(ok, x)
		}
	}
	if len(ok) == 0 {
		fmt.Printf("%-26s %-14s %s\n", name, backend, "all runs failed")
		return
	}
	fmt.Printf("%-26s %-14s %10v %10v %10v\n", name, backend,
		stats.Percentile(ok, 50).Round(time.Microsecond), stats.Percentile(ok, 95).Round(time.Microsecond), stats.Percentile(ok, 99).Round(time.Microsecond))
}

func esQuery(base, body string) error {
	resp, err := http.Post(base+"/logs-*/_search", "application/json", bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func chQuery(base, sql string) error { _, err := chTextErr(base, sql); return err }

func chText(base, sql string) string { s, _ := chTextErr(base, sql); return s }

func chTextErr(base, sql string) (string, error) {
	resp, err := http.Post(base+"/?query="+url.QueryEscape(sql), "text/plain", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, b)
	}
	return string(b), nil
}

func get(u string) string {
	resp, err := http.Get(u)
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}
