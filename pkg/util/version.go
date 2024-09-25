package util

import (
	"runtime/debug"
)

func Version() (v string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	for _, s := range info.Settings {
		if s.Key != "vcs.revision" {
			continue
		}

		v = s.Value
		break
	}
	return

}
