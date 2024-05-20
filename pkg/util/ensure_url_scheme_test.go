package util

import (
	"testing"
)

func TestEnsureUrlScheme(t *testing.T) {
	tests := []struct {
		input          string
		expectedScheme string
		expectedHost   string
		expectedPort   string
	}{
		{"www.example.com", "https", "www.example.com", ""},
		{"http://www.example.com", "http", "www.example.com", ""},
		{"https://www.example.com", "https", "www.example.com", ""},
		{"ftp://www.example.com", "https", "www.example.com", ""},
		{"example", "https", "example", ""},
		{"example:8443", "https", "example", "8443"},
	}

	for _, test := range tests {
		result, err := EnsureUrlScheme(test.input)
		if err != nil {
			t.Errorf("did not expect an error for input %s, but got %v", test.input, err)
			continue
		}
		if result.Scheme != test.expectedScheme {
			t.Errorf("for input %s, expected scheme %s but got %s", test.input, test.expectedScheme, result.Scheme)
		}
		if result.Hostname() != test.expectedHost {
			t.Errorf("for input %s, expected host %s but got %s", test.input, test.expectedHost, result.Hostname())
		}
		if result.Port() != test.expectedPort {
			t.Errorf("for input %s, expected port %s but got %s", test.input, test.expectedPort, result.Port())
		}
	}
}
