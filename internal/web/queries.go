package web

import (
	"dialectrelease/internal/domain"
	"net/http"
	"regexp"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (h *Handler) CaseViews(w http.ResponseWriter, r *http.Request) {
	views, err := h.service.Views(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, views)
}

func (h *Handler) VerifyCredential(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Verify(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (h *Handler) VerifyPresentedCredential(w http.ResponseWriter, r *http.Request) {
	var credential domain.ReleaseCredential
	if err := decodeJSONLimit(w, r, &credential, 32<<10); err != nil {
		writeError(w, err)
		return
	}
	issues := []domain.FieldIssue{}
	if !identifierPattern.MatchString(credential.CaseID) {
		issues = append(issues, domain.FieldIssue{Field: "caseId", Reason: "案件标识格式无效"})
	}
	if !identifierPattern.MatchString(credential.ID) {
		issues = append(issues, domain.FieldIssue{Field: "id", Reason: "凭据标识格式无效"})
	}
	if credential.CaseVersion < 1 {
		issues = append(issues, domain.FieldIssue{Field: "caseVersion", Reason: "案件版本必须为正整数"})
	}
	for field, value := range map[string]string{"manifestDigest": credential.ManifestDigest, "consentDigest": credential.ConsentDigest, "approvalDigest": credential.ApprovalDigest, "integrityHash": credential.IntegrityHash} {
		if !digestPattern.MatchString(value) {
			issues = append(issues, domain.FieldIssue{Field: field, Reason: "必须是 64 位小写十六进制摘要"})
		}
	}
	if credential.IssuedAt.IsZero() {
		issues = append(issues, domain.FieldIssue{Field: "issuedAt", Reason: "签发时间必须是 RFC3339 时间"})
	}
	if len(issues) > 0 {
		writeError(w, &domain.ValidationError{Code: "invalid_credential", Message: "凭据对象字段格式无效", Issues: issues})
		return
	}
	result, err := h.service.VerifyPresentedCredential(r.Context(), credential)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) AuditTimeline(w http.ResponseWriter, r *http.Request) {
	events, err := h.service.Timeline(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"events": events})
}
