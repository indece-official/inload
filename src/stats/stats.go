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

type RunStats struct {
	mutexStepExecutions sync.Mutex
	stepExecutions      []*StepExecution
}

func (r *RunStats) AddStepExecution(stepExecution *StepExecution) {
	r.mutexStepExecutions.Lock()
	defer r.mutexStepExecutions.Unlock()

	r.stepExecutions = append(r.stepExecutions, stepExecution)
}

func (r *RunStats) Print() {
	namedStepsMap := map[string]bool{}
	stepDurationTotalMap := map[string][]time.Duration{}
	stepBytesSentMap := map[string][]int64{}
	stepBytesReceivedMap := map[string][]int64{}
	stepCountTotalMap := map[string]int{}
	stepCountSkippedMap := map[string]int{}
	stepCountSuccessMap := map[string]int{}
	stepCountFailedMap := map[string]int{}
	codeCountMap := map[string]map[string]int{}
	countStepsTotal := 0
	countStepsSkipped := 0
	countStepsSuccess := 0
	countStepsFailed := 0

	for _, stepExecution := range r.stepExecutions {
		if !stepExecution.IsGroup && stepExecution.HasExplicitName {
			namedStepsMap[stepExecution.Name] = true
		}

		if _, ok := codeCountMap[stepExecution.Name]; !ok {
			codeCountMap[stepExecution.Name] = map[string]int{}
		}

		stepDurationTotalMap[stepExecution.Name] = append(stepDurationTotalMap[stepExecution.Name], stepExecution.DurationTotal)

		if stepExecution.BytesSent.Valid {
			stepBytesSentMap[stepExecution.Name] = append(stepBytesSentMap[stepExecution.Name], stepExecution.BytesSent.Int64)
		}

		if stepExecution.BytesReceived.Valid {
			stepBytesReceivedMap[stepExecution.Name] = append(stepBytesReceivedMap[stepExecution.Name], stepExecution.BytesReceived.Int64)
		}

		stepCountTotalMap[stepExecution.Name]++

		countStepsTotal++

		switch stepExecution.Status {
		case StepExecutionStatusSuccess:
			stepCountSuccessMap[stepExecution.Name]++
			countStepsSuccess++
		case StepExecutionStatusSkipped:
			stepCountSkippedMap[stepExecution.Name]++
			countStepsSkipped++
		case StepExecutionStatusFailed:
			stepCountFailedMap[stepExecution.Name]++
			countStepsFailed++
		}

		if stepExecution.Code.Valid {
			codeCountMap[stepExecution.Name][stepExecution.Code.String]++
		}
	}

	log.Infof("######################## Run stats ########################")
	log.Infof("Steps total:   %d", countStepsTotal)
	log.Infof("Steps skipped: %d", countStepsSkipped)
	log.Infof("Steps success: %d", countStepsSuccess)
	log.Infof("Steps failed:  %d", countStepsFailed)

	for name := range namedStepsMap {
		durationTotalMax := time.Duration(0)
		durationTotalSum := time.Duration(0)
		durationTotalMin := time.Duration(0)
		for i, durationTotal := range stepDurationTotalMap[name] {
			durationTotalSum += durationTotal
			durationTotalMax = utils.MaxDuration(durationTotalMax, durationTotal)
			if i == 0 {
				durationTotalMin = durationTotal
			} else {
				durationTotalMin = utils.MinDuration(durationTotalMin, durationTotal)
			}
		}

		durationTotalAvg := float64(durationTotalSum.Milliseconds()) / math.Max(float64(len(stepDurationTotalMap[name])), 1)

		bytesSentMax := int64(0)
		bytesSentSum := int64(0)
		bytesSentMin := int64(0)
		bytesSentCount := 0
		for _, bytesSent := range stepBytesSentMap[name] {
			bytesSentSum += bytesSent
			bytesSentMax = utils.MaxInt64(bytesSentMax, bytesSent)
			if bytesSentCount == 0 {
				bytesSentMin = bytesSent
			} else {
				bytesSentMin = utils.MinInt64(bytesSentMin, bytesSent)
			}
			bytesSentCount++
		}

		bytesSentAvg := float64(bytesSentSum) / math.Max(float64(bytesSentCount), 1)

		bytesReceivedMax := int64(0)
		bytesReceivedSum := int64(0)
		bytesReceivedMin := int64(0)
		bytesReceivedCount := 0
		for _, bytesReceived := range stepBytesReceivedMap[name] {
			bytesReceivedSum += bytesReceived
			bytesReceivedMax = utils.MaxInt64(bytesReceivedMax, bytesReceived)
			if bytesReceivedCount == 0 {
				bytesReceivedMin = bytesReceived
			} else {
				bytesReceivedMin = utils.MinInt64(bytesReceivedMin, bytesReceived)
			}
			bytesReceivedCount++
		}

		bytesReceivedAvg := float64(bytesReceivedSum) / math.Max(float64(bytesReceivedCount), 1)

		log.Infof("")
		log.Infof("Step %s:", name)
		log.Infof("   Count total:     %d", stepCountTotalMap[name])
		log.Infof("   Count skipped:   %d", stepCountSkippedMap[name])
		log.Infof("   Count success:   %d", stepCountSuccessMap[name])
		log.Infof("   Count failed:    %d", stepCountFailedMap[name])
		log.Infof("   Avg duration:  %.0f ms", durationTotalAvg)
		log.Infof("   Max duration:  %d ms", durationTotalMax.Milliseconds())
		log.Infof("   Min duration:  %d ms", durationTotalMin.Milliseconds())

		if bytesSentCount > 0 {
			log.Infof("   Avg bytes sent:  %.0f b", bytesSentAvg)
			log.Infof("   Max bytes sent:  %d b", bytesSentMax)
			log.Infof("   Min bytes sent:  %d b", bytesSentMin)
		}

		if bytesReceivedCount > 0 {
			log.Infof("   Avg bytes received:  %.0f b", bytesReceivedAvg)
			log.Infof("   Max bytes received:  %d b", bytesReceivedMax)
			log.Infof("   Min bytes received:  %d b", bytesReceivedMin)
		}

		for code, count := range codeCountMap[name] {
			log.Infof("   Code %s:        %d", code, count)
		}
	}
}

func NewRunStats() *RunStats {
	return &RunStats{}
}
