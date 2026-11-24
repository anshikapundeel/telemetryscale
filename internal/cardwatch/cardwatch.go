// Package cardwatch flags high-cardinality labels using Mimir's cardinality API.
package cardwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
)

type Label struct {
	Name   string `json:"label_name"`
	Values int    `json:"label_values_count"`
}

type response struct {
	Labels []Label `json:"labels"`
}

// Fetch queries {base}/prometheus/api/v1/cardinality/label_names for a tenant.
func Fetch(ctx context.Context, c *http.Client, base, tenant string, limit int) ([]Label, error) {
	u := fmt.Sprintf("%s/prometheus/api/v1/cardinality/label_names?limit=%d", base, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Scope-OrgID", tenant)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("mimir returned %d", resp.StatusCode)
	}
	var r response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return r.Labels, nil
}

// Offenders returns labels whose distinct value count exceeds threshold, worst first.
func Offenders(ls []Label, threshold int) []Label {
	var out []Label
	for _, l := range ls {
		if l.Values > threshold {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Values > out[j].Values })
	return out
}
