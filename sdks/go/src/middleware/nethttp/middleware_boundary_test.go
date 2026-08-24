package nethttp

import (
	"crypto/tls"
	"net/http"
	"testing"
)

func TestNetHTTPHelperBoundaries(t *testing.T) {
	if got := schemeFromRequest(&http.Request{TLS: &tls.ConnectionState{}}); got != "https" {
		t.Fatalf("TLS scheme = %q", got)
	}
	if got := schemeFromRequest(&http.Request{}); got != "http" {
		t.Fatalf("plain scheme = %q", got)
	}
	if got := firstForwardedIP(" , unknown, "); got != "" {
		t.Fatalf("empty forwarded chain = %q", got)
	}
	if got := remoteAddrIP(" 203.0.113.1 "); got != "203.0.113.1" {
		t.Fatalf("plain remote address = %q", got)
	}
	if isValidAttrKey("") || isValidAttrKey("UPPER") || isValidAttrKey("bad/key") || isValidAttrKey(string(make([]byte, 257))) {
		t.Fatal("invalid attribute keys were accepted")
	}
	_ = Middleware(Config{Event: "custom"})
	req := &http.Request{Header: http.Header{"Bad/Header": []string{"value"}}}
	if attrs := selectedHeaderAttrs(req, []string{"Bad/Header"}); len(attrs) != 0 {
		t.Fatalf("invalid selected header attrs = %#v", attrs)
	}
	if got := remoteAddrIP("example.test:443"); got != "example.test" {
		t.Fatalf("hostname remote address = %q", got)
	}
	if !isValidAttrKey("http.header.x-request-id") {
		t.Fatal("valid attribute key was rejected")
	}
}
