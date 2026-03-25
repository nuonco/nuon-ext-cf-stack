package debug

import (
	"fmt"
	"os"
	"strconv"
)

var enabled bool

func init() {
	enabled = readEnabled()
}

func readEnabled() bool {
	value := os.Getenv("NUON_DEBUG")
	if value == "" {
		return false
	}

	isEnabled, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}

	return isEnabled
}

// Enabled returns true when NUON_DEBUG resolves to a truthy boolean value.
func Enabled() bool {
	return enabled
}

// Log prints debug output to stderr when NUON_DEBUG is enabled.
func Log(format string, args ...any) {
	if !enabled {
		return
	}

	_, _ = fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
}
