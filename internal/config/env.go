package config

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

func loadEnv(k *koanf.Koanf) error {
	if err := k.Load(env.ProviderWithValue(envPrefix, ".", envValue), nil); err != nil {
		return err
	}

	return normalizeEnvSlices(k)
}

func envValue(key, value string) (string, interface{}) {
	segments := envSegments(strings.TrimPrefix(key, envPrefix))
	if len(segments) == 0 {
		return "", nil
	}

	joined := strings.Join(segments, ".")
	if strings.HasPrefix(joined, "channels.") && strings.HasSuffix(joined, ".events") {
		return joined, splitCSV(value)
	}

	return joined, value
}

func envSegments(path string) []string {
	rawSegments := strings.Split(path, envNestingSeparator)
	segments := make([]string, len(rawSegments))
	copy(segments, rawSegments)
	for i := range segments {
		segments[i] = strings.ToLower(segments[i])
	}
	if len(rawSegments) >= 3 && strings.EqualFold(rawSegments[0], "channels") {
		segments[0] = "channels"
		segments[1] = strings.TrimSpace(rawSegments[1])
	}

	return segments
}

func normalizeEnvSlices(k *koanf.Koanf) error {
	rawSources := k.Get("update_check.sources")
	if rawSources == nil {
		return nil
	}

	sources, ok, err := indexedEnvMapToSlice(rawSources)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	return k.Set("update_check.sources", sources)
}

func indexedEnvMapToSlice(raw interface{}) ([]interface{}, bool, error) {
	rawMap, ok := raw.(map[string]interface{})
	if !ok || len(rawMap) == 0 {
		return nil, false, nil
	}

	indexes := make([]int, 0, len(rawMap))
	byIndex := make(map[int]interface{}, len(rawMap))
	for key, value := range rawMap {
		index, err := strconv.Atoi(key)
		if err != nil || index < 0 {
			return nil, false, fmt.Errorf("update_check.sources index %q must be a non-negative integer", key)
		}
		indexes = append(indexes, index)
		byIndex[index] = value
	}
	sort.Ints(indexes)

	out := make([]interface{}, indexes[len(indexes)-1]+1)
	for _, index := range indexes {
		out[index] = byIndex[index]
	}

	return out, true, nil
}
