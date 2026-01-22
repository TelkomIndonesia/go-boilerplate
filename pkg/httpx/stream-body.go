package httpx

import (
	"bytes"
	"io"
	"iter"
	"net/http"
)

var _ io.WriterTo = &StreamBody{}
var _ io.Reader = &StreamBody{}

type StreamBody struct {
	source iter.Seq[io.WriterTo]
	buf    bytes.Buffer
}

func NewStreamBody(stream iter.Seq[io.WriterTo]) *StreamBody {
	return &StreamBody{
		source: stream,
	}
}

func (s *StreamBody) Read(p []byte) (n int, err error) {
	if s.buf.Len() < len(p) {
		for event := range s.source {
			n64, err := event.WriteTo(&s.buf)
			if err != nil {
				return int(n64), err
			}
			if s.buf.Len() >= len(p) {
				break
			}
		}
	}

	if s.buf.Len() == 0 {
		return 0, io.EOF
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
