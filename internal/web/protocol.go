package web

import (
	"dialectrelease/internal/domain"
	"dialectrelease/internal/repository"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxBodyBytes = 1 << 20

type errorEnvelope struct {
	Error struct {
		Code    string              `json:"code"`
		Message string              `json:"message"`
		Current any                 `json:"current,omitempty"`
		Issues  []domain.FieldIssue `json:"issues,omitempty"`
	} `json:"error"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	return decodeJSONLimit(w, r, dst, maxBodyBytes)
}

func decodeJSONLimit(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return domain.NewRuleError("invalid_json", "请求 JSON 无效或字段不受支持："+err.Error())
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return domain.NewRuleError("invalid_json", "请求体只能包含一个 JSON 对象")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status, code, message := http.StatusInternalServerError, "internal_error", "服务器处理失败"
	var rule *domain.RuleError
	var validation *domain.ValidationError
	var scope *domain.ScopeConflict
	var conflict *domain.VersionConflict
	switch {
	case errors.As(err, &validation):
		status = http.StatusUnprocessableEntity
		code = validation.Code
		message = validation.Message
	case errors.As(err, &scope):
		status = http.StatusConflict
		code = "scope_mismatch"
		message = scope.Error()
	case errors.As(err, &rule):
		status = http.StatusUnprocessableEntity
		code = rule.Code
		message = rule.Message
	case errors.As(err, &conflict):
		status = http.StatusConflict
		code = "version_conflict"
		message = conflict.Error()
	case errors.Is(err, repository.ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
		message = "案件不存在"
	case errors.Is(err, repository.ErrDuplicate):
		status = http.StatusConflict
		code = "duplicate"
		message = "记录或幂等键已存在"
	}
	var out errorEnvelope
	out.Error.Code = code
	out.Error.Message = message
	if conflict != nil {
		out.Error.Current = conflict
	} else if scope != nil {
		out.Error.Current = scope.Current
	} else if rule != nil && rule.Current != nil {
		out.Error.Current = rule.Current
	}
	if validation != nil {
		out.Error.Issues = validation.Issues
	}
	writeJSON(w, status, out)
}
