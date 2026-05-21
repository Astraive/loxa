package main

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/klauspost/compress/zstd"
)

func withResponseCompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(strings.ToLower(r.Header.Get("Accept-Encoding")), "gzip") &&
			!strings.Contains(strings.ToLower(r.Header.Get("Accept-Encoding")), "zstd") {
			next.ServeHTTP(w, r)
			return
		}

		var (
			writer io.WriteCloser
			header = w.Header()
		)
		switch {
		case strings.Contains(strings.ToLower(r.Header.Get("Accept-Encoding")), "zstd"):
			encoder, err := zstd.NewWriter(w)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			writer = encoder
			header.Set("Content-Encoding", "zstd")
		case strings.Contains(strings.ToLower(r.Header.Get("Accept-Encoding")), "gzip"):
			encoder := gzip.NewWriter(w)
			writer = encoder
			header.Set("Content-Encoding", "gzip")
		}
		if writer == nil {
			next.ServeHTTP(w, r)
			return
		}
		defer writer.Close()
		header.Add("Vary", "Accept-Encoding")

		cw := &compressedResponseWriter{
			ResponseWriter: w,
			writer:         writer,
		}
		next.ServeHTTP(cw, r)
	})
}

type compressedResponseWriter struct {
	http.ResponseWriter
	writer      io.Writer
	wroteHeader bool
}

func (w *compressedResponseWriter) WriteHeader(statusCode int) {
	w.wroteHeader = true
	w.ResponseWriter.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *compressedResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.writer.Write(p)
}

func (w *compressedResponseWriter) Flush() {
	if flusher, ok := w.writer.(interface{ Flush() error }); ok {
		_ = flusher.Flush()
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
