package logger

import (
	"log/slog"

	"next-gen-ai-web-application/executor/src/utils/env"
)

type ConfigType struct {
	LevelStr string
	Level    slog.Level
}

var Config = ConfigType{
	LevelStr: env.GetEnvWithDefault("LOG_LEVEL", "WARN"),
	Level: env.GetMappedEnvWithDefault("LOG_LEVEL", slog.LevelWarn, map[string]slog.Level{
		"DEBUG":   slog.LevelDebug,
		"INFO":    slog.LevelInfo,
		"WARN":    slog.LevelWarn,
		"WARNING": slog.LevelWarn,
		"ERROR":   slog.LevelError,
	}),
}
