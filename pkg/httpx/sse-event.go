package httpx

import (
	"encoding/json"
	"io"
)

var _ io.WriterTo = &SSEJSONEvent{}

type SSEJSONEvent struct {
	ID    string
	Event string
	Data  any
}

func (e SSEJSONEvent) WriteTo(w io.Writer) (n int64, err error) {
	wn, err := writeField(w, "id: ", e.ID)
	n += int64(wn)
	if err != nil {
		return
	}

	wn, err = writeField(w, "event: ", e.Event)
	n += int64(wn)
	if err != nil {
		return
	}

	wn, err = io.WriteString(w, "data: ")
	n += int64(wn)
	if err != nil {
		return
	}

	tw := &trackedWriter{Writer: w, n: 0}
	err = json.NewEncoder(tw).Encode(e.Data)
	n += int64(tw.n)
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
