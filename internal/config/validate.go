package config

import (
	"errors"
	"fmt"
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

	return nil
}
