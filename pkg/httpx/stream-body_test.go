package httpx

import (
	"bytes"
	"io"
	"iter"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeFlusher struct {
	bytes.Buffer
	flushes int
}

func (f *fakeFlusher) Flush() { f.flushes++ }

var _ http.Flusher = (*fakeFlusher)(nil)

func TestStreamBodyRead(t *testing.T) {
	events := make(chan SSEEvent, 2)
	events <- NewSSEEvent("1", "msg", []byte("hello"))
	events <- NewSSEEvent("2", "msg", []byte("world"))
	close(events)

	seq := func(yield func(io.WriterTo) bool) {
		for e := range events {
			if !yield(e) {
				return
			}
		}
	}
	body := NewStreamBody(seq)

	expected := "id: 1\nevent: msg\ndata: hello\n\n" + "id: 2\nevent: msg\ndata: world\n\n"

	b, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, expected, string(b))

}

func TestStreamBodyWriteTo(t *testing.T) {
	events := make(chan SSEEvent, 2)
	events <- NewSSEEvent("1", "msg", []byte("foo"))
	events <- NewSSEEvent("2", "msg", []byte("bar"))
	close(events)

	seq := func(yield func(io.WriterTo) bool) {
		for e := range events {
			if !yield(e) {
				return
			}
		}
	}
	body := NewStreamBody(iter.Seq[io.WriterTo](seq))

	f := &fakeFlusher{}
	n, err := body.WriteTo(f)
	require.NoError(t, err)

	expected := "" +
		"id: 1\nevent: msg\ndata: foo\n\n" +
		"id: 2\nevent: msg\ndata: bar\n\n"
	require.Equal(t, expected, f.String())
	require.Equal(t, cap(events), f.flushes)
	require.Equal(t, int64(len(expected)), n)
}

func TestStreamBodyWriteToNoFlush(t *testing.T) {
	events := make(chan SSEEvent, 2)
	events <- NewSSEEvent("1", "msg", []byte("foo"))
	events <- NewSSEEvent("2", "msg", []byte("bar"))
	close(events)

	seq := func(yield func(io.WriterTo) bool) {
		for e := range events {
			if !yield(e) {
				return
			}
		}
	}
	body := NewStreamBody(iter.Seq[io.WriterTo](seq))

	var buf bytes.Buffer
	n, err := body.WriteTo(&buf)
	require.NoError(t, err)

	expected := "" +
		"id: 1\nevent: msg\ndata: foo\n\n" +
		"id: 2\nevent: msg\ndata: bar\n\n"
	require.Equal(t, expected, buf.String())
	require.Equal(t, int64(len(expected)), n)
}
