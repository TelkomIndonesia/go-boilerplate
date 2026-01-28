package jsonx

import (
	"fmt"
	"io"
	"iter"
	"mime/multipart"
	"strings"
)

// JSONExternalParts is a JSON document with fields that reference external parts
type JSONExternalParts[T any] struct {
	json      T
	partsRef  map[string]struct{}
	partsIter iter.Seq2[JSONExternalPart, error]
}

func NewJSONExternalParts[T any](rr JSONExternalPartsReader, extract JSONExternalPartsDecoder[T]) (jm JSONExternalParts[T], err error) {
	jsonr, err := rr.JSON()
	if err != nil {
		return JSONExternalParts[T]{}, err
	}

	jm.partsRef, err = extract(jsonr, &jm.json)
	if err != nil {
		return JSONExternalParts[T]{}, fmt.Errorf("failed to extract parts: %w", err)
	}

	jm.partsIter = rr.ExternalParts()
	return jm, nil
}

func (j JSONExternalParts[T]) JSON() T {
	return j.json
}

func (j JSONExternalParts[T]) Parts() iter.Seq2[JSONExternalPart, error] {
	yielded := map[string]struct{}{}
	return func(yield func(JSONExternalPart, error) bool) {
		for part, err := range j.partsIter {
			if err != nil {
				ok := yield(nil, err)
				if !ok {
					return
				}
				continue
			}

			if _, ok := yielded[part.Key()]; ok {
				continue
			}

			if _, ok := j.partsRef[part.Key()]; !ok {
				continue
			}

			yielded[part.Key()] = struct{}{}

			ok := yield(part, nil)
			if !ok {
				return
			}
		}
	}
}

// JSONExternalPartsReader is a wrapper around io.Reader that should contains multiple logical parts
// where the first part is a json
// while the remaining part are an externally-stored data but referenced in the json
type JSONExternalPartsReader interface {
	JSON() (io.Reader, error)
	ExternalParts() iter.Seq2[JSONExternalPart, error]
}

// JSONExternalPart is an external part that is referenced in the json
type JSONExternalPart interface {
	Key() string
	Meta() map[string][]string
	Value() io.Reader
}

// JSONExternalPartsDecoder decode the json and returns a map of references to external parts
type JSONExternalPartsDecoder[T any] func(t io.Reader, pt *T) (refs map[string]struct{}, err error)

var _ JSONExternalPartsReader = mfReader{}

type mfReader struct {
	reader *multipart.Reader
}

func NewJSONMultiPartReader(r *multipart.Reader) JSONExternalPartsReader {
	return &mfReader{reader: r}
}

func (m mfReader) JSON() (io.Reader, error) {
	for {
		part, err := m.reader.NextPart()
		if err == io.EOF {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}

		if !strings.Contains(strings.ToLower(part.Header.Get("content-type")), "application/json") {
			continue
		}

		return part, nil
	}
}

func (m mfReader) ExternalParts() iter.Seq2[JSONExternalPart, error] {
	return func(yield func(JSONExternalPart, error) bool) {
		for {
			part, err := m.reader.NextPart()
			if err == io.EOF {
				return
			}
			if err != nil {
				ok := yield(nil, err)
				if !ok {
					return
				}
				continue
			}

			if !yield(mfPart{part}, nil) {
				return
			}
		}
	}
}

type mfPart struct {
	part *multipart.Part
}

func (p mfPart) Key() string {
	return p.part.FormName()
}

func (p mfPart) Meta() map[string][]string {
	return p.part.Header
}

func (p mfPart) Value() io.Reader {
	return p.part
}
