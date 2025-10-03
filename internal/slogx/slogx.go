package slogx

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

func New(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(groups []string, attribute slog.Attr) slog.Attr {
			switch attribute.Key {
			case slog.TimeKey:
				return formatTime(attribute)
			case slog.SourceKey:
				return formatSource(attribute)
			default:
				return attribute
			}
		},
	}))
}

func formatTime(attribute slog.Attr) slog.Attr {
	t, ok := attribute.Value.Any().(time.Time)
	if !ok {
		return attribute
	}
	attribute.Value = slog.StringValue(t.Format("2006-01-02T15:04:05.000Z07:00"))
	return attribute
}

func formatSource(attribute slog.Attr) slog.Attr {
	source, ok := attribute.Value.Any().(*slog.Source)
	if !ok {
		return attribute
	}
	fullPath := source.File
	file := filepath.Base(fullPath)
	directory := filepath.Dir(fullPath)
	packageName := filepath.Base(directory)
	formatted := fmt.Sprintf("%s/%s:%d", packageName, file, source.Line)
	attribute.Value = slog.StringValue(formatted)
	return attribute
}
