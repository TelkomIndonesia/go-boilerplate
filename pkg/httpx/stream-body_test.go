package httpx

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestStreamBodySSERead(t *testing.T) {
	// 1. Setup the source channel and the stream
	source := make(chan io.WriterTo, 1)
	stream := &StreamBody{
		source: source,
	}

	// 2. Define a test event with data that needs JSON encoding
	event := SSEJSONEvent{
		ID:    "123",
		Event: "update",
		Data:  map[string]string{"status": "ok"},
	}

	// 3. Send the event
	source <- event
	close(source)

	// 4. Read from the stream using the io.Reader interface
	// We'll use a small buffer to test the internal buffering logic
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("failed to read from stream: %v", err)
	}

	output := string(buf[:n])

	// 5. Assertions
	expectedParts := []string{
		"id: 123\n",
		"event: update\n",
		"data: {\"status\":\"ok\"}\n\n", // json.Encoder adds a newline
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("expected output to contain %q, but got %q", part, output)
		}
	}

	// 6. Verify EOF
	_, err = stream.Read(buf)
	if err != io.EOF {
		t.Errorf("expected EOF after channel close, got %v", err)
	}
}

func TestStreamBodySSELargePayload(t *testing.T) {
	// Tests if the internal buffer handles small reads correctly
	source := make(chan io.WriterTo, 1)
	stream := &StreamBody{source: source}

	source <- SSEJSONEvent{ID: "1", Data: "small"}
	close(source)

	// Read only 5 bytes at a time to force the stream to use its internal buffer
	p := make([]byte, 5)
	n, err := stream.Read(p)
	if err != nil || n != 5 {
		t.Fatalf("first read failed: n=%d, err=%v", n, err)
	}

	if stream.buf.Len() == 0 {
		t.Error("expected remaining data to be held in internal buffer")
	}
}

func TestStreamBodySSEProtocolTermination(t *testing.T) {
	var buf bytes.Buffer
	event := SSEJSONEvent{Data: "test"}

	event.WriteTo(&buf)

	if !strings.HasSuffix(buf.String(), "\n\n") {
		t.Errorf("SSE event must end with double newline, got %q", buf.String())
	}
}
