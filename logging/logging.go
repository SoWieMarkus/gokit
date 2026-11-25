// Package logging configures the default slog logger.
package logging

import (
	"fmt"
	"log/slog"
	"os"
)

// Setup makes a JSON ("json") or human readable ("text") logger on stdout the
// default logger.
func Setup(level slog.Level, format string) error {
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		return fmt.Errorf("log format must be json or text, got %q", format)
	}
	slog.SetDefault(slog.New(handler))
	return nil
}
