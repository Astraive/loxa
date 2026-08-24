package lqlclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestQueryResultUnmarshalSupportsStringAndTypedColumns(t *testing.T) {
	var result QueryResult
	err := json.Unmarshal([]byte(`{"columns":["event_id",{"name":"count","type":"int","nullable":true}],"rows":[{"event_id":"evt-1"}],"duration_ms":7,"row_count":1}`), &result)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Columns) != 2 || result.Columns[0].Name != "event_id" || result.Columns[1].Type != "int" || !result.Columns[1].Nullable || result.DurationMS != 7 || result.RowCount != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestQueryResultUnmarshalRejectsMalformedJSONAndColumns(t *testing.T) {
	cases := []string{
		`{"columns":[1],"rows":[]}`,
		`{"columns":[{"name":1}],"rows":[]}`,
		`{"columns":`,
	}
	for _, payload := range cases {
		t.Run(payload, func(t *testing.T) {
			var result QueryResult
			if err := json.Unmarshal([]byte(payload), &result); err == nil {
				t.Fatalf("json.Unmarshal(%s) unexpectedly succeeded", payload)
			}
		})
	}
}

func TestQuerySendsBasicAuthAndDefaultsLimitWithNilParameters(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, `{"columns":[],"rows":[],"duration_ms":0,"row_count":0}`)
	}))
	defer server.Close()

	client, err := New(ConnectionConfig{Endpoint: server.URL, Collector: "demo", Username: "user", Password: "pass", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Query(context.Background(), "from events", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
	if gotAuth != expectedAuth {
		t.Fatalf("authorization = %q, want %q", gotAuth, expectedAuth)
	}
	if gotBody["limit"] != float64(1000) || gotBody["parameters"] != nil || result.RowCount != 0 || result.Duration != 0 {
		t.Fatalf("body/result = %#v/%#v", gotBody, result)
	}
}

func TestQueryClampsLargeLimitAndRejectsEmptySourceAndUnmarshalableParameters(t *testing.T) {
	client, err := New(ConnectionConfig{Endpoint: "http://localhost:9308", Collector: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Query(context.Background(), " \t\n ", nil, 10); err == nil || ErrorCategoryOf(err) != ErrorInvalidConfiguration {
		t.Fatalf("empty source error = %v", err)
	}
	if _, err := client.Query(context.Background(), "from events", map[string]QueryValue{"bad": {Type: "function", Value: func() {}}}, 10); err == nil || ErrorCategoryOf(err) != ErrorInvalidConfiguration {
		t.Fatalf("unmarshalable parameter error = %v", err)
	}

	var gotLimit float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Limit int `json:"limit"` }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		gotLimit = float64(body.Limit)
		_, _ = io.WriteString(w, `{"columns":[],"rows":[]}`)
	}))
	defer server.Close()
	client, err = New(ConnectionConfig{Endpoint: server.URL, Collector: "demo", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Query(context.Background(), "from events", nil, 5000); err != nil {
		t.Fatal(err)
	}
	if gotLimit != 1000 {
		t.Fatalf("large limit = %v", gotLimit)
	}
}

func TestQuerySetsRowCountAndDurationForZeroRowCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"columns":["id"],"rows":[{"id":1},{"id":2}],"duration_ms":12,"row_count":0}`)
	}))
	defer server.Close()
	client, err := New(ConnectionConfig{Endpoint: server.URL, Collector: "demo", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Query(context.Background(), "from events", nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 2 || result.Duration != 12*time.Millisecond {
		t.Fatalf("result = %#v", result)
	}
}

func TestQueryRejectsMalformedResultEnvelopes(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"invalid JSON", `{`},
		{"null columns", `{"columns":null,"rows":[]}`},
		{"null rows", `{"columns":[],"rows":null}`},
		{"invalid column", `{"columns":[1],"rows":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			client, err := New(ConnectionConfig{Endpoint: server.URL, Collector: "demo", HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Query(context.Background(), "from events", nil, 1)
			if err == nil || ErrorCategoryOf(err) != ErrorMalformedResponse {
				t.Fatalf("error = %v, category = %q", err, ErrorCategoryOf(err))
			}
		})
	}
}

func TestQueryRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"columns":[],"rows":[]}`)
	}))
	defer server.Close()
	client, err := New(ConnectionConfig{Endpoint: server.URL, Collector: "demo", MaxResponseBytes: 5, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(context.Background(), "from events", nil, 1)
	if err == nil || ErrorCategoryOf(err) != ErrorMalformedResponse || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("error = %v, category = %q", err, ErrorCategoryOf(err))
	}
}

func TestQueryMapsHTTPErrorCategoriesAndMessages(t *testing.T) {
	cases := []struct {
		status   int
		body     string
		category string
		message  string
	}{
		{http.StatusUnauthorized, `{"error":"bad credentials"}`, ErrorAuthentication, "bad credentials"},
		{http.StatusForbidden, `{"message":"scope denied"}`, ErrorScope, "scope denied"},
		{http.StatusBadRequest, `{"error":"invalid query","message":"ignored","diagnostics":[{"line":2}]}`, ErrorDiagnostics, "invalid query"},
		{http.StatusServiceUnavailable, `{}`, ErrorCompilerUnavailable, "LQL query failed with HTTP 503"},
		{http.StatusInternalServerError, `{}`, ErrorExecution, "LQL query failed with HTTP 500"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d", tc.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			client, err := New(ConnectionConfig{Endpoint: server.URL, Collector: "demo", HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Query(context.Background(), "from events", nil, 1)
			var queryErr *QueryError
			if !errors.As(err, &queryErr) || queryErr.Category != tc.category || queryErr.Status != tc.status || queryErr.Message != tc.message {
				t.Fatalf("error = %#v", err)
			}
			if tc.status == http.StatusBadRequest && len(queryErr.Diagnostics) != 1 {
				t.Fatalf("diagnostics = %#v", queryErr.Diagnostics)
			}
		})
	}
}

func TestQueryMapsTransportFailure(t *testing.T) {
	client, err := New(ConnectionConfig{
		Endpoint: "http://example.com",
		Collector: "demo",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(context.Background(), "from events", nil, 1)
	if err == nil || ErrorCategoryOf(err) != ErrorTransport || !strings.Contains(err.Error(), "transport failed") {
		t.Fatalf("error = %v, category = %q", err, ErrorCategoryOf(err))
	}
}

func TestQueryMapsCanceledContext(t *testing.T) {
	client, err := New(ConnectionConfig{
		Endpoint: "http://example.com",
		Collector: "demo",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()
			return nil, req.Context().Err()
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.Query(ctx, "from events", nil, 1)
	if err == nil || ErrorCategoryOf(err) != ErrorTimeout || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("error = %v, category = %q", err, ErrorCategoryOf(err))
	}
}

func TestQueryMapsResponseReadFailure(t *testing.T) {
	client, err := New(ConnectionConfig{
		Endpoint: "http://example.com",
		Collector: "demo",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: errorBody{}, Header: make(http.Header)}, nil
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(context.Background(), "from events", nil, 1)
	if err == nil || ErrorCategoryOf(err) != ErrorTransport || !strings.Contains(err.Error(), "could not be read") {
		t.Fatalf("error = %v, category = %q", err, ErrorCategoryOf(err))
	}
}

func TestQueryRejectsInvalidRequestConfiguration(t *testing.T) {
	client := &Client{endpoint: "http://example.com/\n", collector: "demo", timeout: time.Second, maxResponseBytes: 100, httpClient: &http.Client{}}
	_, err := client.Query(context.Background(), "from events", nil, 1)
	if err == nil || ErrorCategoryOf(err) != ErrorInvalidConfiguration || !strings.Contains(err.Error(), "request configuration") {
		t.Fatalf("error = %v, category = %q", err, ErrorCategoryOf(err))
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errorBody struct{}

func (errorBody) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (errorBody) Close() error             { return nil }
func TestQueryJSONEncodesSpecialParameterValues(t *testing.T) {
	var gotBody struct {
		Query      string `json:"query"`
		Parameters map[string]QueryValue `json:"parameters"`
		Limit      int `json:"limit"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, `{"columns":[],"rows":[]}`)
	}))
	defer server.Close()
	client, err := New(ConnectionConfig{Endpoint: server.URL, Collector: "demo", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	value := "quote \" and newline\n and unicode ✓"
	if _, err := client.Query(context.Background(), "from events | where value = $value", map[string]QueryValue{
		"value": {Type: "string", Value: value},
	}, 25); err != nil {
		t.Fatal(err)
	}
	if gotBody.Query != "from events | where value = $value" || gotBody.Limit != 25 || gotBody.Parameters["value"].Value != value {
		t.Fatalf("request body = %#v", gotBody)
	}
}
