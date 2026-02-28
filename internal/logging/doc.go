// Package logging configures process logging and component-scoped child loggers.
//
// Manager owns one active slog logger. Reconfiguration swaps the managed logger
// atomically. Global default replacement is opt-in per configuration call via
// Options.SetDefault and is therefore explicit in tests.
package logging
