package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"next-gen-ai-web-application/executor/src/utils/file"
	"next-gen-ai-web-application/executor/src/utils/logger"
)

type ExecLimitTime struct {
	Real time.Duration
	Wall time.Duration
}

type ExecLimit struct {
	Time       ExecLimitTime
	Memory     int
	Processes  int
	OpenFiles  int
	OutputSize int
}

type ExecFilePath struct {
	Path string
	Perm os.FileMode
}

type ExecFileContent struct {
	Content []byte
	Perm    os.FileMode
}

type ExecOptions struct {
	StdioMode         string // none, pipe, file
	Stdin             []byte
	Env               map[string]string
	InputFilePaths    map[string]ExecFilePath    // Copy from value to key
	InputFileContents map[string]ExecFileContent // Write from value to key
	OutputFilePaths   map[string]ExecFilePath    // Copy from key to value
	Limit             ExecLimit
}

type ExecResult struct {
	ExitCode      int
	Output        []byte
	IsolateOutput []byte
	Metadata      ExecResultMeta
}

var boxes = make(chan int, Config.Concurrency.Limit)

func init() {
	for i := 0; i < Config.Concurrency.Limit; i++ {
		boxes <- i
	}

	if err := os.RemoveAll(Config.Path.Temp); err != nil {
		log.Fatalf("failed to cleanup temp directory at %s. error: %v\n", Config.Path.Temp, err)
	}

	if err := os.MkdirAll(Config.Path.Temp, 0750); err != nil {
		log.Fatalf("failed to create temp directory at %s. error: %v\n", Config.Path.Temp, err)
	}

	if err := os.MkdirAll(Config.Path.Metadata, 0750); err != nil {
		log.Fatalf("failed to create metadata directory at %s. error: %v\n", Config.Path.Metadata, err)
	}
}

func Execute(ctx context.Context, command []string, options ExecOptions) (ExecResult, error) {
	id := <-boxes
	defer func() { boxes <- id }()

	logger.Logger.Debug("sandbox: starting execution", "boxId", id)

	boxId := strconv.Itoa(id)
	boxRoot := filepath.Join(Config.Path.BoxesRoot, boxId, "box")
	metadataPath := filepath.Join(Config.Path.Metadata, boxId)

	initCmdCtx, initCmdCancel := context.WithTimeout(ctx, Config.Timeout.Isolate.Init)
	defer initCmdCancel()

	initCmd := exec.CommandContext(initCmdCtx, Config.Path.Isolate, "--init", "-b", boxId)
	initCmd.Stdin = nil
	initCmd.WaitDelay = Config.Timeout.WaitDelay
	if output, err := initCmd.CombinedOutput(); err != nil {
		logger.Logger.Error("sandbox: failed to initialize sandbox", "boxId", boxId, "error", err, "output", output)
		return ExecResult{}, fmt.Errorf("failed to initialize sandbox. error: %w. output: %s", err, output)
	}

	logger.Logger.Debug("sandbox: initialization completed, setting up sandbox", "boxId", boxId)

	defer func() {
		cleanupCmdCtx, cleanupCmdCancel := context.WithTimeout(ctx, Config.Timeout.Isolate.Cleanup)
		defer cleanupCmdCancel()

		cleanupCmd := exec.CommandContext(cleanupCmdCtx, Config.Path.Isolate, "--cleanup", "-b", boxId)
		cleanupCmd.Stdin = nil
		cleanupCmd.WaitDelay = Config.Timeout.WaitDelay
		if output, err := cleanupCmd.CombinedOutput(); err != nil {
			logger.Logger.Warn("sandbox: failed to cleanup sandbox", "boxId", boxId, "error", err, "output", output)
		} else {
			logger.Logger.Debug("sandbox: cleanup completed", "boxId", boxId)
		}
	}()

	isolateArgs := make([]string, 0, 32)
	isolateArgs = append(isolateArgs, "--run", "-b", boxId, "-M", metadataPath)

	setupEG := errgroup.Group{}

	if options.StdioMode == "file" && len(options.Stdin) > 0 {
		setupEG.Go(func() error {
			destinationPath := filepath.Join(boxRoot, Config.Path.Stdin)
			if err := os.WriteFile(destinationPath, options.Stdin, 0644); err != nil {
				logger.Logger.Error("sandbox: failed to create stdin file", "boxId", boxId, "error", err, "destinationPath", destinationPath)
				return fmt.Errorf("failed to create stdin file. error: %w", err)
			}
			logger.Logger.Debug("created stdin file", "boxId", boxId, "destinationPath", destinationPath)
			return nil
		})
		isolateArgs = append(isolateArgs, "-i", Config.Path.Stdin)
	}

	for name, source := range options.InputFilePaths {
		setupEG.Go(func() error {
			destinationPath := filepath.Join(boxRoot, name)
			if err := file.CopyFile(destinationPath, source.Path, source.Perm); err != nil {
				logger.Logger.Error("sandbox: failed to copy input file", "boxId", boxId, "error", err, "sourcePath", source.Path, "destinationPath", destinationPath)
				return fmt.Errorf("failed to copy input file. error: %w", err)
			}
			logger.Logger.Debug("sandbox: copied input file", "boxId", boxId, "sourcePath", source.Path, "destinationPath", destinationPath)
			return nil
		})
	}

	for name, content := range options.InputFileContents {
		setupEG.Go(func() error {
			destinationPath := filepath.Join(boxRoot, name)
			if err := os.WriteFile(destinationPath, content.Content, content.Perm); err != nil {
				logger.Logger.Error("sandbox: failed to write input file", "boxId", boxId, "error", err, "destinationPath", destinationPath)
				return fmt.Errorf("failed to write input file. error: %w", err)
			}
			logger.Logger.Debug("sandbox: wrote input file", "boxId", boxId, "destinationPath", destinationPath)
			return nil
		})
	}

	if err := setupEG.Wait(); err != nil {
		logger.Logger.Error("sandbox: failed to setup sandbox", "boxId", boxId, "error", err)
		return ExecResult{}, fmt.Errorf("failed to setup sandbox: %w", err)
	}

	logger.Logger.Debug("sandbox: setup completed", "boxId", boxId)

	if options.StdioMode == "file" {
		isolateArgs = append(isolateArgs, "-o", Config.Path.Stdout)
		isolateArgs = append(isolateArgs, "--stderr-to-stdout")
	}

	if options.StdioMode == "pipe" {
		isolateArgs = append(isolateArgs, "-s")
	}

	if options.Limit.Time.Real >= 0 {
		isolateArgs = append(isolateArgs, "-t", strconv.FormatFloat(options.Limit.Time.Real.Seconds(), 'f', 3, 64))
	}

	execWallTimeout := options.Limit.Time.Wall
	if execWallTimeout == 0 {
		if options.Limit.Time.Real >= 0 {
			execWallTimeout = max(options.Limit.Time.Real*4, options.Limit.Time.Real+4*time.Second)
		} else {
			execWallTimeout = Config.Timeout.WallDefault
		}
	}
	isolateArgs = append(isolateArgs, "-w", strconv.FormatFloat(execWallTimeout.Seconds(), 'f', 3, 64))

	if options.Limit.Memory >= 0 {
		isolateArgs = append(isolateArgs, "-m", strconv.Itoa((options.Limit.Memory+1023)/1024))
	}

	if options.Limit.Processes >= 0 {
		isolateArgs = append(isolateArgs, "-p"+strconv.Itoa(options.Limit.Processes))
	} else {
		isolateArgs = append(isolateArgs, "-p")
	}

	if options.Limit.OpenFiles >= 0 {
		isolateArgs = append(isolateArgs, "-n", strconv.Itoa(options.Limit.OpenFiles))
	}

	if options.Limit.OutputSize >= 0 {
		isolateArgs = append(isolateArgs, "-f", strconv.Itoa((options.Limit.OutputSize+1023)/1024))
	}

	if len(options.Env) > 0 {
		for key, value := range options.Env {
			isolateArgs = append(isolateArgs, "-E", key+"="+value)
		}
	}

	isolateArgs = append(isolateArgs, "--")
	isolateArgs = append(isolateArgs, command...)

	logger.Logger.Debug("sandbox: executing command", "boxId", boxId, "command", Config.Path.Isolate+" "+strings.Join(isolateArgs, " "))

	execCmdCtx, execCmdCancel := context.WithTimeout(ctx, execWallTimeout+Config.Timeout.Isolate.Exec)
	defer execCmdCancel()

	execCmd := exec.CommandContext(execCmdCtx, Config.Path.Isolate, isolateArgs...)
	if options.StdioMode == "pipe" {
		execCmd.Stdin = bytes.NewReader(options.Stdin)
	} else {
		execCmd.Stdin = nil
	}
	execCmd.WaitDelay = Config.Timeout.WaitDelay
	execOutput, execErr := execCmd.CombinedOutput()

	exitCode := 0
	if execErr != nil {
		exitCode = 1
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	logger.Logger.Debug("sandbox: executed command", "boxId", boxId, "output", execOutput, "exitCode", exitCode)

	teardownEG := errgroup.Group{}

	var stdout []byte

	if options.StdioMode == "pipe" {
		stdout = execOutput
	}

	if options.StdioMode == "file" {
		teardownEG.Go(func() error {
			var err error
			sourcePath := filepath.Join(boxRoot, Config.Path.Stdout)
			stdout, err = os.ReadFile(filepath.Join(boxRoot, Config.Path.Stdout))
			if err != nil {
				stdout = []byte{}
				logger.Logger.Warn("sandbox: failed to read stdout file", "boxId", boxId, "error", err, "sourcePath", sourcePath)
			} else {
				logger.Logger.Debug("sandbox: read stdout file", "boxId", boxId, "sourcePath", sourcePath)
			}
			return nil
		})
	}

	for name, destination := range options.OutputFilePaths {
		teardownEG.Go(func() error {
			sourcePath := filepath.Join(boxRoot, name)
			if err := file.CopyFile(destination.Path, sourcePath, destination.Perm); err != nil {
				logger.Logger.Warn("sandbox: failed to copy output file", "boxId", boxId, "error", err, "sourcePath", sourcePath, "destinationPath", destination.Path)
			} else {
				logger.Logger.Debug("sandbox: copied output file", "boxId", boxId, "sourcePath", sourcePath, "destinationPath", destination.Path)
			}
			return nil
		})
	}

	var metadata ExecResultMeta

	teardownEG.Go(func() error {
		defer func() {
			err := os.Remove(metadataPath)
			if err != nil {
				logger.Logger.Warn("sandbox: failed to delete metadata", "boxId", boxId, "error", err, "metadataPath", metadataPath)
			}
		}()

		var err error
		metadata, err = ParseMetadata(metadataPath)
		if err != nil {
			logger.Logger.Error("sandbox: failed to parse metadata", "boxId", boxId, "error", err, "metadataPath", metadataPath)
			return fmt.Errorf("failed to parse metadata: %w", err)
		}
		logger.Logger.Debug("sandobx: parsed metadata", "boxId", boxId, "metadataPath", metadataPath)
		return nil
	})

	if err := teardownEG.Wait(); err != nil {
		logger.Logger.Error("sandbox: failed to teardown sandbox", "boxId", boxId, "error", err)
		return ExecResult{}, fmt.Errorf("failed to teardown sandbox: %w", err)
	}

	logger.Logger.Debug("sandbox: teardown completed", "boxId", boxId)

	return ExecResult{
		ExitCode:      exitCode,
		Output:        stdout,
		IsolateOutput: execOutput,
		Metadata:      metadata,
	}, nil
}
