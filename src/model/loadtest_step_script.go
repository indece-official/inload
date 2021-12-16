package model

import (
	"fmt"

	"github.com/indece-official/loadtest/src/report"
	"github.com/indece-official/loadtest/src/stats"
	"github.com/robertkrimen/otto"
)

type LoadTestStepExec struct {
	Script ExecutableStringOrNull `yaml:"script"`
}

func (l *LoadTestStepExec) Validate() error {
	if !l.Script.Valid {
		return fmt.Errorf("script must not be empty")
	}

	return nil
}

func (l *LoadTestStepExec) Execute(path []string, vm *otto.Otto, runStats *stats.RunStats, report *report.Report) (*StepExecutionStats, error) {
	_, err := l.Script.Execute(vm)
	if err != nil {
		return nil, fmt.Errorf("can't execute script: %s", err)
	}

	return nil, nil
}

var _ IRunnableStep = (*LoadTestStepExec)(nil)
