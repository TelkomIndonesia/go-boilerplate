package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAsLogStruct(t *testing.T) {
	in := struct {
		A MaskedString
		S []MaskedString
		M map[string]MaskedString
		C struct {
			A MaskedString
		}
	}{
		A: MaskedString("oijnlkjoiasd"),
		S: []MaskedString{MaskedString("...aosias2"), MaskedString("aojunlknasrc2")},
		M: map[string]MaskedString{
			"one":      MaskedString("adso-olmk0342"),
			"twothree": MaskedString("alk0iasd"),
		},
		C: struct{ A MaskedString }{
			A: MaskedString("asdasdasdasd"),
		},
	}

	out := map[string]interface{}{
		"A": in.A.AsLog(),
		"S": []interface{}{},
		"M": map[interface{}]interface{}{},
		"C": map[string]interface{}{
			"A": in.C.A.AsLog(),
		},
	}
	for _, v := range in.S {
		out["S"] = append(out["S"].([]interface{}), v.AsLog())
	}
	for k, v := range in.M {
		out["M"].(map[interface{}]interface{})[k] = v.AsLog()
	}

	assert.Equal(t, out, AsLog(in))
	assert.Equal(t, out, AsLog(&in))
}

func TestAsLogSlice(t *testing.T) {
	in := []interface{}{
		MaskedString("ajshopijnmlksda"),
		"a",
	}
	out := []interface{}{
		"ajs***",
		"a",
	}
	assert.Equal(t, out, AsLog(in))
	assert.Equal(t, out, AsLog(&in))
}
