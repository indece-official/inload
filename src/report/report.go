package report

import (
	"bytes"
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

type ReportData struct {
	Datetime           string
	DurationTotal      time.Duration
	CountStepsTotal    int64
	CountStepsSkipped  int64
	CountStepsSucceded int64
	CountStepsFailed   int64
	Steps              []*ReportDataStep
}

type ReportDataStep struct {
	Name             string
	CountTotal       int64
	CountSkipped     int64
	CountSucceded    int64
	CountFailed      int64
	Errors           []string
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
	data.Datetime = time.Now().Format("2006-01-02 15:04:05")
	data.DurationTotal = runStats.TotalDuration
	data.CountStepsTotal = runStats.CountStepsTotal
	data.CountStepsSkipped = runStats.CountStepsSkipped
	data.CountStepsSucceded = runStats.CountStepsSucceded
	data.CountStepsFailed = runStats.CountStepsFailed
	data.Steps = []*ReportDataStep{}

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

		data.Steps = append(data.Steps, step)
	}

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
