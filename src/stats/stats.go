package stats

import (
	"math"
	"sync"
	"time"

	"github.com/indece-official/loadtest/src/utils"
	log "github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v4"
)

type StepExecutionStatus string

const (
	StepExecutionStatusSuccess StepExecutionStatus = "success"
	StepExecutionStatusFailed  StepExecutionStatus = "failed"
	StepExecutionStatusSkipped StepExecutionStatus = "skipped"
)

type StepExecution struct {
	Name             string
	HasExplicitName  bool
	StartTime        time.Time
	EndTime          time.Time
	IsGroup          bool
	DurationTotal    time.Duration
	DurationRequest  *time.Duration
	DurationResponse *time.Duration
	Status           StepExecutionStatus
	Error            error
	Code             null.String
	BytesSent        null.Int
	BytesReceived    null.Int
}

type RunStatStep struct {
	HasExplicitName  bool
	IsGroup          bool
	CountTotal       int64
	CountSkipped     int64
	CountSucceded    int64
	CountFailed      int64
	Errors           []error
	DurationAvg      time.Duration
	DurationMin      time.Duration
	DurationMax      time.Duration
	BytesSentAvg     null.Float
	BytesSentMin     null.Int
	BytesSentMax     null.Int
	BytesReceivedAvg null.Float
	BytesReceivedMin null.Int
	BytesReceivedMax null.Int
	Codes            map[string]int
	Executions       []*StepExecution
}

type RunStats struct {
	mutexStepExecutions sync.Mutex
	stepExecutions      []*StepExecution

	StartTime          time.Time
	TotalDuration      time.Duration
	CountStepsTotal    int64
	CountStepsSkipped  int64
	CountStepsSucceded int64
	CountStepsFailed   int64
	Steps              map[string]*RunStatStep
}

func (r *RunStats) SetStart() {
	r.StartTime = time.Now()
}

func (r *RunStats) SetEnd() {
	r.TotalDuration = time.Since(r.StartTime)
}

func (r *RunStats) AddStepExecution(stepExecution *StepExecution) {
	r.mutexStepExecutions.Lock()
	defer r.mutexStepExecutions.Unlock()

	r.stepExecutions = append(r.stepExecutions, stepExecution)
}

func (r *RunStats) Aggregate() {
	r.CountStepsTotal = 0
	r.CountStepsSkipped = 0
	r.CountStepsSucceded = 0
	r.CountStepsFailed = 0
	r.Steps = map[string]*RunStatStep{}

	durationMinMap := map[string]time.Duration{}
	durationMaxMap := map[string]time.Duration{}
	durationSumMap := map[string]time.Duration{}
	durationCountMap := map[string]int{}

	bytesSentMinMap := map[string]int64{}
	bytesSentMaxMap := map[string]int64{}
	bytesSentSumMap := map[string]int64{}
	bytesSentCountMap := map[string]int{}

	bytesReceivedMinMap := map[string]int64{}
	bytesReceivedMaxMap := map[string]int64{}
	bytesReceivedSumMap := map[string]int64{}
	bytesReceivedCountMap := map[string]int{}

	for _, stepExecution := range r.stepExecutions {
		if _, ok := r.Steps[stepExecution.Name]; !ok {
			r.Steps[stepExecution.Name] = &RunStatStep{}
			r.Steps[stepExecution.Name].IsGroup = stepExecution.IsGroup
			r.Steps[stepExecution.Name].HasExplicitName = stepExecution.HasExplicitName
			r.Steps[stepExecution.Name].Codes = map[string]int{}
		}

		r.CountStepsTotal++
		r.Steps[stepExecution.Name].CountTotal++

		r.Steps[stepExecution.Name].Executions = append(r.Steps[stepExecution.Name].Executions, stepExecution)

		switch stepExecution.Status {
		case StepExecutionStatusSuccess:
			r.CountStepsSucceded++
			r.Steps[stepExecution.Name].CountSucceded++
		case StepExecutionStatusSkipped:
			r.CountStepsSkipped++
			r.Steps[stepExecution.Name].CountSkipped++
		case StepExecutionStatusFailed:
			r.CountStepsFailed++
			r.Steps[stepExecution.Name].CountFailed++
		}

		if stepExecution.Error != nil {
			r.Steps[stepExecution.Name].Errors = append(r.Steps[stepExecution.Name].Errors, stepExecution.Error)
		}

		if durationCountMap[stepExecution.Name] == 0 {
			durationMinMap[stepExecution.Name] = stepExecution.DurationTotal
		} else {
			durationMinMap[stepExecution.Name] = utils.MinDuration(durationMinMap[stepExecution.Name], stepExecution.DurationTotal)
		}
		durationMaxMap[stepExecution.Name] = utils.MaxDuration(durationMaxMap[stepExecution.Name], stepExecution.DurationTotal)
		durationSumMap[stepExecution.Name] += stepExecution.DurationTotal
		durationCountMap[stepExecution.Name]++

		if stepExecution.BytesSent.Valid {
			if bytesSentCountMap[stepExecution.Name] == 0 {
				bytesSentMinMap[stepExecution.Name] = stepExecution.BytesSent.Int64
			} else {
				bytesSentMinMap[stepExecution.Name] = utils.MinInt64(bytesSentMaxMap[stepExecution.Name], stepExecution.BytesSent.Int64)
			}
			bytesSentMaxMap[stepExecution.Name] = utils.MaxInt64(bytesSentMaxMap[stepExecution.Name], stepExecution.BytesSent.Int64)
			bytesSentSumMap[stepExecution.Name] += stepExecution.BytesSent.Int64
			bytesSentCountMap[stepExecution.Name]++
		}

		if stepExecution.BytesReceived.Valid {
			if bytesReceivedCountMap[stepExecution.Name] == 0 {
				bytesReceivedMinMap[stepExecution.Name] = stepExecution.BytesReceived.Int64
			} else {
				bytesReceivedMinMap[stepExecution.Name] = utils.MinInt64(bytesReceivedMaxMap[stepExecution.Name], stepExecution.BytesReceived.Int64)
			}
			bytesReceivedMaxMap[stepExecution.Name] = utils.MaxInt64(bytesReceivedMaxMap[stepExecution.Name], stepExecution.BytesReceived.Int64)
			bytesReceivedSumMap[stepExecution.Name] += stepExecution.BytesReceived.Int64
			bytesReceivedCountMap[stepExecution.Name]++
		}

		if stepExecution.Code.Valid {
			r.Steps[stepExecution.Name].Codes[stepExecution.Code.String]++
		}
	}

	for name := range bytesSentCountMap {
		r.Steps[name].BytesSentMin.Scan(bytesSentMinMap[name])
		r.Steps[name].BytesSentMax.Scan(bytesSentMaxMap[name])
		r.Steps[name].BytesSentAvg.Scan(float64(bytesSentSumMap[name]) / float64(bytesSentCountMap[name]))
	}

	for name := range bytesReceivedCountMap {
		r.Steps[name].BytesReceivedMin.Scan(bytesReceivedMinMap[name])
		r.Steps[name].BytesReceivedMax.Scan(bytesReceivedMaxMap[name])
		r.Steps[name].BytesReceivedAvg.Scan(float64(bytesReceivedSumMap[name]) / float64(bytesReceivedCountMap[name]))
	}

	for name := range durationCountMap {
		r.Steps[name].DurationMin = durationMinMap[name]
		r.Steps[name].DurationMax = durationMaxMap[name]
		r.Steps[name].DurationAvg = time.Duration(math.Round(float64(durationSumMap[name]) / float64(durationCountMap[name])))
	}
}

func (r *RunStats) Print() {
	log.Infof("######################## Run stats ########################")
	log.Infof("Steps total:    %d", r.CountStepsTotal)
	log.Infof("Steps skipped:  %d", r.CountStepsSkipped)
	log.Infof("Steps succeded: %d", r.CountStepsSucceded)
	log.Infof("Steps failed:   %d", r.CountStepsFailed)

	for name, step := range r.Steps {
		if step.IsGroup || !step.HasExplicitName {
			continue
		}

		log.Infof("")
		log.Infof("Step %s:", name)
		log.Infof("   Count total:     %d", step.CountTotal)
		log.Infof("   Count skipped:   %d", step.CountSkipped)
		log.Infof("   Count success:   %d", step.CountSucceded)
		log.Infof("   Count failed:    %d", step.CountFailed)
		log.Infof("   Avg duration:  %d ms", step.DurationAvg.Milliseconds())
		log.Infof("   Max duration:  %d ms", step.DurationMax.Milliseconds())
		log.Infof("   Min duration:  %d ms", step.DurationMin.Milliseconds())

		if step.BytesSentAvg.Valid {
			log.Infof("   Avg bytes sent:  %.0f b", step.BytesSentAvg.Float64)
			log.Infof("   Max bytes sent:  %d b", step.BytesSentMax.Int64)
			log.Infof("   Min bytes sent:  %d b", step.BytesSentMin.Int64)
		}

		if step.BytesReceivedAvg.Valid {
			log.Infof("   Avg bytes received:  %.0f b", step.BytesReceivedAvg.Float64)
			log.Infof("   Max bytes received:  %d b", step.BytesReceivedMax.Int64)
			log.Infof("   Min bytes received:  %d b", step.BytesReceivedMin.Int64)
		}

		for code, count := range step.Codes {
			log.Infof("   Code %s:        %d", code, count)
		}
	}
}

func NewRunStats() *RunStats {
	return &RunStats{}
}
