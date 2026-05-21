package nethttp

import (
	"bufio"
	"io"
	"net"
	"net/http"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	bytes       int
	wroteHeader bool
}

func newResponseWriter(w http.ResponseWriter) (http.ResponseWriter, *responseWriter) {
	base := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	_, flusher := w.(http.Flusher)
	_, hijacker := w.(http.Hijacker)
	_, pusher := w.(http.Pusher)
	_, readerFrom := w.(io.ReaderFrom)

	switch optionalInterfaceMask(flusher, hijacker, pusher, readerFrom) {
	case 0:
		return base, base
	case optionalFlusher:
		return &responseWriterFlusher{responseWriter: base}, base
	case optionalHijacker:
		return &responseWriterHijacker{responseWriter: base}, base
	case optionalPusher:
		return &responseWriterPusher{responseWriter: base}, base
	case optionalReaderFrom:
		return &responseWriterReaderFrom{responseWriter: base}, base
	case optionalFlusher | optionalHijacker:
		return &responseWriterFlusherHijacker{responseWriter: base}, base
	case optionalFlusher | optionalPusher:
		return &responseWriterFlusherPusher{responseWriter: base}, base
	case optionalFlusher | optionalReaderFrom:
		return &responseWriterFlusherReaderFrom{responseWriter: base}, base
	case optionalHijacker | optionalPusher:
		return &responseWriterHijackerPusher{responseWriter: base}, base
	case optionalHijacker | optionalReaderFrom:
		return &responseWriterHijackerReaderFrom{responseWriter: base}, base
	case optionalPusher | optionalReaderFrom:
		return &responseWriterPusherReaderFrom{responseWriter: base}, base
	case optionalFlusher | optionalHijacker | optionalPusher:
		return &responseWriterFlusherHijackerPusher{responseWriter: base}, base
	case optionalFlusher | optionalHijacker | optionalReaderFrom:
		return &responseWriterFlusherHijackerReaderFrom{responseWriter: base}, base
	case optionalFlusher | optionalPusher | optionalReaderFrom:
		return &responseWriterFlusherPusherReaderFrom{responseWriter: base}, base
	case optionalHijacker | optionalPusher | optionalReaderFrom:
		return &responseWriterHijackerPusherReaderFrom{responseWriter: base}, base
	default:
		return &responseWriterFlusherHijackerPusherReaderFrom{responseWriter: base}, base
	}
}

func (w *responseWriter) WriteHeader(code int) {
	w.wroteHeader = true
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

func (w *responseWriter) flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *responseWriter) hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}

func (w *responseWriter) push(target string, opts *http.PushOptions) error {
	p, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return p.Push(target, opts)
}

func (w *responseWriter) readFrom(r io.Reader) (int64, error) {
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		cr := &countingReader{r: r}
		n, err := rf.ReadFrom(cr)
		w.bytes += int(cr.n)
		if !w.wroteHeader {
			w.wroteHeader = true
		}
		if n > cr.n {
			// Defensive: prefer underlying returned byte count when larger.
			w.bytes += int(n - cr.n)
		}
		return n, err
	}
	return io.Copy(w, r)
}

type responseWriterFlusher struct{ *responseWriter }

func (w *responseWriterFlusher) Flush() { w.flush() }

type responseWriterHijacker struct{ *responseWriter }

func (w *responseWriterHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.hijack()
}

type responseWriterPusher struct{ *responseWriter }

func (w *responseWriterPusher) Push(target string, opts *http.PushOptions) error {
	return w.push(target, opts)
}

type responseWriterReaderFrom struct{ *responseWriter }

func (w *responseWriterReaderFrom) ReadFrom(r io.Reader) (int64, error) { return w.readFrom(r) }

type responseWriterFlusherHijacker struct{ *responseWriter }

func (w *responseWriterFlusherHijacker) Flush() { w.flush() }
func (w *responseWriterFlusherHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.hijack()
}

type responseWriterFlusherPusher struct{ *responseWriter }

func (w *responseWriterFlusherPusher) Flush() { w.flush() }
func (w *responseWriterFlusherPusher) Push(target string, opts *http.PushOptions) error {
	return w.push(target, opts)
}

type responseWriterFlusherReaderFrom struct{ *responseWriter }

func (w *responseWriterFlusherReaderFrom) Flush() { w.flush() }
func (w *responseWriterFlusherReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return w.readFrom(r)
}

type responseWriterHijackerPusher struct{ *responseWriter }

func (w *responseWriterHijackerPusher) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.hijack()
}
func (w *responseWriterHijackerPusher) Push(target string, opts *http.PushOptions) error {
	return w.push(target, opts)
}

type responseWriterHijackerReaderFrom struct{ *responseWriter }

func (w *responseWriterHijackerReaderFrom) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.hijack()
}
func (w *responseWriterHijackerReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return w.readFrom(r)
}

type responseWriterPusherReaderFrom struct{ *responseWriter }

func (w *responseWriterPusherReaderFrom) Push(target string, opts *http.PushOptions) error {
	return w.push(target, opts)
}
func (w *responseWriterPusherReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return w.readFrom(r)
}

type responseWriterFlusherHijackerPusher struct{ *responseWriter }

func (w *responseWriterFlusherHijackerPusher) Flush() { w.flush() }
func (w *responseWriterFlusherHijackerPusher) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.hijack()
}
func (w *responseWriterFlusherHijackerPusher) Push(target string, opts *http.PushOptions) error {
	return w.push(target, opts)
}

type responseWriterFlusherHijackerReaderFrom struct{ *responseWriter }

func (w *responseWriterFlusherHijackerReaderFrom) Flush() { w.flush() }
func (w *responseWriterFlusherHijackerReaderFrom) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.hijack()
}
func (w *responseWriterFlusherHijackerReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return w.readFrom(r)
}

type responseWriterFlusherPusherReaderFrom struct{ *responseWriter }

func (w *responseWriterFlusherPusherReaderFrom) Flush() { w.flush() }
func (w *responseWriterFlusherPusherReaderFrom) Push(target string, opts *http.PushOptions) error {
	return w.push(target, opts)
}
func (w *responseWriterFlusherPusherReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return w.readFrom(r)
}

type responseWriterHijackerPusherReaderFrom struct{ *responseWriter }

func (w *responseWriterHijackerPusherReaderFrom) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.hijack()
}
func (w *responseWriterHijackerPusherReaderFrom) Push(target string, opts *http.PushOptions) error {
	return w.push(target, opts)
}
func (w *responseWriterHijackerPusherReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return w.readFrom(r)
}

type responseWriterFlusherHijackerPusherReaderFrom struct{ *responseWriter }

func (w *responseWriterFlusherHijackerPusherReaderFrom) Flush() { w.flush() }
func (w *responseWriterFlusherHijackerPusherReaderFrom) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.hijack()
}
func (w *responseWriterFlusherHijackerPusherReaderFrom) Push(target string, opts *http.PushOptions) error {
	return w.push(target, opts)
}
func (w *responseWriterFlusherHijackerPusherReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return w.readFrom(r)
}

const (
	optionalFlusher = 1 << iota
	optionalHijacker
	optionalPusher
	optionalReaderFrom
)

func optionalInterfaceMask(flusher, hijacker, pusher, readerFrom bool) int {
	mask := 0
	if flusher {
		mask |= optionalFlusher
	}
	if hijacker {
		mask |= optionalHijacker
	}
	if pusher {
		mask |= optionalPusher
	}
	if readerFrom {
		mask |= optionalReaderFrom
	}
	return mask
}

type countingReader struct {
	r io.Reader
	n int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}
