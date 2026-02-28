package logging

import (
	"io"
	"log/slog"
	"os"
	"time"
)

const humanTimeLayout = "2006-01-02 15:04:05.000"

// Options configures logger construction and global default replacement.
type Options struct {
	Level      string
	Writer     io.Writer
	SetDefault bool
}

func buildLogger(opts Options) (*slog.Logger, error) {
	handler, err := buildHandler(opts)
	if err != nil {
		return nil, err
	}

	return slog.New(handler), nil
}

func buildHandler(opts Options) (slog.Handler, error) {
	level, err := parseLevel(opts.Level)
	if err != nil {
		return nil, err
	}

	writer := opts.Writer
	if writer == nil {
		writer = os.Stdout
	}

	return slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceAttrs,
	}), nil
}

func replaceAttrs(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key != slog.TimeKey {
		return attr
	}
	ts, ok := attr.Value.Any().(time.Time)
	if !ok {
		return attr
	}

	return slog.String(slog.TimeKey, ts.Format(humanTimeLayout))
}
