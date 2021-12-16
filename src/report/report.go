package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"time"

	"github.com/indece-official/loadtest/src/assets"
	"github.com/indece-official/loadtest/src/stats"
	"gopkg.in/guregu/null.v4"
)

type Report struct {
}

type ReportDataJSONStepExecution struct {
	StartTime     int64 `json:"start_time"`
	DurationTotal int64 `json:"duration_total"`
}

type ReportDataJSONStep struct {
	Name       string                         `json:"name"`
	Executions []*ReportDataJSONStepExecution `json:"executions"`
}

type ReportDataJSON struct {
	Steps []*ReportDataJSONStep `json:"steps"`
}

type ReportData struct {
	Datetime           string
	DurationTotal      time.Duration
	CountStepsTotal    int64
	CountStepsSkipped  int64
	CountStepsSucceded int64
	CountStepsFailed   int64
	Steps              []*ReportDataStep
	ExecutionsJSON     string
}

type ReportDataStepCode struct {
	Code  string
	Count int
}

type ReportDataStep struct {
	Name             string
	CountTotal       int64
	CountSkipped     int64
	CountSucceded    int64
	CountFailed      int64
	Errors           []string
	Codes            []*ReportDataStepCode
	DurationAvg      time.Duration
	DurationMin      time.Duration
	DurationMax      time.Duration
	BytesSentAvg     null.Float
	BytesSentMin     null.Int
	BytesSentMax     null.Int
	BytesReceivedAvg null.Float
	BytesReceivedMin null.Int
	BytesReceivedMax null.Int
}

func (r *Report) Generate(runStats *stats.RunStats) ([]byte, error) {
	templateFile, err := assets.Assets.Open("template/report.html")
	if err != nil {
		return nil, fmt.Errorf("can't open template file: %s", err)
	}
	defer templateFile.Close()

	templateStr, err := ioutil.ReadAll(templateFile)
	if err != nil {
		return nil, fmt.Errorf("can't read report template file: %s", err)
	}

	tpl, err := template.New("report").Parse(string(templateStr))
	if err != nil {
		return nil, fmt.Errorf("can't load report template: %s", err)
	}

	data := &ReportData{}
	data.Datetime = runStats.StartTime.Format("2006-01-02 15:04:05")
	data.DurationTotal = runStats.TotalDuration
	data.CountStepsTotal = runStats.CountStepsTotal
	data.CountStepsSkipped = runStats.CountStepsSkipped
	data.CountStepsSucceded = runStats.CountStepsSucceded
	data.CountStepsFailed = runStats.CountStepsFailed
	data.Steps = []*ReportDataStep{}

	dataJSON := &ReportDataJSON{}
	dataJSON.Steps = []*ReportDataJSONStep{}

	for name, runStatStep := range runStats.Steps {
		if runStatStep.IsGroup || !runStatStep.HasExplicitName {
			continue
		}

		step := &ReportDataStep{}
		step.Name = name
		step.CountTotal = runStatStep.CountTotal
		step.CountSkipped = runStatStep.CountSkipped
		step.CountSucceded = runStatStep.CountSucceded
		step.CountFailed = runStatStep.CountFailed
		step.DurationAvg = runStatStep.DurationAvg
		step.DurationMin = runStatStep.DurationMin
		step.DurationMax = runStatStep.DurationMax

		step.BytesSentAvg = runStatStep.BytesSentAvg
		step.BytesSentMin = runStatStep.BytesSentMin
		step.BytesSentMax = runStatStep.BytesSentMax
		step.BytesReceivedAvg = runStatStep.BytesReceivedAvg
		step.BytesReceivedMin = runStatStep.BytesReceivedMin
		step.BytesReceivedMax = runStatStep.BytesReceivedMax

		step.Codes = []*ReportDataStepCode{}
		for code, count := range runStatStep.Codes {
			stepCode := &ReportDataStepCode{}

			stepCode.Code = code
			stepCode.Count = count

			step.Codes = append(step.Codes, stepCode)
		}

		mapErrors := map[string]bool{}
		for _, err := range runStatStep.Errors {
			mapErrors[err.Error()] = true
		}

		step.Errors = []string{}
		for errorStr := range mapErrors {
			step.Errors = append(step.Errors, errorStr)
		}

		dataJSONStep := &ReportDataJSONStep{}
		dataJSONStep.Name = name

		for _, runStatExecution := range runStatStep.Executions {
			dataJSONExecution := &ReportDataJSONStepExecution{}

			dataJSONExecution.StartTime = runStatExecution.StartTime.Sub(runStats.StartTime).Milliseconds()
			dataJSONExecution.DurationTotal = runStatExecution.DurationTotal.Milliseconds()

			dataJSONStep.Executions = append(dataJSONStep.Executions, dataJSONExecution)
		}

		dataJSON.Steps = append(dataJSON.Steps, dataJSONStep)
		data.Steps = append(data.Steps, step)
	}

	executionsJSON, err := json.Marshal(dataJSON)
	if err != nil {
		return nil, fmt.Errorf("can't excode json for report template: %s", err)
	}

	data.ExecutionsJSON = string(executionsJSON)

	var buf bytes.Buffer

	err = tpl.Execute(&buf, data)
	if err != nil {
		return nil, fmt.Errorf("can't execute report template: %s", err)
	}

	return buf.Bytes(), nil
}

func NewReport() *Report {
	return &Report{}
}
