package httpx

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
)

var _ io.WriterTo = &SSEEvent{}

type SSEEvent struct {
	id         string
	event      string
	dataWriter func(w io.Writer) (n int, err error)
	retry      int
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

func (e SSEEvent) WithRetry(retry int) SSEEvent {
	return SSEEvent{e.id, e.event, e.dataWriter, retry}
}

func (e SSEEvent) WriteTo(w io.Writer) (n int64, err error) {
	written := false

	if e.id != "" {
		written = true
		wn, err := writeField(w, "id: ", e.id)
		n += int64(wn)
		if err != nil {
			return n, err
		}
	}

	if e.event != "" {
		written = true
		wn, err := writeField(w, "event: ", e.event)
		n += int64(wn)
		if err != nil {
			return n, err
		}
	}

	if e.dataWriter != nil {
		written = true
		wn, err := io.WriteString(w, "data: ")
		n += int64(wn)
		if err != nil {
			return n, err
		}

		wn, err = e.dataWriter(w)
		n += int64(wn)
		if err != nil {
			return n, err
		}
	}

	if e.retry > 0 {
		written = true
		wn, err := writeField(w, "retry: ", strconv.Itoa(e.retry))
		n += int64(wn)
		if err != nil {
			return n, err
		}
	}

	eol := "\n"
	if !written {
		eol = ":\n"
	}
	wn, err := io.WriteString(w, eol)
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
