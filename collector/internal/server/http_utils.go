package server

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
)

func parseNDJSONEvents(body io.Reader, maxBytes int64) ([][]byte, error) {
	limitReader := body
	if maxBytes > 0 {
		limitReader = io.LimitReader(body, maxBytes+1)
	}
	scanner := bufio.NewScanner(limitReader)
	if maxBytes > 0 {
		buf := make([]byte, maxBytes)
		scanner.Buffer(buf, int(maxBytes))
	}
	var events [][]byte
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		events = append(events, line)
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return events, errors.New("payload exceeds max_body_bytes")
		}
		return nil, err
	}
	return events, nil
}

func itoa(n int) string {
	return strconv.Itoa(n)
}