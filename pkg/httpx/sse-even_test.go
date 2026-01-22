package httpx

import (
	"bytes"
	"testing"
)

func TestSSEEvent_WriteTo(t *testing.T) {
	tests := []struct {
		name   string
		event  SSEEvent
		expect string
	}{
		{
			name:   "simple data",
			event:  NewSSEEvent("123", "msg", []byte("hello")),
			expect: "id: 123\nevent: msg\ndata: hello\n\n",
		},
		{
			name: "json data",
			event: NewSSEEventJSON("id1", "json", map[string]any{
				"foo": "bar",
			}),
			expect: "id: id1\nevent: json\ndata: {\"foo\":\"bar\"}\n\n",
		},
		{
			name:   "with retry",
			event:  NewSSEEvent("1", "update", []byte("ping")).WithRetry(5000),
			expect: "id: 1\nevent: update\ndata: ping\nretry: 5000\n\n",
		},
		{
			name:   "blank event produces :",
			event:  SSEEvent{},
			expect: ":\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			_, err := tc.event.WriteTo(&buf)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := buf.String()
			if got != tc.expect {
				t.Fatalf("unexpected output:\nwant %q\ngot  %q", tc.expect, got)
			}
		})
	}
}
