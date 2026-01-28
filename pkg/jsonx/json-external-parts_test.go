package jsonx

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// simple JSON model with fileref-like reference
type testDoc struct {
	Name  string `json:"name"`
	Photo string `json:"photo"`
}

func TestJSONMultiPart_Success(t *testing.T) {
	// --- prepare JSON ---
	jsonContent := `{"name":"alice","photo":"fileref://photo"}`

	// --- prepare multipart ---
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	// JSON part
	h := make(textproto.MIMEHeader)
	h.Set("Content-Type", "application/json")
	jsonPart, _ := w.CreatePart(h)
	jsonPart.Write([]byte(jsonContent))

	// external photo part
	photoPart, _ := w.CreateFormFile("photo", "photo.bin")
	photoPart.Write([]byte("BINARYDATA"))

	// unreferenced external photo part
	photoxPart, _ := w.CreateFormFile("photox", "photox.bin")
	photoxPart.Write([]byte("BINARYDATA"))

	w.Close()

	// --- create reader ---
	rr := NewJSONMultiPartReader(multipart.NewReader(&body, w.Boundary()))

	// --- define schema registering fileref format on "photo" field ---
	extractor := func(r io.Reader, pt *testDoc) (map[string]struct{}, error) {
		err := json.NewDecoder(r).Decode(pt)
		if err != nil {
			return nil, err
		}

		ref, _ := strings.CutPrefix(pt.Photo, "fileref://")
		return map[string]struct{}{
			ref: {},
		}, nil
	}
	// --- parse ---
	jm, err := NewJSONExternalParts(rr, extractor)
	require.NoError(t, err)

	// --- verify decoded JSON ---
	doc := jm.JSON()
	assert.Equal(t, "alice", doc.Name)
	assert.Equal(t, "fileref://photo", doc.Photo)

	// --- verify parts ---
	var parts []JSONExternalPart
	for part, err := range jm.Parts() {
		require.NoError(t, err)

		assert.Equal(t, "photo", part.Key())
		assert.NotEmpty(t, part.Meta())

		parts = append(parts, part)
		val, err := io.ReadAll(part.Value())
		require.NoError(t, err)
		assert.Equal(t, "BINARYDATA", string(val))
	}
	require.Len(t, parts, 1)
}
