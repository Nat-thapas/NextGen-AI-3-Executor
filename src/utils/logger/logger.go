package logger

import (
	"log/slog"
	"os"
)

var Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	Level: Config.Level,
}))

func init() {
	Logger.Info("Logger started", "level", Config.LevelStr)
}
