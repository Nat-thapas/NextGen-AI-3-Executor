package python

import (
	"fmt"

	"github.com/Oudwins/zog"
)

type ExecLimitTime struct {
	Real float64 `zog:"real" json:"real"`
	Wall float64 `zog:"wall" json:"wall"`
}

type ExecLimit struct {
	Time       ExecLimitTime `zog:"time" json:"time"`
	Memory     int           `zog:"memory" json:"memory"`
	Processes  int           `zog:"processes" json:"processes"`
	OpenFiles  int           `zog:"open_files" json:"open_files"`
	OutputSize int           `zog:"output_size" json:"output_size"`
}

type ExecStdioSet struct {
	// - medium: isolated batch execution
	// - high: individual execution
	IsolationLevel string    `zog:"isolation_level"`
	Limit          ExecLimit `zog:"limit"`
	Cases          []struct {
		Input string    `zog:"input"`
		Limit ExecLimit `zog:"limit"`
	} `zog:"cases"`
}

type ExecUnitSet struct {
	// - low: batch execution
	// - high: individual execution
	IsolationLevel string    `zog:"isolation_level"`
	Limit          ExecLimit `zog:"limit"`
	Cases          []struct {
		Script string    `zog:"script"`
		Limit  ExecLimit `zog:"limit"`
	} `zog:"cases"`
}

type ExecData struct {
	Version            string         `zog:"version"`
	Submission         string         `zog:"submission"`
	ColoredDiagnostics bool           `zog:"colored_diagnostics"`
	StdioSets          []ExecStdioSet `zog:"stdio_sets"`
	UnitSets           []ExecUnitSet  `zog:"unit_sets"`
}

type ExecResultOutput struct {
	Output     string  `json:"output"`
	Status     string  `json:"status"`
	ExitCode   int     `json:"exit_code"`
	ExitSignal int     `json:"exit_signal"`
	Time       float64 `json:"time"`
	Memory     int     `json:"memory"`
}

type ExecResultTest struct {
	Output     string  `json:"output"`
	Passed     bool    `json:"passed"`
	Status     string  `json:"status"`
	ExitCode   int     `json:"exit_code"`
	ExitSignal int     `json:"exit_signal"`
	Time       float64 `json:"time"`
	Memory     int     `json:"memory"`
}

type ExecResult struct {
	StdioSets [][]ExecResultOutput `json:"outputs"`
	UnitSets  [][]ExecResultTest   `json:"tests"`
}

type BatchStdioRunnerCase struct {
	Input    string    `json:"input"`
	Output   string    `json:"output"`
	Metadata string    `json:"metadata"`
	Limit    ExecLimit `json:"limit"`
}

type BatchStdioRunnerConfig struct {
	Submission string                 `json:"submission"`
	Cases      []BatchStdioRunnerCase `json:"cases"`
}

type BatchUnitRunnerCase struct {
	Name     string    `json:"name"`
	Output   string    `json:"output"`
	Metadata string    `json:"metadata"`
	Limit    ExecLimit `json:"limit"`
}

type BatchUnitRunnerConfig struct {
	Submission string                `json:"submission"`
	Cases      []BatchUnitRunnerCase `json:"cases"`
}

type PytestReportSummary struct {
	Failed    int `json:"failed"`
	Passed    int `json:"passed"`
	Total     int `json:"total"`
	Collected int `json:"collected"`
}

type PytestReportStageCrash struct {
	Path    string `json:"path"`
	LineNo  int    `json:"lineno"`
	Message string `json:"message"`
}

type PytestReportStage struct {
	Duration float64                `json:"duration"`
	Outcome  string                 `json:"outcome"`
	Crash    PytestReportStageCrash `json:"crash"`
	LongRepr string                 `json:"longrepr"`
}

type PytestReportTest struct {
	NodeId   string            `json:"nodeid"`
	LineNo   int               `json:"lineno"`
	Outcome  string            `json:"outcome"`
	Setup    PytestReportStage `json:"setup"`
	Call     PytestReportStage `json:"call"`
	Teardown PytestReportStage `json:"teardown"`
}

type PytestReport struct {
	Created      float64             `json:"created"`
	Duration     float64             `json:"duration"`
	ExitCode     int                 `json:"exitcode"`
	Root         string              `json:"root"`
	Environtment map[string]string   `json:"environment"`
	Summary      PytestReportSummary `json:"summary"`
	Tests        []PytestReportTest  `json:"tests"`
}

var execStdioIsolatedLimitSchema = zog.Struct(zog.Shape{
	"time": zog.Struct(zog.Shape{
		"real": zog.Float64().
			Default(Config.Limit.Stdio.Default.Isolated.Time.Real).
			GT(0, zog.Message("stdio test execution time limit must be positive")).
			LTE(
				Config.Limit.Stdio.Max.Isolated.Time.Real,
				zog.Message(fmt.Sprintf("stdio test execution time limit must be at most %g", Config.Limit.Stdio.Max.Isolated.Time.Real)),
			),
		"wall": zog.Float64().
			Default(Config.Limit.Stdio.Default.Isolated.Time.Wall).
			GTE(0, zog.Message("stdio test execution wall-clock time limit must be positive")).
			LTE(
				Config.Limit.Stdio.Max.Isolated.Time.Wall,
				zog.Message(fmt.Sprintf("stdio test execution wall-clock time limit must be at most %g", Config.Limit.Stdio.Max.Isolated.Time.Wall)),
			),
	}),
	"memory": zog.Int().
		Default(Config.Limit.Stdio.Default.Isolated.Memory).
		GT(0, zog.Message("stdio test execution memory limit must be positive")).
		LTE(
			Config.Limit.Stdio.Max.Isolated.Memory,
			zog.Message(fmt.Sprintf("stdio test execution memory limit must be at most %d", Config.Limit.Stdio.Max.Isolated.Memory)),
		),
	"processes": zog.Int().
		Default(Config.Limit.Stdio.Default.Isolated.Processes).
		GT(0, zog.Message("stdio test execution processes limit must be positive")).
		LTE(
			Config.Limit.Stdio.Max.Isolated.Processes,
			zog.Message(fmt.Sprintf("stdio test execution processes limit must be at most %d", Config.Limit.Stdio.Max.Isolated.Processes)),
		),
	"openFiles": zog.Int().
		Default(Config.Limit.Stdio.Default.Isolated.OpenFiles).
		GT(0, zog.Message("stdio test execution open files limit must be positive")).
		LTE(
			Config.Limit.Stdio.Max.Isolated.OpenFiles,
			zog.Message(fmt.Sprintf("stdio test execution open files limit must be at most %d", Config.Limit.Stdio.Max.Isolated.OpenFiles)),
		),
	"outputSize": zog.Int().
		Default(Config.Limit.Stdio.Default.Isolated.OutputSize).
		GT(0, zog.Message("stdio test execution output size limit must be positive")).
		LTE(
			Config.Limit.Stdio.Max.Isolated.OutputSize,
			zog.Message(fmt.Sprintf("stdio test execution output size limit must be at most %d", Config.Limit.Stdio.Max.Isolated.OutputSize)),
		),
})

var execStdioBatchLimitSchema = zog.Struct(zog.Shape{
	"time": zog.Struct(zog.Shape{
		"real": zog.Float64().
			Default(Config.Limit.Stdio.Default.Batch.Time.Real).
			GT(0, zog.Message("batch stdio test execution time limit must be positive")).
			LTE(
				Config.Limit.Stdio.Max.Batch.Time.Real,
				zog.Message(fmt.Sprintf("batch stdio test execution time limit must be at most %g", Config.Limit.Stdio.Max.Batch.Time.Real)),
			),
		"wall": zog.Float64().
			Default(Config.Limit.Stdio.Default.Batch.Time.Wall).
			GTE(0, zog.Message("batch stdio test execution wall-clock time limit must be positive or zero")).
			LTE(
				Config.Limit.Stdio.Max.Batch.Time.Wall,
				zog.Message(fmt.Sprintf("batch stdio test execution wall-clock time limit must be at most %g", Config.Limit.Stdio.Max.Batch.Time.Wall)),
			),
	}),
	"memory": zog.Int().
		Default(Config.Limit.Stdio.Default.Batch.Memory).
		GT(0, zog.Message("batch stdio test execution memory limit must be positive")).
		LTE(
			Config.Limit.Stdio.Max.Batch.Memory,
			zog.Message(fmt.Sprintf("batch stdio test execution memory limit must be at most %d", Config.Limit.Stdio.Max.Batch.Memory)),
		),
	"processes": zog.Int().
		Default(Config.Limit.Stdio.Default.Batch.Processes).
		GT(0, zog.Message("batch stdio test execution processes limit must be positive")).
		LTE(
			Config.Limit.Stdio.Max.Batch.Processes,
			zog.Message(fmt.Sprintf("batch stdio test execution processes limit must be at most %d", Config.Limit.Stdio.Max.Batch.Processes)),
		),
	"openFiles": zog.Int().
		Default(Config.Limit.Stdio.Default.Batch.OpenFiles).
		GT(0, zog.Message("batch stdio test execution open files limit must be positive")).
		LTE(
			Config.Limit.Stdio.Max.Batch.OpenFiles,
			zog.Message(fmt.Sprintf("batch stdio test execution open files limit must be at most %d", Config.Limit.Stdio.Max.Batch.OpenFiles)),
		),
	"outputSize": zog.Int().
		Default(Config.Limit.Stdio.Default.Batch.OutputSize).
		GT(0, zog.Message("batch stdio test execution output size limit must be positive")).
		LTE(
			Config.Limit.Stdio.Max.Batch.OutputSize,
			zog.Message(fmt.Sprintf("batch stdio test execution output size limit must be at most %d", Config.Limit.Stdio.Max.Batch.OutputSize)),
		),
})

var execUnitIsolatedLimitSchema = zog.Struct(zog.Shape{
	"time": zog.Struct(zog.Shape{
		"real": zog.Float64().
			Default(Config.Limit.Unit.Default.Isolated.Time.Real).
			GT(0, zog.Message("unit test execution time limit must be positive")).
			LTE(
				Config.Limit.Unit.Max.Isolated.Time.Real,
				zog.Message(fmt.Sprintf("unit test execution time limit must be at most %g", Config.Limit.Unit.Max.Isolated.Time.Real)),
			),
		"wall": zog.Float64().
			Default(Config.Limit.Unit.Default.Isolated.Time.Wall).
			GTE(0, zog.Message("unit test execution wall-clock time limit must be positive")).
			LTE(
				Config.Limit.Unit.Max.Isolated.Time.Wall,
				zog.Message(fmt.Sprintf("unit test execution wall-clock time limit must be at most %g", Config.Limit.Unit.Max.Isolated.Time.Wall)),
			),
	}),
	"memory": zog.Int().
		Default(Config.Limit.Unit.Default.Isolated.Memory).
		GT(0, zog.Message("unit test execution memory limit must be positive")).
		LTE(
			Config.Limit.Unit.Max.Isolated.Memory,
			zog.Message(fmt.Sprintf("unit test execution memory limit must be at most %d", Config.Limit.Unit.Max.Isolated.Memory)),
		),
	"processes": zog.Int().
		Default(Config.Limit.Unit.Default.Isolated.Processes).
		GT(0, zog.Message("unit test execution processes limit must be positive")).
		LTE(
			Config.Limit.Unit.Max.Isolated.Processes,
			zog.Message(fmt.Sprintf("unit test execution processes limit must be at most %d", Config.Limit.Unit.Max.Isolated.Processes)),
		),
	"openFiles": zog.Int().
		Default(Config.Limit.Unit.Default.Isolated.OpenFiles).
		GT(0, zog.Message("unit test execution open files limit must be positive")).
		LTE(
			Config.Limit.Unit.Max.Isolated.OpenFiles,
			zog.Message(fmt.Sprintf("unit test execution open files limit must be at most %d", Config.Limit.Unit.Max.Isolated.OpenFiles)),
		),
	"outputSize": zog.Int().
		Default(Config.Limit.Unit.Default.Isolated.OutputSize).
		GT(0, zog.Message("unit test execution output size limit must be positive")).
		LTE(
			Config.Limit.Unit.Max.Isolated.OutputSize,
			zog.Message(fmt.Sprintf("unit test execution output size limit must be at most %d", Config.Limit.Unit.Max.Isolated.OutputSize)),
		),
})

var execUnitBatchLimitSchema = zog.Struct(zog.Shape{
	"time": zog.Struct(zog.Shape{
		"real": zog.Float64().
			Default(Config.Limit.Unit.Default.Batch.Time.Real).
			GT(0, zog.Message("batch unit test execution time limit must be positive")).
			LTE(
				Config.Limit.Unit.Max.Batch.Time.Real,
				zog.Message(fmt.Sprintf("batch unit test execution time limit must be at most %g", Config.Limit.Unit.Max.Batch.Time.Real)),
			),
		"wall": zog.Float64().
			Default(Config.Limit.Unit.Default.Batch.Time.Wall).
			GTE(0, zog.Message("batch unit test execution wall-clock time limit must be positive or zero")).
			LTE(
				Config.Limit.Unit.Max.Batch.Time.Wall,
				zog.Message(fmt.Sprintf("batch unit test execution wall-clock time limit must be at most %g", Config.Limit.Unit.Max.Batch.Time.Wall)),
			),
	}),
	"memory": zog.Int().
		Default(Config.Limit.Unit.Default.Batch.Memory).
		GT(0, zog.Message("batch unit test execution memory limit must be positive")).
		LTE(
			Config.Limit.Unit.Max.Batch.Memory,
			zog.Message(fmt.Sprintf("batch unit test execution memory limit must be at most %d", Config.Limit.Unit.Max.Batch.Memory)),
		),
	"processes": zog.Int().
		Default(Config.Limit.Unit.Default.Batch.Processes).
		GT(0, zog.Message("batch unit test execution processes limit must be positive")).
		LTE(
			Config.Limit.Unit.Max.Batch.Processes,
			zog.Message(fmt.Sprintf("batch unit test execution processes limit must be at most %d", Config.Limit.Unit.Max.Batch.Processes)),
		),
	"openFiles": zog.Int().
		Default(Config.Limit.Unit.Default.Batch.OpenFiles).
		GT(0, zog.Message("batch unit test execution open files limit must be positive")).
		LTE(
			Config.Limit.Unit.Max.Batch.OpenFiles,
			zog.Message(fmt.Sprintf("batch unit test execution open files limit must be at most %d", Config.Limit.Unit.Max.Batch.OpenFiles)),
		),
	"outputSize": zog.Int().
		Default(Config.Limit.Unit.Default.Batch.OutputSize).
		GT(0, zog.Message("batch unit test execution output size limit must be positive")).
		LTE(
			Config.Limit.Unit.Max.Batch.OutputSize,
			zog.Message(fmt.Sprintf("batch unit test execution output size limit must be at most %d", Config.Limit.Unit.Max.Batch.OutputSize)),
		),
})

var execStdioSetSchema = zog.Struct(zog.Shape{
	"isolationLevel": zog.String().Default("medium").OneOf([]string{"medium", "high"}, zog.Message("invalid isolation level")),
	"limit":          execStdioBatchLimitSchema,
	"cases": zog.Slice(zog.Struct(zog.Shape{
		"input": zog.String().Default(""),
		"limit": execStdioIsolatedLimitSchema,
	})),
})

var execUnitSetSchema = zog.Struct(zog.Shape{
	"isolationLevel": zog.String().Default("low").OneOf([]string{"low", "medium", "high"}, zog.Message("invalid isolation level")),
	"limit":          execUnitBatchLimitSchema,
	"cases": zog.Slice(zog.Struct(zog.Shape{
		"script": zog.String().Required(zog.Message("test script is required")),
		"limit":  execUnitIsolatedLimitSchema,
	})),
})

var ExecDataSchema = zog.Struct(zog.Shape{
	"version":            zog.String().Required(zog.Message("version is required")).OneOf(Config.Versions, zog.Message("unsupported version")),
	"submission":         zog.String().Required(zog.Message("submission is required")),
	"coloredDiagnostics": zog.Bool().Default(false),
	"stdioSets":          zog.Slice(execStdioSetSchema),
	"unitSets":           zog.Slice(execUnitSetSchema),
})
