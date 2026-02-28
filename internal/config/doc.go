// Package config loads application settings from YAML and environment variables.
//
// YAML provides the base configuration and `MML_` environment variables override
// it using `__` as the nesting separator. Channel names are resolved
// case-insensitively while preserving the first configured key casing.
package config
