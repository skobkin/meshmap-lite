package meshtastic

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

func nodeIDFromNum(v uint32) string {
	if v == 0 {
		return ""
	}

	return fmt.Sprintf("!%08x", v)
}

func normalizeNodeID(v string) string {
	s := strings.TrimSpace(strings.ToLower(v))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "!") {
		return s
	}
	if len(s) == 8 {
		if _, err := hex.DecodeString(s); err == nil {
			return "!" + s
		}
	}
	if n, err := strconv.ParseUint(s, 10, 32); err == nil && n > 0 {
		return fmt.Sprintf("!%08x", uint32(n))
	}

	return s
}
