package codex

import (
	"fmt"
	"io"
	"iter"
	"mime/multipart"
	"strings"
)

// ExternalFielder is a generic type with fields that reference external parts
type ExternalFielder[T any] struct {
	main      T
	partsRef  map[string]struct{}
	partsIter iter.Seq2[ExternalField, error]
}

func NewExternalFielder[T any](efr ExternalFielderReader, extract ExternalFielderDecoder[T]) (jm ExternalFielder[T], err error) {
	mr, err := efr.Main()
	if err != nil {
		return ExternalFielder[T]{}, err
	}

	jm.partsRef, err = extract(mr, &jm.main)
	if err != nil {
		return ExternalFielder[T]{}, fmt.Errorf("failed to extract parts: %w", err)
	}

	jm.partsIter = efr.ExternalFields()
	return jm, nil
}

func (j ExternalFielder[T]) Main() T {
	return j.main
}

func (j ExternalFielder[T]) ExternalFields() iter.Seq2[ExternalField, error] {
	yielded := map[string]struct{}{}
	return func(yield func(ExternalField, error) bool) {
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

// ExternalFielderReader is a wrapper around io.Reader that should contains multiple logical parts
// where the first part is the main object/data
// while the remaining part are an externally-stored data but referenced in the main object/data
type ExternalFielderReader interface {
	Main() (io.Reader, error)
	ExternalFields() iter.Seq2[ExternalField, error]
}

// ExternalField is an external part that is referenced in the main object
type ExternalField interface {
	Key() string
	Meta() map[string][]string
	Value() io.Reader
}

// ExternalFielderDecoder decode the main object and returns a map of references to external parts
type ExternalFielderDecoder[T any] func(t io.Reader, pt *T) (refs map[string]struct{}, err error)

var _ ExternalFielderReader = mfReader{}

type mfReader struct {
	reader      *multipart.Reader
	contentType string
}

func NewExternalFielderMultipart(r *multipart.Reader, ct string) ExternalFielderReader {
	return &mfReader{reader: r, contentType: ct}
}

func (m mfReader) Main() (io.Reader, error) {
	for {
		part, err := m.reader.NextPart()
		if err == io.EOF {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}

		if m.contentType != "" && !strings.Contains(strings.ToLower(part.Header.Get("content-type")), m.contentType) {
			continue
		}

		return part, nil
	}
}

func (m mfReader) ExternalFields() iter.Seq2[ExternalField, error] {
	return func(yield func(ExternalField, error) bool) {
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
