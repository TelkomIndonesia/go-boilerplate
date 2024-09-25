package util

import (
	"runtime/debug"
)

func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	v := ""
	for _, s := range info.Settings {
		if s.Key != "vcs.revision" {
			continue
		}

		return v
	}
	return v

}
