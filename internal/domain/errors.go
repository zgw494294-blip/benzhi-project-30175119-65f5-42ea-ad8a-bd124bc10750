package domain

import "fmt"

type RuleError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Current any    `json:"current,omitempty"`
}

func (e *RuleError) Error() string { return e.Message }

func NewRuleError(code, message string) error { return &RuleError{Code: code, Message: message} }

type FieldIssue struct {
	Row    int    `json:"row,omitempty"`
	ItemID string `json:"itemId,omitempty"`
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

type ValidationError struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Issues  []FieldIssue `json:"issues"`
}

func (e *ValidationError) Error() string { return e.Message }

type ScopeConflict struct {
	Provided string           `json:"provided"`
	Current  ConsentChecklist `json:"current"`
}

func (e *ScopeConflict) Error() string {
	return "同意范围核对失败：scopeDigest 缺失或与当前冻结范围不一致"
}

type VersionConflict struct {
	Expected int64      `json:"expected"`
	Current  int64      `json:"current"`
	Status   CaseStatus `json:"status"`
}

func (e *VersionConflict) Error() string {
	return fmt.Sprintf("版本冲突：预期 %d，当前 %d（%s）", e.Expected, e.Current, e.Status)
}
