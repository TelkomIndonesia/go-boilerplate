package version

import (
	"fmt"
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
			fmt.Println(s.Key, s.Value)
			continue
		}
		v = s.Value
	}
	return v

}
