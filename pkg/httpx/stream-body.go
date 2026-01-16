package httpx

import (
	"bytes"
	"io"
	"net/http"
)

var _ io.WriterTo = &StreamBody{}
var _ io.Reader = &StreamBody{}

type StreamBody struct {
	source <-chan io.WriterTo
	buf    bytes.Buffer
}

func NewStreamBody(stream <-chan io.WriterTo) *StreamBody {
	return &StreamBody{
		source: stream,
	}
}

// Read implements [io.Reader].
func (s *StreamBody) Read(p []byte) (n int, err error) {
	if s.buf.Len() == 0 {
		event, ok := <-s.source
		if !ok {
			return 0, io.EOF
		}

		_, err := event.WriteTo(&s.buf)
		if err != nil {
			return 0, err
		}
	}

	return s.buf.Read(p)
}

func (s *StreamBody) WriteTo(w io.Writer) (n int64, err error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		flusher = noopFlusher{}
	}

	for e := range s.source {
		wn, err := e.WriteTo(w)
		n += wn
		if err != nil {
			return n, err
		}

		flusher.Flush()
	}
	return
}

type noopFlusher struct{}

func (noopFlusher) Flush() {}
