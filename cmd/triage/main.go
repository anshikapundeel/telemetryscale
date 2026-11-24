// triage gathers metrics (Mimir) and logs (ClickHouse) for an alert and drafts a note.
// Uses an LLM if ANTHROPIC_API_KEY is set; otherwise falls back to an offline summary.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/anshikapundeel/telemetryscale/internal/triage"
)

func main() {
	alert := flag.String("alert", "SLOBurnRate ingest-availability", "alert name/description")
	mimir := flag.String("mimir", "http://localhost:9009", "Mimir base URL")
	tenant := flag.String("tenant", "tenant-a", "tenant id")
	ch := flag.String("ch", "http://localhost:8123", "ClickHouse HTTP URL")
	model := flag.String("model", envOr("TRIAGE_MODEL", "claude-sonnet-5-5"), "model id")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tc := triage.Context{Alert: *alert}
	for _, q := range []string{
		`sum(rate(ts_batches_total{result="failed"}[5m]))`,
		`ts_queue_depth`,
		`sum(rate(ts_batch_retries_total[5m]))`,
	} {
		if v, err := instant(ctx, *mimir, *tenant, q); err == nil {
			tc.Metrics = append(tc.Metrics, fmt.Sprintf("%s = %s", q, v))
		}
	}
	if rows, err := chText(ctx, *ch, "SELECT concat(severity,' ',service,' ',body) FROM telemetry.logs WHERE severity IN ('ERROR','WARN') ORDER BY ts DESC LIMIT 40"); err == nil {
		tc.Logs = strings.Split(strings.TrimSpace(rows), "\n")
	}

	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		fmt.Println(triage.Offline(tc))
		return
	}
	c := &triage.Client{Endpoint: "https://api.anthropic.com/v1/messages", APIKey: key, Model: *model, HTTP: &http.Client{Timeout: 45 * time.Second}}
	out, err := c.Summarize(ctx, tc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "llm failed, using offline summary:", err)
		fmt.Println(triage.Offline(tc))
		return
	}
	fmt.Println(out)
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func instant(ctx context.Context, base, tenant, q string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/prometheus/api/v1/query?query="+url.QueryEscape(q), nil)
	req.Header.Set("X-Scope-OrgID", tenant)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r struct {
		Data struct {
			Result []struct{ Value [2]any } `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil || len(r.Data.Result) == 0 {
		return "", fmt.Errorf("no data")
	}
	return fmt.Sprint(r.Data.Result[0].Value[1]), nil
}

func chText(ctx context.Context, base, sql string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/?query="+url.QueryEscape(sql), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	return string(b), nil
}
