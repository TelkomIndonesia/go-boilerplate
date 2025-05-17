package logvaluer

import (
	"net/url"

	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

var (
	_ log.Valuer = MaskedString("")
	_ log.Valuer = MaskedStringUserURL("")
)

type MaskedString string

func (m MaskedString) String() string {
	return string(m)
}

func (m MaskedString) AsLog() any {
	return m.mask("***")
}

func (m MaskedString) mask(replacer string) string {
	if l := len(m); l <= 3 {
		return replacer
	}
	return string(m)[:3] + replacer
}

type MaskedStringPrefix string

func (m MaskedStringPrefix) String() string {
	return string(m)
}

func (m MaskedStringPrefix) AsLog() any {
	return m.mask("***")
}

func (m MaskedStringPrefix) mask(replacer string) string {
	if l := len(m); l <= 3 {
		return replacer
	}
	return replacer + string(m)[len(m)-3:]
}

type MaskedStringUserURL string

func (m MaskedStringUserURL) String() string {
	return string(m)
}

func (m MaskedStringUserURL) AsLog() any {
	u, err := url.Parse(string(m))
	if err != nil {
		return string(m)
	}

	if u.User == nil {
		return string(m)
	}

	p, _ := u.User.Password()
	u.User = url.UserPassword(
		MaskedString(u.User.Username()).mask("~~~"),
		MaskedString(p).mask("~~~"),
	)
	return u.String()
}
