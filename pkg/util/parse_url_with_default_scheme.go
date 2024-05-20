package util

import (
	"fmt"
	"net/url"
	"strings"
)

func ParseURLWithDefaultScheme(u string, defaultScheme string) (*url.URL, error) {
	if !strings.Contains(u, "://") {
		u = fmt.Sprintf("%s://%s", defaultScheme, u)
	}
	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("fail to parse url: %w", err)
	}
	return parsedURL, nil
}
