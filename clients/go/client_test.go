package lqlclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientQueryUsesScopedRouteTypedParametersAndBearer(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotEnv string
	var gotService string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotEnv = r.Header.Get("X-Loza-Env")
		gotService = r.Header.Get("X-Loza-Service")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"columns":[{"name":"event_id","type":"string"}],"rows":[{"event_id":"evt-1"}],"duration_ms":2,"row_count":1}`))
	}))
	defer server.Close()

	client, err := New(ConnectionConfig{
		Endpoint:   server.URL,
		Collector:  "demo",
		Env:        "prod",
		Service:    "cli",
		APIKey:     "api-key",
		Username:   "basic-user",
		Password:   "basic-pass",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Query(context.Background(), "from events | where event_id = $id", map[string]QueryValue{
		"id": {Type: "string", Value: "evt-1"},
	}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/collectors/demo/lql/query" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer api-key" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if gotEnv != "prod" || gotService != "cli" {
		t.Fatalf("scope headers = %q/%q", gotEnv, gotService)
	}
	if gotBody["query"] != "from events | where event_id = $id" {
		t.Fatalf("query body = %#v", gotBody)
	}
	params := gotBody["parameters"].(map[string]any)
	if params["id"].(map[string]any)["type"] != "string" {
		t.Fatalf("typed parameters = %#v", params)
	}
	if result.RowCount != 1 || result.Rows[0]["event_id"] != "evt-1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestClientQueryReturnsStableTimeoutCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()
	client, err := New(ConnectionConfig{Endpoint: server.URL, Collector: "demo", Timeout: time.Millisecond, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(context.Background(), "from events", nil, 1)
	if err == nil || ErrorCategoryOf(err) != ErrorTimeout {
		t.Fatalf("error = %v, category = %q", err, ErrorCategoryOf(err))
	}
}
