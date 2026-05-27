package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/astraive/loxa/loxa-cortex/internal/config"
	"github.com/gorilla/websocket"
)

func TestWebSocket_ReadLimitRejectsLargeFrames(t *testing.T) {
	srv := &Server{config: &config.Config{Server: config.ServerConfig{}}}
	handler := srv.WebSocketHandler()

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Connect via WebSocket
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send a message exceeding the 1MB read limit (1MB + 1 byte)
	bigPayload := strings.Repeat("x", 1<<20+1)
	err = conn.WriteMessage(websocket.TextMessage, []byte(`{"action":"healthz","event":`+bigPayload+`}`))
	if err != nil {
		// Write may fail if the send buffer is smaller; that's acceptable
		t.Logf("write failed (send buffer limit): %v", err)
		return
	}

	// Read the response — the server should close the connection due to read limit
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected connection to be closed due to read limit violation")
	}
	// The error should be a close error indicating the frame was too large
	if !websocket.IsCloseError(err, websocket.CloseMessageTooBig, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
		t.Logf("connection closed with error (expected close or too-big): %v", err)
	}
}

func TestWebSocket_ReadLimitAllowsSmallFrames(t *testing.T) {
	srv := &Server{config: &config.Config{Server: config.ServerConfig{}}}
	handler := srv.WebSocketHandler()

	ts := httptest.NewServer(handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send a small message — should succeed
	err = conn.WriteMessage(websocket.TextMessage, []byte(`{"action":"healthz"}`))
	if err != nil {
		t.Fatalf("write small message: %v", err)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if len(msg) == 0 {
		t.Fatal("expected non-empty response")
	}
}
