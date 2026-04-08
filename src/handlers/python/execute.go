package python

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Oudwins/zog"
	"github.com/Oudwins/zog/parsers/zjson"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"next-gen-ai-web-application/executor/src/sandbox"
	"next-gen-ai-web-application/executor/src/utils/logger"
)

func Execute(ctx fiber.Ctx) error {
	execData := ExecData{}

	if errs := ExecDataSchema.Parse(zjson.Decode(bytes.NewReader(ctx.Body())), &execData); errs != nil {
		if errs[0].Code == "invalid_json" {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"errors":     []string{fmt.Sprintf("JSON parse error: %s", errs[0].Err.Error())},
				"properties": fiber.Map{},
			})
		}
		return ctx.Status(fiber.StatusUnprocessableEntity).JSON(zog.Issues.Treeify(errs))
	}

	pythonPath := strings.ReplaceAll(Config.Path.Python, "{version}", execData.Version)

	result := ExecResult{}
	result.StdioSets = make([][]ExecResultOutput, len(execData.StdioSets))
	result.UnitSets = make([][]ExecResultTest, len(execData.UnitSets))
	for i := range execData.StdioSets {
		result.StdioSets[i] = make([]ExecResultOutput, len(execData.StdioSets[i].Cases))
	}
	for i := range execData.UnitSets {
		result.UnitSets[i] = make([]ExecResultTest, len(execData.UnitSets[i].Cases))
	}

	env := map[string]string{}
	if execData.ColoredDiagnostics {
		env["PYTHON_COLORS"] = "1"
		env["PY_COLORS"] = "1"
		env["FORCE_COLOR"] = "1"
		env["PY_FORCE_COLOR"] = "1"
	} else {
		env["PYTHON_COLORS"] = "0"
		env["PY_COLORS"] = "0"
		env["NO_COLOR"] = "1"
	}

	wg := sync.WaitGroup{}

	for setIdx, setData := range execData.StdioSets {
		if setData.Limit.Time.Wall == 0 {
			setData.Limit.Time.Wall = max(setData.Limit.Time.Real*4, setData.Limit.Time.Real+4)
		}

		switch setData.IsolationLevel {
		case "medium":
			for start := 0; start < len(setData.Cases); start += Config.BatchSize {
				end := min(start+Config.BatchSize, len(setData.Cases))
				count := end - start
				wg.Go(func() {
					tempFolder := filepath.Join(sandbox.Config.Path.Temp, uuid.NewString())

					defer func() {
						if err := os.RemoveAll(tempFolder); err != nil {
							logger.Logger.Warn("handlers/python/stdio: failed to delete temporary folder", "isolationLevel", "medium", "error", err, "tempFolder", tempFolder)
						}
					}()

					runnerConfig := BatchStdioRunnerConfig{
						Submission: "submission.py",
						Cases:      make([]BatchStdioRunnerCase, count),
					}

					inputFileContents := make(map[string]sandbox.ExecFileContent, count+2)
					outputFilePaths := make(map[string]sandbox.ExecFilePath, count*2)

					for caseIdx, caseData := range setData.Cases[start:end] {
						stdinName := "__" + strconv.Itoa(caseIdx) + "__stdin__"
						stdoutName := "__" + strconv.Itoa(caseIdx) + "__stdout__"
						metadataName := "__" + strconv.Itoa(caseIdx) + "__metadata__"

						if caseData.Limit.Time.Wall == 0 {
							caseData.Limit.Time.Wall = max(caseData.Limit.Time.Real*4, caseData.Limit.Time.Real+4)
						}
						if caseData.Limit.OpenFiles > 0 {
							caseData.Limit.OpenFiles++
						}

						runnerConfig.Cases[caseIdx] = BatchStdioRunnerCase{
							Input:    stdinName,
							Output:   stdoutName,
							Metadata: metadataName,
							Limit:    caseData.Limit,
						}

						inputFileContents[stdinName] = sandbox.ExecFileContent{
							Content: []byte(caseData.Input),
							Perm:    0644,
						}
						outputFilePaths[stdoutName] = sandbox.ExecFilePath{
							Path: filepath.Join(tempFolder, stdoutName),
							Perm: 0644,
						}
						outputFilePaths[metadataName] = sandbox.ExecFilePath{
							Path: filepath.Join(tempFolder, metadataName),
							Perm: 0644,
						}
					}

					runnerConfigJson, err := json.Marshal(runnerConfig)
					if err != nil {
						logger.Logger.Error("handlers/python/stdio: failed to serialize data to JSON", "isolationLevel", "medium", "error", err)
						for caseIdx := range setData.Cases[start:end] {
							realCaseIdx := caseIdx + start
							result.StdioSets[setIdx][realCaseIdx] = ExecResultOutput{
								Status: "IE",
							}
						}
						return
					}

					maps.Copy(inputFileContents, map[string]sandbox.ExecFileContent{
						"submission.py": {
							Content: []byte(execData.Submission),
							Perm:    0644,
						},
						"config.json": {
							Content: runnerConfigJson,
							Perm:    0644,
						},
					})

					_, err = sandbox.Execute(
						ctx,
						[]string{pythonPath, "-IBu", "runner.py"},
						sandbox.ExecOptions{
							Env:               env,
							InputFileContents: inputFileContents,
							InputFilePaths: map[string]sandbox.ExecFilePath{
								"runner.py": {
									Path: Config.Path.BatchStdioRunner,
									Perm: 0644,
								},
							},
							OutputFilePaths: outputFilePaths,
							Limit: sandbox.ExecLimit{
								Time: sandbox.ExecLimitTime{
									Real: time.Duration(setData.Limit.Time.Real * float64(time.Second)),
									Wall: time.Duration(setData.Limit.Time.Wall * float64(time.Second)),
								},
								Memory:     setData.Limit.Memory,
								Processes:  setData.Limit.Processes,
								OpenFiles:  setData.Limit.OpenFiles,
								OutputSize: setData.Limit.OutputSize,
							},
						},
					)

					setStatus := "OK"

					if err != nil {
						setStatus = "IE" // Internal Error
					}

					for caseIdx, caseData := range setData.Cases[start:end] {
						realCaseIdx := caseIdx + start

						stdoutName := "__" + strconv.Itoa(caseIdx) + "__stdout__"
						metadataName := "__" + strconv.Itoa(caseIdx) + "__metadata__"

						stdout, _ := os.ReadFile(filepath.Join(tempFolder, stdoutName))
						metadata, metadataErr := sandbox.ParseMetadata(filepath.Join(tempFolder, metadataName))

						caseStatus := setStatus

						if metadataErr != nil {
							caseStatus = "IE" // Internal Error
							logger.Logger.Error("handlers/python/stdio: failed to parse metadata", "isolationLevel", "medium", "error", metadataErr, "metadataPath", filepath.Join(tempFolder, metadataName))
						} else if metadata.Status == "XX" {
							caseStatus = "IE" // Internal Error
						} else if metadata.Status == "TO" ||
							(caseData.Limit.Time.Real > 0 && metadata.Time.Real.Seconds() > caseData.Limit.Time.Real) ||
							(caseData.Limit.Time.Wall > 0 && metadata.Time.Wall.Seconds() > caseData.Limit.Time.Wall) {
							caseStatus = "TLE" // Time Limit Exceeded
						} else if caseData.Limit.Memory > 0 &&
							(metadata.Memory > caseData.Limit.Memory ||
								metadata.ExitCode != 0 && metadata.Memory > min(caseData.Limit.Memory*8/10, caseData.Limit.Memory-2*1024*1024)) {
							caseStatus = "MLE" // Memory Limit Exceeded
						} else if caseData.Limit.OutputSize > 0 && metadata.ExitCode != 0 && len(stdout) >= caseData.Limit.OutputSize {
							caseStatus = "OLE" // Output Limit Exceeded
						} else if metadata.ExitSignal != 0 || metadata.ExitCode != 0 {
							caseStatus = "RE" // Runtime Error
						}

						result.StdioSets[setIdx][realCaseIdx] = ExecResultOutput{
							Output:     string(stdout),
							Status:     caseStatus,
							ExitCode:   metadata.ExitCode,
							ExitSignal: metadata.ExitSignal,
							Time:       metadata.Time.Real.Seconds(),
							Memory:     metadata.Memory,
						}
					}
				})
			}
		case "high":
			for caseIdx, caseData := range setData.Cases {
				wg.Go(func() {
					if caseData.Limit.Time.Wall == 0 {
						caseData.Limit.Time.Wall = max(caseData.Limit.Time.Real*4, caseData.Limit.Time.Real+4)
					}

					execResult, err := sandbox.Execute(
						ctx,
						[]string{pythonPath, "-IBu", "submission.py"},
						sandbox.ExecOptions{
							Stdin:     []byte(caseData.Input),
							StdioMode: "file",
							Env:       env,
							InputFileContents: map[string]sandbox.ExecFileContent{
								"submission.py": {
									Content: []byte(execData.Submission),
									Perm:    0644,
								},
							},
							Limit: sandbox.ExecLimit{
								Time: sandbox.ExecLimitTime{
									Real: time.Duration(caseData.Limit.Time.Real * float64(time.Second)),
									Wall: time.Duration(caseData.Limit.Time.Wall * float64(time.Second)),
								},
								Memory:     caseData.Limit.Memory,
								Processes:  caseData.Limit.Processes,
								OpenFiles:  caseData.Limit.OpenFiles,
								OutputSize: caseData.Limit.OutputSize,
							},
						},
					)

					status := "OK"

					if err != nil {
						status = "IE" // Internal Error
					} else if execResult.Metadata.Status == "XX" {
						status = "IE" // Internal Error
					} else if execResult.Metadata.Status == "TO" ||
						(caseData.Limit.Time.Real > 0 && execResult.Metadata.Time.Real.Seconds() > caseData.Limit.Time.Real) ||
						(caseData.Limit.Time.Wall > 0 && execResult.Metadata.Time.Wall.Seconds() > caseData.Limit.Time.Wall) {
						status = "TLE" // Time Limit Exceeded
					} else if caseData.Limit.Memory > 0 &&
						(execResult.Metadata.Memory > caseData.Limit.Memory ||
							execResult.Metadata.ExitCode != 0 && execResult.Metadata.Memory > min(caseData.Limit.Memory*8/10, caseData.Limit.Memory-2*1024*1024)) {
						status = "MLE" // Memory Limit Exceeded
					} else if caseData.Limit.OutputSize > 0 && execResult.Metadata.ExitCode != 0 && len(execResult.Output) >= caseData.Limit.OutputSize {
						status = "OLE" // Output Limit Exceeded
					} else if execResult.Metadata.ExitSignal != 0 || execResult.Metadata.ExitCode != 0 {
						status = "RE" // Runtime Error
					}

					result.StdioSets[setIdx][caseIdx] = ExecResultOutput{
						Output:     string(execResult.Output),
						Status:     status,
						ExitCode:   execResult.Metadata.ExitCode,
						ExitSignal: execResult.Metadata.ExitSignal,
						Time:       execResult.Metadata.Time.Real.Seconds(),
						Memory:     execResult.Metadata.Memory,
					}
				})
			}
		}
	}

	for setIdx, setData := range execData.UnitSets {
		if setData.Limit.Time.Wall == 0 {
			setData.Limit.Time.Wall = max(setData.Limit.Time.Real*4, setData.Limit.Time.Real+4)
		}

		switch setData.IsolationLevel {
		case "low":
			if len(setData.Cases) > 0 {
				wg.Go(func() {
					testScriptBuilder := strings.Builder{}
					testScriptBuilder.WriteString("import submission\n\n")

					for caseIdx, caseData := range setData.Cases {
						testScriptBuilder.WriteString("\n")
						fmt.Fprintf(&testScriptBuilder, "def test_%d():\n", caseIdx)
						for line := range strings.SplitSeq(caseData.Script, "\n") {
							line = strings.TrimSuffix(line, "\r")
							testScriptBuilder.WriteString("    ")
							testScriptBuilder.WriteString(line)
							testScriptBuilder.WriteString("\n")
						}
					}

					testScript := testScriptBuilder.String()
					reportPath := filepath.Join(sandbox.Config.Path.Temp, uuid.NewString()+".json")

					defer func() {
						if err := os.Remove(reportPath); err != nil {
							logger.Logger.Warn("handlers/python/unit: failed to delete temporary file", "isolationLevel", "low", "error", err, "reportPath", reportPath)
						}
					}()

					inputFilePaths := map[string]sandbox.ExecFilePath{}
					if execData.ColoredDiagnostics {
						inputFilePaths["conftest.py"] = sandbox.ExecFilePath{

							Path: Config.Path.BatchUnitColorConfTest,
							Perm: 0644,
						}
					}

					execResult, err := sandbox.Execute(
						ctx,
						[]string{
							pythonPath,
							"-IBu", "-m", "pytest",
							"-p", "no:cacheprovider",
							"--json-report", "--json-report-file=report.json",
							"--json-report-omit", "collectors", "log", "traceback", "streams", "warnings", "keywords",
							"submission_test.py",
						},
						sandbox.ExecOptions{
							InputFileContents: map[string]sandbox.ExecFileContent{
								"submission.py": {
									Content: []byte(execData.Submission),
									Perm:    0644,
								},
								"submission_test.py": {
									Content: []byte(testScript),
									Perm:    0644,
								},
							},
							InputFilePaths: inputFilePaths,
							OutputFilePaths: map[string]sandbox.ExecFilePath{
								"report.json": {
									Path: reportPath,
									Perm: 0640,
								},
							},
							Limit: sandbox.ExecLimit{
								Time: sandbox.ExecLimitTime{
									Real: time.Duration(setData.Limit.Time.Real * float64(time.Second)),
									Wall: time.Duration(setData.Limit.Time.Wall * float64(time.Second)),
								},
								Memory:     setData.Limit.Memory,
								Processes:  setData.Limit.Processes,
								OpenFiles:  setData.Limit.OpenFiles,
								OutputSize: setData.Limit.OutputSize,
							},
						},
					)

					setStatus := "OK"

					if execResult.Metadata.Status == "XX" {
						setStatus = "IE" // Internal Error
					} else if execResult.Metadata.Status == "TO" ||
						(setData.Limit.Time.Real > 0 && execResult.Metadata.Time.Real.Seconds() > setData.Limit.Time.Real) ||
						(setData.Limit.Time.Wall > 0 && execResult.Metadata.Time.Wall.Seconds() > setData.Limit.Time.Wall) {
						setStatus = "TLE" // Time Limit Exceeded
					} else if setData.Limit.Memory > 0 &&
						(execResult.Metadata.Memory > setData.Limit.Memory ||
							execResult.Metadata.ExitCode != 0 && execResult.Metadata.Memory > min(setData.Limit.Memory*8/10, setData.Limit.Memory-2*1024*1024)) {
						setStatus = "MLE" // Memory Limit Exceeded
					} else if setData.Limit.OutputSize > 0 && execResult.Metadata.ExitCode != 0 && len(execResult.Output) >= setData.Limit.OutputSize {
						setStatus = "OLE" // Output Limit Exceeded
					}

					if setStatus != "OK" {
						for caseIdx := range setData.Cases {
							result.UnitSets[setIdx][caseIdx] = ExecResultTest{
								Status: setStatus,
							}
						}
						return
					}

					content, err := os.ReadFile(reportPath)
					if err != nil {
						logger.Logger.Error("handlers/python/unit: failed to read report data", "isolationLevel", "low", "error", err, "reportPath", reportPath)
						setStatus = "IE"
					}

					report := PytestReport{}
					if err := json.Unmarshal(content, &report); err != nil {
						logger.Logger.Error("handlers/python/unit/w: failed to parse report data", "isolationLevel", "low", "error", err, "rawReport", string(content))
						setStatus = "IE"
					}

					if len(report.Tests) != len(setData.Cases) {
						logger.Logger.Error("handlers/python/unit: report data length mismatch", "isolationLevel", "low", "error", err, "rawReport", string(content))
						setStatus = "IE"
					}

					if setStatus != "OK" {
						for caseIdx := range setData.Cases {
							result.UnitSets[setIdx][caseIdx] = ExecResultTest{
								Status: setStatus,
							}
						}
						return
					}

					for caseIdx := range setData.Cases {
						caseReport := report.Tests[caseIdx]

						output := ""
						passed := caseReport.Outcome == "passed"
						status := "OK"
						exitCode := 0
						exitSignal := 0

						if !passed {
							outputs := make([]string, 0)
							if len(caseReport.Setup.LongRepr) > 0 {
								outputs = append(outputs, caseReport.Setup.LongRepr)
							}
							if len(caseReport.Call.LongRepr) > 0 {
								outputs = append(outputs, caseReport.Call.LongRepr)
							}
							if len(caseReport.Teardown.LongRepr) > 0 {
								outputs = append(outputs, caseReport.Teardown.LongRepr)
							}
							output = strings.Join(outputs, "\n\n")
							output = strings.TrimLeft(output, "\n")
							status = "RE"
							exitCode = 1
						}

						result.UnitSets[setIdx][caseIdx] = ExecResultTest{
							Output:     output,
							Passed:     passed,
							Status:     status,
							ExitCode:   exitCode,
							ExitSignal: exitSignal,
							Time:       caseReport.Setup.Duration + caseReport.Call.Duration + caseReport.Teardown.Duration,
						}
					}
				})
			}
		case "medium":
			for start := 0; start < len(setData.Cases); start += Config.BatchSize {
				end := min(start+Config.BatchSize, len(setData.Cases))
				count := end - start
				wg.Go(func() {
					tempFolder := filepath.Join(sandbox.Config.Path.Temp, uuid.NewString())

					defer func() {
						if err := os.RemoveAll(tempFolder); err != nil {
							logger.Logger.Warn("handlers/python/unit: failed to delete temporary folder", "isolationLevel", "medium", "error", err, "tempFolder", tempFolder)
						}
					}()

					runnerConfig := BatchUnitRunnerConfig{
						Submission: "submission.py",
						Cases:      make([]BatchUnitRunnerCase, count),
					}

					testScriptBuilder := strings.Builder{}
					testScriptBuilder.WriteString("import submission\n\n")

					outputFilePaths := make(map[string]sandbox.ExecFilePath, count*2)

					for caseIdx, caseData := range setData.Cases[start:end] {
						testScriptBuilder.WriteString("\n")
						fmt.Fprintf(&testScriptBuilder, "def test_%d():\n", caseIdx)
						for line := range strings.SplitSeq(caseData.Script, "\n") {
							line = strings.TrimSuffix(line, "\r")
							testScriptBuilder.WriteString("    ")
							testScriptBuilder.WriteString(line)
							testScriptBuilder.WriteString("\n")
						}

						stdoutName := "__" + strconv.Itoa(caseIdx) + "__stdout__"
						metadataName := "__" + strconv.Itoa(caseIdx) + "__metadata__"

						if caseData.Limit.Time.Wall == 0 {
							caseData.Limit.Time.Wall = max(caseData.Limit.Time.Real*4, caseData.Limit.Time.Real+4)
						}
						if caseData.Limit.OpenFiles > 0 {
							caseData.Limit.OpenFiles++
						}

						runnerConfig.Cases[caseIdx] = BatchUnitRunnerCase{
							Name:     "test_" + strconv.Itoa(caseIdx),
							Output:   stdoutName,
							Metadata: metadataName,
							Limit:    caseData.Limit,
						}

						outputFilePaths[stdoutName] = sandbox.ExecFilePath{
							Path: filepath.Join(tempFolder, stdoutName),
							Perm: 0644,
						}
						outputFilePaths[metadataName] = sandbox.ExecFilePath{
							Path: filepath.Join(tempFolder, metadataName),
							Perm: 0644,
						}
					}

					runnerConfigJson, err := json.Marshal(runnerConfig)
					if err != nil {
						logger.Logger.Error("handlers/python/unit: failed to serialize data to JSON", "isolationLevel", "medium", "error", err)
						for caseIdx := range setData.Cases[start:end] {
							realCaseIdx := caseIdx + start
							result.UnitSets[setIdx][realCaseIdx] = ExecResultTest{
								Status: "IE",
							}
						}
						return
					}

					testScript := testScriptBuilder.String()

					_, err = sandbox.Execute(
						ctx,
						[]string{pythonPath, "-IBu", "runner.py"},
						sandbox.ExecOptions{
							Env: env,
							InputFileContents: map[string]sandbox.ExecFileContent{
								"submission.py": {
									Content: []byte(execData.Submission),
									Perm:    0644,
								},
								"submission_test.py": {
									Content: []byte(testScript),
									Perm:    0644,
								},
								"config.json": {
									Content: runnerConfigJson,
									Perm:    0644,
								},
							},
							InputFilePaths: map[string]sandbox.ExecFilePath{
								"runner.py": {
									Path: Config.Path.BatchUnitRunner,
									Perm: 0644,
								},
							},
							OutputFilePaths: outputFilePaths,
							Limit: sandbox.ExecLimit{
								Time: sandbox.ExecLimitTime{
									Real: time.Duration(setData.Limit.Time.Real * float64(time.Second)),
									Wall: time.Duration(setData.Limit.Time.Wall * float64(time.Second)),
								},
								Memory:     setData.Limit.Memory,
								Processes:  setData.Limit.Processes,
								OpenFiles:  setData.Limit.OpenFiles,
								OutputSize: setData.Limit.OutputSize,
							},
						},
					)

					setStatus := "OK"

					if err != nil {
						setStatus = "IE" // Internal Error
					}

					for caseIdx, caseData := range setData.Cases[start:end] {
						realCaseIdx := caseIdx + start

						stdoutName := "__" + strconv.Itoa(caseIdx) + "__stdout__"
						metadataName := "__" + strconv.Itoa(caseIdx) + "__metadata__"

						stdout, _ := os.ReadFile(filepath.Join(tempFolder, stdoutName))
						metadata, metadataErr := sandbox.ParseMetadata(filepath.Join(tempFolder, metadataName))

						caseStatus := setStatus

						if metadataErr != nil {
							caseStatus = "IE" // Internal Error
							logger.Logger.Error("handlers/python/unit: failed to parse metadata", "isolationLevel", "medium", "error", metadataErr, "metadataPath", filepath.Join(tempFolder, metadataName))
						} else if metadata.Status == "XX" {
							caseStatus = "IE" // Internal Error
						} else if metadata.Status == "TO" ||
							(caseData.Limit.Time.Real > 0 && metadata.Time.Real.Seconds() > caseData.Limit.Time.Real) ||
							(caseData.Limit.Time.Wall > 0 && metadata.Time.Wall.Seconds() > caseData.Limit.Time.Wall) {
							caseStatus = "TLE" // Time Limit Exceeded
						} else if caseData.Limit.Memory > 0 &&
							(metadata.Memory > caseData.Limit.Memory ||
								metadata.ExitCode != 0 && metadata.Memory > min(caseData.Limit.Memory*8/10, caseData.Limit.Memory-2*1024*1024)) {
							caseStatus = "MLE" // Memory Limit Exceeded
						} else if caseData.Limit.OutputSize > 0 && metadata.ExitCode != 0 && len(stdout) >= caseData.Limit.OutputSize {
							caseStatus = "OLE" // Output Limit Exceeded
						} else if metadata.ExitSignal != 0 || metadata.ExitCode != 0 {
							caseStatus = "RE" // Runtime Error
						}

						result.UnitSets[setIdx][realCaseIdx] = ExecResultTest{
							Output:     string(stdout),
							Passed:     caseStatus == "OK" && metadata.ExitCode == 0,
							Status:     caseStatus,
							ExitCode:   metadata.ExitCode,
							ExitSignal: metadata.ExitSignal,
							Time:       metadata.Time.Real.Seconds(),
							Memory:     metadata.Memory,
						}
					}
				})
			}
		case "high":
			for caseIdx, caseData := range setData.Cases {
				wg.Go(func() {
					if caseData.Limit.Time.Wall == 0 {
						caseData.Limit.Time.Wall = max(caseData.Limit.Time.Real*4, caseData.Limit.Time.Real+4)
					}

					testScriptBuilder := strings.Builder{}
					testScriptBuilder.WriteString("import submission\n\n\n")
					testScriptBuilder.WriteString("def test():\n")
					for line := range strings.SplitSeq(caseData.Script, "\n") {
						line = strings.TrimSuffix(line, "\r")
						testScriptBuilder.WriteString("    ")
						testScriptBuilder.WriteString(line)
						testScriptBuilder.WriteString("\n")
					}

					testScript := testScriptBuilder.String()

					execResult, err := sandbox.Execute(
						ctx,
						[]string{pythonPath, "-IBu", "-m", "pytest", "-p", "no:cacheprovider", "submission_test.py"},
						sandbox.ExecOptions{
							StdioMode: "file",
							Env:       env,
							InputFileContents: map[string]sandbox.ExecFileContent{
								"submission.py": {
									Content: []byte(execData.Submission),
									Perm:    0644,
								},
								"submission_test.py": {
									Content: []byte(testScript),
									Perm:    0644,
								},
							},
							Limit: sandbox.ExecLimit{
								Time: sandbox.ExecLimitTime{
									Real: time.Duration(caseData.Limit.Time.Real * float64(time.Second)),
									Wall: time.Duration(caseData.Limit.Time.Wall * float64(time.Second)),
								},
								Memory:     caseData.Limit.Memory,
								Processes:  caseData.Limit.Processes,
								OpenFiles:  caseData.Limit.OpenFiles,
								OutputSize: caseData.Limit.OutputSize,
							},
						},
					)

					status := "OK"

					if err != nil {
						status = "IE" // Internal Error
					} else if execResult.Metadata.Status == "XX" {
						status = "IE" // Internal Error
					} else if execResult.Metadata.Status == "TO" ||
						(caseData.Limit.Time.Real > 0 && execResult.Metadata.Time.Real.Seconds() > caseData.Limit.Time.Real) ||
						(caseData.Limit.Time.Wall > 0 && execResult.Metadata.Time.Wall.Seconds() > caseData.Limit.Time.Wall) {
						status = "TLE" // Time Limit Exceeded
					} else if caseData.Limit.Memory > 0 &&
						(execResult.Metadata.Memory > caseData.Limit.Memory ||
							execResult.Metadata.ExitCode != 0 && execResult.Metadata.Memory > min(caseData.Limit.Memory*8/10, caseData.Limit.Memory-2*1024*1024)) {
						status = "MLE" // Memory Limit Exceeded
					} else if caseData.Limit.OutputSize > 0 && execResult.Metadata.ExitCode != 0 && len(execResult.Output) >= caseData.Limit.OutputSize {
						status = "OLE" // Output Limit Exceeded
					} else if execResult.Metadata.ExitSignal != 0 || execResult.Metadata.ExitCode != 0 {
						status = "RE" // Runtime Error
					}

					result.UnitSets[setIdx][caseIdx] = ExecResultTest{
						Output:     string(execResult.Output),
						Passed:     status == "OK" && execResult.Metadata.ExitCode == 0,
						Status:     status,
						ExitCode:   execResult.Metadata.ExitCode,
						ExitSignal: execResult.Metadata.ExitSignal,
						Time:       execResult.Metadata.Time.Real.Seconds(),
						Memory:     execResult.Metadata.Memory,
					}
				})
			}
		}
	}

	wg.Wait()

	return ctx.Status(fiber.StatusOK).JSON(result)
}
