package model

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/indece-official/loadtest/src/report"
	"github.com/indece-official/loadtest/src/stats"
	"github.com/robertkrimen/otto"
	"gopkg.in/guregu/null.v4"
)

type HttpMethod string

const (
	HttpMethodGet     HttpMethod = "GET"
	HttpMethodPost    HttpMethod = "POST"
	HttpMethodPut     HttpMethod = "PUT"
	HttpMethodDelete  HttpMethod = "DELETE"
	HttpMethodHead    HttpMethod = "HEAD"
	HttpMethodConnect HttpMethod = "CONNECT"
	HttpMethodOptions HttpMethod = "OPTIONS"
	HttpMethodTrace   HttpMethod = "TRACE"
	HttpMethodPatch   HttpMethod = "PATCH"
)

type HttpHeader struct {
	Name  string                 `yaml:"name"`
	Value null.String            `yaml:"value"`
	Expr  ExecutableStringOrNull `yaml:"expr"`
}

type HttpBody struct {
	Value null.String            `yaml:"value"`
	Expr  ExecutableStringOrNull `yaml:"expr"`
}

type HttpAssertion struct {
	Name          null.String            `yaml:"name"`
	Status        null.String            `yaml:"status"`
	StatusCode    null.Int               `yaml:"statuscode"`
	ContentType   null.String            `yaml:"contenttype"`
	MinBodyLength null.Int               `yaml:"min_body_length"`
	MaxBodyLength null.Int               `yaml:"max_body_length"`
	Expr          ExecutableStringOrNull `yaml:"expr"`
}

func (h *HttpAssertion) Verify(resp *http.Response, bodyLength int64, vm *otto.Otto) error {
	name := ""
	if h.Name.Valid {
		name = fmt.Sprintf("'%s' ", h.Name.String)
	}

	if h.Status.Valid && resp.Status != h.Status.String {
		return fmt.Errorf("assertion %son http response status failed: expected '%s', got '%s'", name, h.Status.String, resp.Status)
	}

	if h.StatusCode.Valid && resp.StatusCode != int(h.StatusCode.Int64) {
		return fmt.Errorf("assertion %son http response status code failed: expected %d, got %d", name, h.StatusCode.Int64, resp.StatusCode)
	}

	if h.ContentType.Valid && resp.Header.Get("Content-type") != h.ContentType.String {
		return fmt.Errorf("assertion %son http response content type failed: expected '%s', got '%s'", name, h.ContentType.String, resp.Header.Get("Content-type"))
	}

	if h.MinBodyLength.Valid && bodyLength < h.MinBodyLength.Int64 {
		return fmt.Errorf("assertion %son http response body length failed: expected >= %d, got %d", name, h.MinBodyLength.Int64, bodyLength)
	}

	if h.MaxBodyLength.Valid && bodyLength > h.MaxBodyLength.Int64 {
		return fmt.Errorf("assertion %son http response body length failed: expected <= %d, got %d", name, h.MaxBodyLength.Int64, bodyLength)
	}

	if h.Expr.Valid {
		val, err := h.Expr.Execute(vm)
		if err != nil {
			return fmt.Errorf("error executing 'expr' for asserion %s: %s", name, err)
		}

		valBool, err := val.ToBoolean()
		if err != nil {
			return fmt.Errorf("'expr' of assertion %smust return a boolean: %s", name, err)
		}

		if !valBool {
			return fmt.Errorf("assertion %sfailed", name)
		}
	}

	return nil
}

type LoadTestStepHttp struct {
	URL         null.String            `yaml:"url"`
	URLExpr     ExecutableStringOrNull `yaml:"url_expr"`
	Method      HttpMethod             `yaml:"method"`
	RequestBody *HttpBody              `yaml:"request_body"`
	Headers     []HttpHeader           `yaml:"headers"`
	Timeout     null.String            `yaml:"timeout"`
	Assertions  []HttpAssertion        `yaml:"assertions"`
}

func (l *LoadTestStepHttp) Validate() error {
	if !l.URL.Valid && !l.URLExpr.Valid {
		return fmt.Errorf("loop must contain one child of 'url' | 'url_expr'")
	}

	if l.URL.Valid && l.URL.String == "" {
		return fmt.Errorf("'url' must not be empty")
	}

	return nil
}

func (l *LoadTestStepHttp) assignResponseObject(resp *http.Response, vm *otto.Otto) error {
	mutexVm.Lock()
	defer mutexVm.Unlock()

	vmObject, err := vm.Object(`response = {}`)
	if err != nil {
		return fmt.Errorf("error creating response object: %s", err)
	}

	vmObject.Set("status", resp.Status)
	vmObject.Set("statuscode", resp.StatusCode)
	vmHeaderObject, err := vm.Object(`response.header = {}`)
	if err != nil {
		return fmt.Errorf("error creating response header object: %s", err)
	}

	for name, values := range resp.Header {
		if len(values) > 0 {
			vmHeaderObject.Set(name, values[0])
		}
	}

	return nil
}

func (l *LoadTestStepHttp) Execute(path []string, vm *otto.Otto, runStats *stats.RunStats, report *report.Report) (*StepExecutionStats, error) {
	var url string

	if l.URL.Valid {
		url = l.URL.String
	} else if l.URLExpr.Valid {
		val, err := l.URLExpr.Execute(vm)
		if err != nil {
			return nil, fmt.Errorf("error executing 'url_expr': %s", err)
		}

		if !val.IsString() {
			return nil, fmt.Errorf("'url_expr' must return a string")
		}

		url, err = val.ToString()
		if err != nil {
			return nil, fmt.Errorf("'url_expr' must return a string: %s", err)
		}
	}

	stepStats := &StepExecutionStats{}

	reqBody := ""

	if l.RequestBody != nil && l.RequestBody.Value.Valid {
		reqBody = l.RequestBody.Value.String
	} else if l.RequestBody != nil && l.RequestBody.Expr.Valid {
		val, err := l.RequestBody.Expr.Execute(vm)
		if err != nil {
			return stepStats, fmt.Errorf("error executing 'expr' for 'request_body': %s", err)
		}

		strVal, err := val.ToString()
		if err != nil {
			return stepStats, fmt.Errorf("'expr' for 'request_body' must return a string: %s", err)
		}

		reqBody = strVal
	}

	reqBodyBuffer := bytes.NewBufferString(reqBody)

	qctx := context.Background()
	if l.Timeout.Valid {
		timeout, err := time.ParseDuration(l.Timeout.String)
		if err != nil {
			return stepStats, fmt.Errorf("can't parse 'timeout': %s", err)
		}

		var cancel func()

		qctx, cancel = context.WithTimeout(qctx, timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(qctx, string(l.Method), url, reqBodyBuffer)
	if err != nil {
		return stepStats, fmt.Errorf("can't create http request: %s", err)
	}

	dumpRequest, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		return stepStats, fmt.Errorf("can't analyze http request: %s", err)
	}

	stepStats.BytesSent.Scan(len(dumpRequest))

	for _, header := range l.Headers {
		if header.Name == "" {
			return stepStats, fmt.Errorf("header item must have a name")
		}

		if !header.Value.Valid && !header.Expr.Valid {
			return stepStats, fmt.Errorf("header '%s' must have a child of 'value' | 'expr'", header.Name)
		}

		if header.Value.Valid {
			req.Header.Add(header.Name, header.Value.String)
		} else if header.Expr.Valid {
			val, err := header.Expr.Execute(vm)
			if err != nil {
				return stepStats, fmt.Errorf("error executing 'expr' for header '%s': %s", header.Name, err)
			}

			strVal, err := val.ToString()
			if err != nil {
				return stepStats, fmt.Errorf("'expr' for header '%s' must return a string: %s", header.Name, err)
			}

			req.Header.Add(header.Name, strVal)
		}
	}

	startReq := time.Now()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return stepStats, fmt.Errorf("can't execute http request: %s", err)
	}
	defer resp.Body.Close()

	durationReq := time.Since(startReq)
	stepStats.DurationRequest = &durationReq

	startResp := time.Now()

	stepStats.Code.Scan(fmt.Sprintf("%d", resp.StatusCode))

	dumpResponse, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return stepStats, fmt.Errorf("can't analyze http response: %s", err)
	}

	stepStats.BytesReceived.Scan(len(dumpResponse))

	durationResp := time.Since(startResp)
	stepStats.DurationResponse = &durationResp

	err = l.assignResponseObject(resp, vm)
	if err != nil {
		return stepStats, fmt.Errorf("can't assign reponse object to vm")
	}

	for _, assertion := range l.Assertions {
		err = assertion.Verify(resp, int64(len(dumpResponse)), vm)
		if err != nil {
			return stepStats, err
		}
	}

	return stepStats, nil
}

var _ IRunnableStep = (*LoadTestStepHttp)(nil)
