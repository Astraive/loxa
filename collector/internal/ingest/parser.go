package ingest

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	speccontract "github.com/astraive/loxa/spec/generated/go/contract"
	"github.com/klauspost/compress/zstd"
)

func ParseEvents(r *http.Request, maxBodyBytes int64) ([][]byte, error) {
	defer r.Body.Close()

	reader := io.Reader(r.Body)
	encoding := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding")))
	switch {
	case strings.Contains(encoding, "gzip"):
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	case strings.Contains(encoding, "zstd"):
		decoder, err := zstd.NewReader(r.Body)
		if err != nil {
			return nil, err
		}
		defer decoder.Close()
		reader = decoder
	}

	payload, err := io.ReadAll(io.LimitReader(reader, maxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > maxBodyBytes {
		return nil, fmt.Errorf("payload exceeds max body bytes")
	}
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return nil, nil
	}

	switch payload[0] {
	case '[':
		var arr []json.RawMessage
		if err := json.Unmarshal(payload, &arr); err != nil {
			return nil, err
		}
		out := make([][]byte, 0, len(arr))
		for _, item := range arr {
			out = append(out, bytes.TrimSpace([]byte(item)))
		}
		return out, nil
	case '{':
		var envelope speccontract.IngestEnvelope
		if err := json.Unmarshal(payload, &envelope); err == nil && len(envelope.Events) > 0 {
			out := make([][]byte, 0, len(envelope.Events))
			for _, item := range envelope.Events {
				out = append(out, bytes.TrimSpace([]byte(item)))
			}
			return out, nil
		}
		var single json.RawMessage
		if err := json.Unmarshal(payload, &single); err == nil {
			return [][]byte{payload}, nil
		}
		// If object parsing failed, fall back to NDJSON handling.
		if bytes.Contains(payload, []byte{'\n'}) {
			ndjson := ParseNDJSON(payload)
			if len(ndjson) > 1 {
				return ndjson, nil
			}
		}
		return nil, fmt.Errorf("invalid JSON object payload")
	default:
		return ParseNDJSON(payload), nil
	}
}

func ParseNDJSON(payload []byte) [][]byte {
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	buffer := make([]byte, 0, 1024*1024)
	scanner.Buffer(buffer, math.MaxInt32)

	out := make([][]byte, 0, 64)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		copyLine := make([]byte, len(line))
		copy(copyLine, line)
		out = append(out, copyLine)
	}
	return out
}
