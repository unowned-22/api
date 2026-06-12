package health

import "context"

type Status string

const (
	StatusOK   Status = "ok"
	StatusFail Status = "fail"
)

type CheckResult struct {
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
}

type Report struct {
	Status Status                 `json:"status"`
	Checks map[string]CheckResult `json:"checks"`
}

type Checker interface {
	Check(ctx context.Context) Report
}
