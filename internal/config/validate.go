package config

import (
	"errors"
	"fmt"
	"strings"

	"meshmap-lite/internal/updatecheck"
)

func validate(cfg Config) error {
	if cfg.MQTT.RootTopic == "" {
		return errors.New("mqtt.root_topic is required")
	}
	if cfg.Storage.SQL.Driver != "sqlite" {
		return fmt.Errorf("unsupported storage.sql.driver: %s", cfg.Storage.SQL.Driver)
	}
	if cfg.Storage.KV.Driver != "memory" {
		return fmt.Errorf("unsupported storage.kv.driver: %s", cfg.Storage.KV.Driver)
	}
	if len(cfg.Channels) == 0 {
		return errors.New("at least one channel must be configured")
	}
	switch cfg.Web.Map.PrecisionCirclesMode {
	case MapPrecisionCirclesNone, MapPrecisionCirclesSelected, MapPrecisionCirclesAlways:
	default:
		return fmt.Errorf("unsupported web.map.precision_circles_mode: %s", cfg.Web.Map.PrecisionCirclesMode)
	}

	primary := 0
	for _, channel := range cfg.Channels {
		if channel.Primary {
			primary++
		}
	}
	if primary > 1 {
		return errors.New("at most one channels.*.primary=true is allowed")
	}

	return validateUpdateCheck(cfg.UpdateCheck)
}

// validateUpdateCheck enforces the multi-source update-check invariants.
func validateUpdateCheck(cfg UpdateCheckConfig) error {
	if !cfg.Enabled {
		return nil
	}
	seen := make(map[string]struct{}, len(cfg.Sources))
	for i, src := range cfg.Sources {
		name := strings.TrimSpace(src.Name)
		if name == "" {
			return fmt.Errorf("update_check.sources[%d].name is required", i)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("update_check.sources[%d]: duplicate source name %q", i, name)
		}
		seen[name] = struct{}{}

		switch strings.TrimSpace(src.Type) {
		case updatecheck.SourceTypeForgejo, updatecheck.SourceTypeGitHub:
		default:
			return fmt.Errorf("update_check.sources[%d].type %q is not supported", i, src.Type)
		}
		if strings.TrimSpace(src.Repository) == "" {
			return fmt.Errorf("update_check.sources[%d].repository is required", i)
		}
	}

	return nil
}
