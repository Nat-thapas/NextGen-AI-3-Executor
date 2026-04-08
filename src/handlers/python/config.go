package python

import (
	"next-gen-ai-web-application/executor/src/utils/env"
	"strings"
)

type ConfigPath struct {
	Python                 string
	BatchStdioRunner       string
	BatchUnitRunner        string
	BatchUnitColorConfTest string
}

type ConfigLimitModes struct {
	Isolated ExecLimit
	Batch    ExecLimit
}

type ConfigLimitKinds struct {
	Default ConfigLimitModes
	Max     ConfigLimitModes
}

type ConfigLimitTypes struct {
	Stdio ConfigLimitKinds
	Unit  ConfigLimitKinds
}

type ConfigType struct {
	Path      ConfigPath
	Versions  []string
	BatchSize int
	Limit     ConfigLimitTypes
}

var Config = ConfigType{
	Path: ConfigPath{
		Python:                 env.GetEnvWithDefault("PYTHON_PATH", "/usr/bin/python{version}"),
		BatchStdioRunner:       env.GetEnvWithDefault("PYTHON_BATCH_STDIO_RUNNER_SCRIPT", "./helpers/python/batch_stdio_runner.py"),
		BatchUnitRunner:        env.GetEnvWithDefault("PYTHON_BATCH_UNIT_RUNNER_SCRIPT", "./helpers/python/batch_unit_runner.py"),
		BatchUnitColorConfTest: env.GetEnvWithDefault("PYTHON_BATCH_UNIT_COLOR_CONFTEST_PATH", "./helpers/python/batch_unit_color_conftest.py"),
	},
	Versions:  strings.Split(env.GetEnvWithDefault("PYTHON_AVAILABLE_VERSIONS", "3.12"), ","),
	BatchSize: 64,
	Limit: ConfigLimitTypes{
		Stdio: ConfigLimitKinds{
			Default: ConfigLimitModes{
				Isolated: ExecLimit{
					Processes: 1,
					Time: ExecLimitTime{
						Real: 1,
						Wall: 0, // Auto
					},
					Memory:     32 * 1024 * 1024,
					OutputSize: 1 * 1024 * 1024,
					OpenFiles:  16,
				},
				Batch: ExecLimit{
					Processes: 128,
					Time: ExecLimitTime{
						Real: 5,
						Wall: 0, // Auto
					},
					Memory:     128 * 1024 * 1024,
					OutputSize: 4 * 1024 * 1024,
					OpenFiles:  512,
				},
			},
			Max: ConfigLimitModes{
				Isolated: ExecLimit{
					Processes: 4,
					Time: ExecLimitTime{
						Real: 15,
						Wall: 60,
					},
					Memory:     256 * 1024 * 1024,
					OutputSize: 16 * 1024 * 1024,
					OpenFiles:  64,
				},
				Batch: ExecLimit{
					Processes: 256,
					Time: ExecLimitTime{
						Real: 15,
						Wall: 60,
					},
					Memory:     256 * 1024 * 1024,
					OutputSize: 16 * 1024 * 1024,
					OpenFiles:  1024,
				},
			},
		},
		Unit: ConfigLimitKinds{
			Default: ConfigLimitModes{
				Isolated: ExecLimit{
					Processes: 1,
					Time: ExecLimitTime{
						Real: 1,
						Wall: 0, // Auto
					},
					Memory:     48 * 1024 * 1024,
					OutputSize: 1 * 1024 * 1024,
					OpenFiles:  16,
				},
				Batch: ExecLimit{
					Processes: 128,
					Time: ExecLimitTime{
						Real: 5,
						Wall: 0, // Auto
					},
					Memory:     128 * 1024 * 1024,
					OutputSize: 4 * 1024 * 1024,
					OpenFiles:  512,
				},
			},
			Max: ConfigLimitModes{
				Isolated: ExecLimit{
					Processes: 4,
					Time: ExecLimitTime{
						Real: 15,
						Wall: 60,
					},
					Memory:     256 * 1024 * 1024,
					OutputSize: 16 * 1024 * 1024,
					OpenFiles:  32,
				},
				Batch: ExecLimit{
					Processes: 256,
					Time: ExecLimitTime{
						Real: 15,
						Wall: 60,
					},
					Memory:     256 * 1024 * 1024,
					OutputSize: 16 * 1024 * 1024,
					OpenFiles:  1024,
				},
			},
		},
	},
}
