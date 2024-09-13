package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
)

func TestMasked(t *testing.T) {
	data := []struct {
		in  log.Loggable
		out string
	}{
		{
			in:  MaskedString(""),
			out: "***",
		},
		{
			in:  MaskedString("1"),
			out: "***",
		},
		{
			in:  MaskedString("12"),
			out: "***",
		},
		{
			in:  MaskedString("123"),
			out: "***",
		},
		{
			in:  MaskedString("1234567890"),
			out: "123***",
		},
		{
			in:  MaskedStringPrefix(""),
			out: "***",
		},
		{
			in:  MaskedStringPrefix("1"),
			out: "***",
		},
		{
			in:  MaskedStringPrefix("12"),
			out: "***",
		},
		{
			in:  MaskedStringPrefix("123"),
			out: "***",
		},
		{
			in:  MaskedStringPrefix("1234567890"),
			out: "***890",
		},
		{
			in:  MaskedStringUserURL("http://username:password@host:1000/path"),
			out: "http://use~~~:pas~~~@host:1000/path",
		},
		{
			in:  MaskedStringUserURL("http://name:word@host/path"),
			out: "http://nam~~~:wor~~~@host/path",
		},
		{
			in:  MaskedStringUserURL("http://tes:tes@host/path"),
			out: "http://~~~:~~~@host/path",
		},
		{
			in:  MaskedStringUserURL("http://host/path"),
			out: "http://host/path",
		},
		{
			in:  MaskedStringUserURL("postgres://testing:testing@postgres:5432/testing?sslmode=disable"),
			out: "postgres://tes~~~:tes~~~@postgres:5432/testing?sslmode=disable",
		},
	}

	for _, d := range data {
		out := d.in.AsLog()
		assert.Equal(t, d.out, out)
	}
}

func TestString(t *testing.T) {
	strings := []string{
		"asdasdadas",
		"***",
		"http://host.com",
	}

	for _, d := range strings {
		m := MaskedString(d)
		assert.Equal(t, d, m.String())

		p := MaskedStringPrefix(d)
		assert.Equal(t, d, p.String())

		u := MaskedStringUserURL(d)
		assert.Equal(t, d, u.String())
	}
}
