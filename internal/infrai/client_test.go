package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryMetricsIncludesRequiredParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("name"); got != "donations.received" {
			t.Errorf("name = %q, want donations.received", got)
		}
		if got := r.URL.Query().Get("agg"); got != "sum" {
			t.Errorf("agg = %q, want sum", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.baseURL = server.URL
	if _, err := client.QueryMetrics(context.Background(), "donations.received", "sum"); err != nil {
		t.Fatal(err)
	}
}
