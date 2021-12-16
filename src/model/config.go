package model

import (
	"fmt"

	"github.com/indece-official/loadtest/src/report"
	"github.com/indece-official/loadtest/src/stats"
	"github.com/robertkrimen/otto"
)

type ConfigVersion string

const ConfigVersionV1 ConfigVersion = "v1"

type Config struct {
	Version ConfigVersion `yaml:"version"`
	Tests   []*LoadTest   `yaml:"tests"`
}

func (l *Config) Validate() error {
	if l.Version != ConfigVersionV1 {
		return fmt.Errorf("unsupported config version")
	}

	for _, test := range l.Tests {
		err := test.Validate()
		if err != nil {
			return fmt.Errorf("error in load test '%s': %s", test.Name, err)
		}
	}

	return nil
}

func (l *Config) Execute(path []string, vm *otto.Otto, runStats *stats.RunStats, report *report.Report) error {
	for _, test := range l.Tests {
		err := test.Execute(path, vm, runStats, report)
		if err != nil {
			return fmt.Errorf("load test %s failed: %s", test.Name, err)
		}
	}

	return nil
}

var _ IRunnable = (*Config)(nil)
