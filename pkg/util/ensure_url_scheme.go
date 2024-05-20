package util

import (
	"fmt"
	"net/url"
	"strings"
)

func EnsureUrlScheme(rawURL string) (*url.URL, error) {
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("fail to parse url: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		parsedURL.Scheme = "https"
	}
	return parsedURL, nil
}
