package nethttp

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

type maskFH struct{ *baseTestWriter }
func (*maskFH) Flush() {}
func (*maskFH) Hijack() (net.Conn, *bufio.ReadWriter, error) { return nil, nil, context.Canceled }

type maskFP struct{ *baseTestWriter }
func (*maskFP) Flush() {}
func (*maskFP) Push(string, *http.PushOptions) error { return nil }

type maskFR struct{ *baseTestWriter }
func (*maskFR) Flush() {}
func (*maskFR) ReadFrom(r io.Reader) (int64, error) { return io.Copy(io.Discard, r) }

type maskHP struct{ *baseTestWriter }
func (*maskHP) Hijack() (net.Conn, *bufio.ReadWriter, error) { return nil, nil, context.Canceled }
func (*maskHP) Push(string, *http.PushOptions) error { return nil }

type maskHR struct{ *baseTestWriter }
func (*maskHR) Hijack() (net.Conn, *bufio.ReadWriter, error) { return nil, nil, context.Canceled }
func (*maskHR) ReadFrom(r io.Reader) (int64, error) { return io.Copy(io.Discard, r) }

type maskPR struct{ *baseTestWriter }
func (*maskPR) Push(string, *http.PushOptions) error { return nil }
func (*maskPR) ReadFrom(r io.Reader) (int64, error) { return io.Copy(io.Discard, r) }

type maskFHP struct{ *baseTestWriter }
func (*maskFHP) Flush() {}
func (*maskFHP) Hijack() (net.Conn, *bufio.ReadWriter, error) { return nil, nil, context.Canceled }
func (*maskFHP) Push(string, *http.PushOptions) error { return nil }

type maskFHR struct{ *baseTestWriter }
func (*maskFHR) Flush() {}
func (*maskFHR) Hijack() (net.Conn, *bufio.ReadWriter, error) { return nil, nil, context.Canceled }
func (*maskFHR) ReadFrom(r io.Reader) (int64, error) { return io.Copy(io.Discard, r) }

type maskFPR struct{ *baseTestWriter }
func (*maskFPR) Flush() {}
func (*maskFPR) Push(string, *http.PushOptions) error { return nil }
func (*maskFPR) ReadFrom(r io.Reader) (int64, error) { return io.Copy(io.Discard, r) }

type inflatedReaderFrom struct{ *baseTestWriter }
func (*inflatedReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	_, _ = io.Copy(io.Discard, r)
	return 2, nil
}

type maskHPR struct{ *baseTestWriter }
func (*maskHPR) Hijack() (net.Conn, *bufio.ReadWriter, error) { return nil, nil, context.Canceled }
func (*maskHPR) Push(string, *http.PushOptions) error { return nil }
func (*maskHPR) ReadFrom(r io.Reader) (int64, error) { return io.Copy(io.Discard, r) }

func TestResponseWriterCoversEveryOptionalInterfaceCombination(t *testing.T) {
	cases := []struct {
		name string
		writer http.ResponseWriter
		mask int
	}{
		{"fh", &maskFH{newBaseTestWriter()}, optionalFlusher | optionalHijacker},
		{"fp", &maskFP{newBaseTestWriter()}, optionalFlusher | optionalPusher},
		{"fr", &maskFR{newBaseTestWriter()}, optionalFlusher | optionalReaderFrom},
		{"hp", &maskHP{newBaseTestWriter()}, optionalHijacker | optionalPusher},
		{"hr", &maskHR{newBaseTestWriter()}, optionalHijacker | optionalReaderFrom},
		{"pr", &maskPR{newBaseTestWriter()}, optionalPusher | optionalReaderFrom},
		{"fhp", &maskFHP{newBaseTestWriter()}, optionalFlusher | optionalHijacker | optionalPusher},
		{"fhr", &maskFHR{newBaseTestWriter()}, optionalFlusher | optionalHijacker | optionalReaderFrom},
		{"fpr", &maskFPR{newBaseTestWriter()}, optionalFlusher | optionalPusher | optionalReaderFrom},
		{"hpr", &maskHPR{newBaseTestWriter()}, optionalHijacker | optionalPusher | optionalReaderFrom},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped, state := newResponseWriter(tc.writer)
			if state == nil || wrapped == nil {
				t.Fatal("newResponseWriter returned nil")
			}
			if tc.mask&optionalFlusher != 0 {
				wrapped.(http.Flusher).Flush()
			}
			if tc.mask&optionalHijacker != 0 {
				_, _, _ = wrapped.(http.Hijacker).Hijack()
			}
			if tc.mask&optionalPusher != 0 {
				_ = wrapped.(http.Pusher).Push("/asset", nil)
			}
			if tc.mask&optionalReaderFrom != 0 {
				_, _ = wrapped.(io.ReaderFrom).ReadFrom(strings.NewReader("reader"))
			}
		})
	}
	fullWrapped, _ := newResponseWriter(&fullFeaturedWriter{newBaseTestWriter()})
	fullWrapped.(http.Flusher).Flush()
	_, _, _ = fullWrapped.(http.Hijacker).Hijack()
	_ = fullWrapped.(http.Pusher).Push("/full", nil)
	_, _ = fullWrapped.(io.ReaderFrom).ReadFrom(strings.NewReader("full"))
	baseWrapped := &responseWriter{ResponseWriter: newBaseTestWriter()}
	if _, _, err := baseWrapped.hijack(); err != http.ErrNotSupported {
		t.Fatalf("unsupported hijack error = %v", err)
	}
	if err := baseWrapped.push("/unsupported", nil); err != http.ErrNotSupported {
		t.Fatalf("unsupported push error = %v", err)
	}

	base := &responseWriter{ResponseWriter: &inflatedReaderFrom{newBaseTestWriter()}}
	if n, err := base.readFrom(strings.NewReader("x")); err != nil || n != 2 || base.bytes != 2 {
		t.Fatalf("inflated ReadFrom = n:%d err:%v bytes:%d", n, err, base.bytes)
	}
}
