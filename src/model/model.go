package model

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/indece-official/loadtest/src/report"
	"github.com/indece-official/loadtest/src/stats"
	"github.com/robertkrimen/otto"
	log "github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v4"
)

type StepExecutionStats struct {
	DurationRequest  *time.Duration
	DurationResponse *time.Duration
	Code             null.String
	BytesSent        null.Int
	BytesReceived    null.Int
}

type IRunnable interface {
	Validate() error
	Execute(path []string, vm *otto.Otto, runStats *stats.RunStats, report *report.Report) error
}

type IRunnableStep interface {
	Validate() error
	Execute(path []string, vm *otto.Otto, runStats *stats.RunStats, report *report.Report) (*StepExecutionStats, error)
}

type LoadTest struct {
	Name     string                 `yaml:"name"`
	Disabled null.Bool              `yaml:"disabled"`
	Vars     map[string]interface{} `yaml:"vars"`
	Steps    []*LoadTestStep        `yaml:"steps"`
}

func (l *LoadTest) Validate() error {
	if l.Name == "" {
		return fmt.Errorf("no name for load test defined")
	}

	if len(l.Steps) == 0 {
		return fmt.Errorf("no step defined for load test '%s'", l.Name)
	}

	for i, step := range l.Steps {
		err := step.Validate()
		if err != nil {
			return fmt.Errorf("error in step %d of load test '%s': %s", i+1, l.Name, err)
		}
	}

	return nil
}

func (l *LoadTest) Execute(path []string, vm *otto.Otto, runStats *stats.RunStats, report *report.Report) error {
	newPath := append(path, l.Name)

	log.Debugf("Starting test %s", strings.Join(newPath, "."))

	for key, value := range l.Vars {
		err := vm.Set(key, value)
		if err != nil {
			return fmt.Errorf("can't set value for var '%s'", key)
		}
	}

	for i, step := range l.Steps {
		var subPath []string

		if step.Name.Valid {
			subPath = append(newPath, step.Name.String)
		} else {
			subPath = append(newPath, fmt.Sprintf("%d", i))
		}

		err := step.Execute(subPath, vm, runStats, report)
		if err != nil {
			return fmt.Errorf("step %d of load test %s failed: %s", i, l.Name, err)
		}
	}

	log.Debugf("Finished test %s", strings.Join(newPath, "."))

	return nil
}

var _ IRunnable = (*LoadTest)(nil)

type LoadTestStep struct {
	Name     null.String          `yaml:"name"`
	Disabled null.Bool            `yaml:"disabled"`
	Loop     *LoadTestStepLoop    `yaml:"loop"`
	Log      *LoadTestStepLog     `yaml:"log"`
	Threads  *LoadTestStepThreads `yaml:"threads"`
	Http     *LoadTestStepHttp    `yaml:"http"`
	Exec     *LoadTestStepExec    `yaml:"exec"`
}

func (l *LoadTestStep) Validate() error {
	switch {
	case l.Loop != nil:
		err := l.Loop.Validate()
		if err != nil {
			return fmt.Errorf("error in step '%s': invalid loop: %s", l.Name.String, err)
		}
	case l.Threads != nil:
		err := l.Threads.Validate()
		if err != nil {
			return fmt.Errorf("error in step '%s': invalid threads: %s", l.Name.String, err)
		}
	case l.Log != nil:
		err := l.Log.Validate()
		if err != nil {
			return fmt.Errorf("error in step '%s': invalid log: %s", l.Name.String, err)
		}
	case l.Http != nil:
		err := l.Http.Validate()
		if err != nil {
			return fmt.Errorf("error in step '%s': invalid http: %s", l.Name.String, err)
		}
	case l.Exec != nil:
		err := l.Exec.Validate()
		if err != nil {
			return fmt.Errorf("error in step '%s': invalid exec: %s", l.Name.String, err)
		}
	default:
		return fmt.Errorf("loop must contain one child of 'loop' | 'threads' | 'log' | 'http'")
	}

	return nil
}

func (l *LoadTestStep) Execute(path []string, vm *otto.Otto, runStats *stats.RunStats, report *report.Report) error {
	name := l.Name.String
	if name == "" {
		name = strings.Join(path, ".")
	}

	log.Debugf("Starting test step '%s'", name)

	if l.Disabled.Valid && l.Disabled.Bool {
		log.Debugf("Skipped test step '%s'", name)

		runStats.AddStepExecution(&stats.StepExecution{
			HasExplicitName: l.Name.Valid,
			Name:            name,
			Status:          stats.StepExecutionStatusSkipped,
			Error:           nil,
			DurationTotal:   0,
		})

		return nil
	}

	start := time.Now()

	var err error
	var stepStats *StepExecutionStats
	isGroup := false

	switch {
	case l.Loop != nil:
		isGroup = true
		stepStats, err = l.Loop.Execute(path, vm, runStats, report)
	case l.Threads != nil:
		isGroup = true
		stepStats, err = l.Threads.Execute(path, vm, runStats, report)
	case l.Log != nil:
		stepStats, err = l.Log.Execute(path, vm, runStats, report)
	case l.Http != nil:
		stepStats, err = l.Http.Execute(path, vm, runStats, report)
	case l.Exec != nil:
		stepStats, err = l.Exec.Execute(path, vm, runStats, report)
	}

	duration := time.Since(start)

	execution := &stats.StepExecution{
		IsGroup:         isGroup,
		Status:          stats.StepExecutionStatusSuccess,
		HasExplicitName: l.Name.Valid,
		StartTime:       start,
		Name:            name,
		DurationTotal:   duration,
	}

	if stepStats != nil {
		execution.BytesReceived = stepStats.BytesReceived
		execution.BytesSent = stepStats.BytesSent
		execution.DurationRequest = stepStats.DurationRequest
		execution.DurationResponse = stepStats.DurationResponse
		execution.Code = stepStats.Code
	}

	if err != nil {
		execution.Error = err
		execution.Status = stats.StepExecutionStatusFailed

		log.Errorf("Test step %s failed: %s", name, err)
	}

	runStats.AddStepExecution(execution)

	log.Debugf("Finished test step %s", name)

	return nil
}

var _ IRunnable = (*LoadTestStep)(nil)

type LoadTestStepLoop struct {
	Count           null.Int               `yaml:"count"`
	CounterVariable null.String            `yaml:"counter_variable"`
	While           ExecutableStringOrNull `yaml:"while"`
	Steps           []*LoadTestStep        `yaml:"steps"`
}

func (l *LoadTestStepLoop) Validate() error {
	if !l.Count.Valid && !l.While.Valid {
		return fmt.Errorf("loop must contain one child of 'count' | 'while'")
	}

	if l.Count.Valid && l.Count.Int64 <= 0 {
		return fmt.Errorf("count must be greater 0")
	}

	for i, step := range l.Steps {
		err := step.Validate()
		if err != nil {
			return fmt.Errorf("error in step %d of loop: %s", i+1, err)
		}
	}

	return nil
}

func (l *LoadTestStepLoop) Execute(path []string, vm *otto.Otto, runStats *stats.RunStats, report *report.Report) (*StepExecutionStats, error) {
	counter := int64(0)

	for {
		if l.Count.Valid && counter >= l.Count.Int64 {
			// Finished
			return nil, nil
		}

		if l.While.Valid {
			val, err := l.While.Execute(vm)
			if err != nil {
				return nil, fmt.Errorf("error executing 'while' condition: %s", err)
			}

			boolVal, err := val.ToBoolean()
			if err != nil {
				return nil, fmt.Errorf("'while' condition must return a boolean")
			}

			if !boolVal {
				// Finished

				return nil, nil
			}
		}

		counterVariable := l.CounterVariable.String
		if counterVariable == "" {
			counterVariable = "counter"
		}

		vm.Set(counterVariable, counter)

		for i, step := range l.Steps {
			var subPath []string

			if step.Name.Valid {
				subPath = append(path, step.Name.String)
			} else {
				subPath = append(path, fmt.Sprintf("%d", i))
			}

			err := step.Execute(subPath, vm, runStats, report)
			if err != nil {
				return nil, fmt.Errorf("step %d of loop failed: %s", i, err)
			}
		}

		counter++
	}
}

var _ IRunnableStep = (*LoadTestStepLoop)(nil)

type LoadTestStepThreads struct {
	Count           int64           `yaml:"count"`
	CounterVariable null.String     `yaml:"counter_variable"`
	Steps           []*LoadTestStep `yaml:"steps"`
}

func (l *LoadTestStepThreads) Validate() error {
	if l.Count <= 0 {
		return fmt.Errorf("count must be greater 0")
	}

	for i, step := range l.Steps {
		err := step.Validate()
		if err != nil {
			return fmt.Errorf("error in step %d of loop: %s", i+1, err)
		}
	}

	return nil
}

func (l *LoadTestStepThreads) Execute(path []string, vm *otto.Otto, runStats *stats.RunStats, report *report.Report) (*StepExecutionStats, error) {
	waitGroup := sync.WaitGroup{}
	mutexVm := sync.Mutex{}

	for i := 0; i < int(l.Count); i++ {
		waitGroup.Add(1)
		mutexVm.Lock()
		vmCopy := vm.Copy()
		mutexVm.Unlock()

		go func(counter int, threadVm *otto.Otto) {
			defer waitGroup.Done()

			counterVariable := l.CounterVariable.String
			if counterVariable == "" {
				counterVariable = "counter"
			}

			threadVm.Set(counterVariable, counter)

			for i, step := range l.Steps {
				var subPath []string

				if step.Name.Valid {
					subPath = append(path, step.Name.String)
				} else {
					subPath = append(path, fmt.Sprintf("%d", i))
				}

				err := step.Execute(subPath, vm, runStats, report)
				if err != nil {
					log.Errorf("Step %d of thread failed: %s", i, err)

					return
				}
			}
		}(i, vmCopy)
	}

	waitGroup.Wait()

	return nil, nil
}

var _ IRunnableStep = (*LoadTestStepThreads)(nil)

type LoadTestStepLog struct {
	Message    null.String            `yaml:"msg"`
	Expression ExecutableStringOrNull `yaml:"expr"`
}

func (l *LoadTestStepLog) Validate() error {
	if !l.Message.Valid && !l.Expression.Valid {
		return fmt.Errorf("loop must contain one child of 'msg' | 'expr'")
	}

	if l.Message.Valid && l.Message.String == "" {
		return fmt.Errorf("msg must not be empty")
	}

	return nil
}

func (l *LoadTestStepLog) Execute(path []string, vm *otto.Otto, runStats *stats.RunStats, report *report.Report) (*StepExecutionStats, error) {
	if l.Message.Valid {
		log.Infof("[%s]: %s", strings.Join(path, "."), l.Message.String)

		return nil, nil
	}

	if l.Expression.Valid {
		val, err := l.Expression.Execute(vm)
		if err != nil {
			return nil, fmt.Errorf("can't execute expr: %s", err)
		}

		log.Infof("[%s]: %s", strings.Join(path, "."), val.String())

		return nil, nil
	}

	return nil, nil
}

var _ IRunnableStep = (*LoadTestStepLog)(nil)
