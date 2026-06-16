# Upgrade Notes

## 2026-06-16 - Environment configuration decoding is stricter

### Affected configuration

- `MML_*` environment variables

### Explanation

Environment variables are now loaded through `koanf`, the same configuration tree used for YAML. This makes nested ENV-only configuration available for all config fields, including list entries such as `MML_UPDATE_CHECK__SOURCES__0__TYPE=github`.

Invalid ENV values for typed fields now fail startup during config decoding instead of being silently ignored in favor of YAML or default values. Examples include invalid integers, booleans, floats, durations, and byte-sized values.

### Migration

1. Review local `MML_*` environment variables before upgrading.
2. Fix or remove invalid values such as `MML_MQTT__PORT=not-a-number` or `MML_WEB__CHAT__HISTORY_WINDOW=not-a-duration`.
3. For list-style configuration from ENV, use numeric path segments such as `MML_UPDATE_CHECK__SOURCES__0__NAME=meshtastic-firmware`.
