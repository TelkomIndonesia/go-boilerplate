package httpx

import (
	"bytes"
	"encoding/json"
	"io"
)

var _ io.WriterTo = &SSEEvent{}

type SSEEvent struct {
	id         string
	event      string
	dataWriter func(w io.Writer) (n int, err error)
}

var newline = []byte{'\n'}

func NewSSEEvent(id string, event string, data []byte) SSEEvent {
	return NewSSEEventIOReader(id, event, bytes.NewReader(data))
}

func NewSSEEventIOReader(id string, event string, data io.Reader) SSEEvent {
	return SSEEvent{
		id:    id,
		event: event,
		dataWriter: func(w io.Writer) (n int, err error) {
			n64, err := io.Copy(w, data)
			if err != nil {
				return int(n64), err
			}

			n, err = w.Write(newline)
			return int(n64) + n, err
		},
	}
}

func NewSSEEventJSON(id string, event string, data any) SSEEvent {
	return SSEEvent{
		id:    id,
		event: event,
		dataWriter: func(w io.Writer) (n int, err error) {
			tw := &trackedWriter{Writer: w}
			err = json.NewEncoder(tw).Encode(data)
			return tw.n, err
		},
	}
}

func (e SSEEvent) WriteTo(w io.Writer) (n int64, err error) {
	wn, err := writeField(w, "id: ", e.id)
	n += int64(wn)
	if err != nil {
		return
	}

	wn, err = writeField(w, "event: ", e.event)
	n += int64(wn)
	if err != nil {
		return
	}

	wn, err = io.WriteString(w, "data: ")
	n += int64(wn)
	if err != nil {
		return
	}

	wn, err = e.dataWriter(w)
	n += int64(wn)
	if err != nil {
		return
	}

	wn, err = io.WriteString(w, "\n")
	n += int64(wn)
	if err != nil {
		return n, err
	}

	return
}

func writeField(w io.Writer, prefix, value string) (n int, err error) {
	if value == "" {
		return 0, nil
	}

	wn, err := io.WriteString(w, prefix)
	n += wn
	if err != nil {
		return n, err
	}

	wn, err = io.WriteString(w, value)
	n += wn
	if err != nil {
		return n, err
	}

	wn, err = io.WriteString(w, "\n")
	n += wn
	return n, err
}

type trackedWriter struct {
	io.Writer
	n int
}

func (tw *trackedWriter) Write(p []byte) (n int, err error) {
	n, err = tw.Writer.Write(p)
	tw.n += n
	return
}
