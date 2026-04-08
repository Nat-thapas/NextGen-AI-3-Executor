package sandbox

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type ExecResultMetaCSW struct {
	Forced    int
	Voluntary int
}

type ExecResultMetaTime struct {
	Real time.Duration
	Wall time.Duration
}

type ExecResultMeta struct {
	Time            ExecResultMetaTime
	Memory          int
	ContextSwitches ExecResultMetaCSW
	ExitCode        int
	ExitSignal      int
	Killed          bool
	Message         string
	Status          string
}

func ParseMetadata(metadataPath string) (ExecResultMeta, error) {
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		return ExecResultMeta{}, fmt.Errorf("failed to read metadata file. error: %w", err)
	}

	metadata := ExecResultMeta{}

	for line := range strings.SplitSeq(string(content), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch key {
		case "csw-forced":
			if val, err := strconv.Atoi(value); err == nil {
				metadata.ContextSwitches.Forced = val
			}
		case "csw-voluntary":
			if val, err := strconv.Atoi(value); err == nil {
				metadata.ContextSwitches.Voluntary = val
			}
		case "exitcode":
			if val, err := strconv.Atoi(value); err == nil {
				metadata.ExitCode = val
			}
		case "exitsig":
			if val, err := strconv.Atoi(value); err == nil {
				metadata.ExitSignal = val
			}
		case "killed":
			if val, err := strconv.Atoi(value); err == nil {
				metadata.Killed = val == 1
			}
		case "max-rss":
			if val, err := strconv.Atoi(value); err == nil {
				metadata.Memory = val * 1024
			}
		case "message":
			metadata.Message = value
		case "status":
			metadata.Status = value
		case "time":
			if val, err := strconv.ParseFloat(value, 64); err == nil {
				metadata.Time.Real = time.Duration(val * float64(time.Second))
			}
		case "time-wall":
			if val, err := strconv.ParseFloat(value, 64); err == nil {
				metadata.Time.Wall = time.Duration(val * float64(time.Second))
			}
		}
	}

	return metadata, nil
}
