package cardwatch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchAndOffenders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Scope-OrgID") != "tenant-b" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`{"labels":[{"label_name":"pod","label_values_count":50000},{"label_name":"svc","label_values_count":50},{"label_name":"user_id","label_values_count":90000}]}`))
	}))
	defer srv.Close()
	ls, err := Fetch(context.Background(), srv.Client(), srv.URL, "tenant-b", 20)
	if err != nil {
		t.Fatal(err)
	}
	off := Offenders(ls, 1000)
	if len(off) != 2 || off[0].Name != "user_id" {
		t.Fatalf("unexpected offenders: %+v", off)
	}
}
