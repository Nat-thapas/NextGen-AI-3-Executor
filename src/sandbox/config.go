package sandbox

import (
	"path/filepath"
	"time"

	"next-gen-ai-web-application/executor/src/utils/env"
)

type ConfigConcurrency struct {
	Limit int
}

type ConfigPath struct {
	Isolate   string
	BoxesRoot string
	Temp      string
	Metadata  string
	Stdin     string
	Stdout    string
}

type ConfigTimeoutIsolate struct {
	Init    time.Duration
	Exec    time.Duration
	Cleanup time.Duration
}

type ConfigTimeout struct {
	WaitDelay   time.Duration
	Isolate     ConfigTimeoutIsolate
	WallDefault time.Duration
}

type ConfigType struct {
	Concurrency ConfigConcurrency
	Path        ConfigPath
	Timeout     ConfigTimeout
}

var Config = ConfigType{
	Concurrency: ConfigConcurrency{
		Limit: env.GetIntEnvWithDefault("CONCURRENT_EXEC_LIMIT", 16),
	},
	Path: ConfigPath{
		Isolate:   env.GetEnvWithDefault("ISOLATE_PATH", "/usr/local/bin/isolate"),
		BoxesRoot: env.GetEnvWithDefault("ISOLATE_BOXES_ROOT", "/var/local/lib/isolate/"),
		Temp:      env.GetEnvWithDefault("TEMP_PATH", "/tmp/next-gen-ai-executor"),
		Metadata:  filepath.Join(env.GetEnvWithDefault("TEMP_PATH", "/tmp/next-gen-ai-executor"), "__metadata__"),
		Stdin:     "__stdin__",
		Stdout:    "__stdout__",
	},
	Timeout: ConfigTimeout{
		WaitDelay: env.GetDurationEnvWithDefault("EXEC_WAIT_DELAY", 5*time.Second),
		Isolate: ConfigTimeoutIsolate{
			Init:    env.GetDurationEnvWithDefault("ISOLATE_INIT_TIMEOUT", 5*time.Second),
			Exec:    env.GetDurationEnvWithDefault("ISOLATE_EXEC_TIMEOUT", 5*time.Second),
			Cleanup: env.GetDurationEnvWithDefault("ISOLATE_CLEANUP_TIMEOUT", 5*time.Second),
		},
		WallDefault: 300 * time.Second,
	},
}
