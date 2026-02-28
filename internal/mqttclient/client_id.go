package mqttclient

import (
	"fmt"
	"hash/fnv"
	"os"
	"regexp"
	"strings"
)

func resolveClientID(opts Options) string {
	if id := strings.TrimSpace(opts.ClientID); id != "" {
		return sanitizeClientID(id)
	}

	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = fallbackClientHostname
	}
	hostPart := sanitizeClientID(host)
	if len(hostPart) > clientIDHostnamePrefixLen {
		hostPart = hostPart[:clientIDHostnamePrefixLen]
	}
	if hostPart == "" {
		hostPart = fallbackClientHostname
	}

	// Stable ID keeps persistent sessions usable across restarts.
	sum := fnv.New32a()
	_, _ = sum.Write([]byte(host + "|" + opts.RootTopic))

	return fmt.Sprintf("mml-%s-%08x", hostPart, sum.Sum32())
}

var mqttClientIDUnsafe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeClientID(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return fallbackClientID
	}
	s = mqttClientIDUnsafe.ReplaceAllString(s, "-")
	if len(s) > maxClientIDLength {
		s = s[:maxClientIDLength]
	}
	if s == "" {
		return fallbackClientID
	}

	return s
}
