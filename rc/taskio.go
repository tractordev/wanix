package rc

import "strings"

// encodeVector NUL-joins v for the task argv/env files. rc is a separate Go
// module with no dependency on tractor.dev/wanix, so misc.EncodeVector is not
// importable here.
// shortcut: no embedded-NUL rejection because shell argv/env cannot carry NUL;
// upgrade if rc gains a misc dependency or task files get non-shell writers.
func encodeVector(v []string) []byte {
	if len(v) == 0 {
		return nil
	}
	return []byte(strings.Join(v, "\x00") + "\x00")
}
