package config

import (
	"strconv"
	"strings"
	"time"
)

func splitCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}

func mustInt(v string, d int) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		return d
	}

	return n
}

func mustBool(v string, d bool) bool {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return d
	}

	return b
}

func mustDuration(v string, d time.Duration) time.Duration {
	t, err := time.ParseDuration(v)
	if err != nil {
		return d
	}

	return t
}

func mustFloat(v string, d float64) float64 {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return d
	}

	return f
}

func mustByte(v string, d byte) byte {
	n, err := strconv.Atoi(v)
	if err != nil {
		return d
	}
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}

	return byte(n)
}
