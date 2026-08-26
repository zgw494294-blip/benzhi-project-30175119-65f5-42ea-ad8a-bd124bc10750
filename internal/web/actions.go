package web

import (
	"dialectrelease/internal/domain"
	"dialectrelease/internal/workflow"
	"net/http"
)

type mutationResponse struct {
	*domain.Aggregate
	Assessment domain.PublicationAssessment `json:"assessment"`
}

func (h *Handler) RequestConsent(w http.ResponseWriter, r *http.Request) {
	var in workflow.CommandMeta
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	a, err := h.service.RequestConsent(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, a)
}
func (h *Handler) ConfirmConsent(w http.ResponseWriter, r *http.Request) {
	var in workflow.ConfirmConsentInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	if !digestPattern.MatchString(in.ScopeDigest) {
		views, viewErr := h.service.Views(r.Context(), r.PathValue("id"))
		if viewErr != nil {
			writeError(w, viewErr)
			return
		}
		writeError(w, &domain.ScopeConflict{Provided: in.ScopeDigest, Current: views.Checklist})
		return
	}
	a, err := h.service.ConfirmConsent(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, a)
}
func (h *Handler) ScanSensitivity(w http.ResponseWriter, r *http.Request) {
	var in workflow.CommandMeta
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	a, err := h.service.Scan(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, mutationResponse{Aggregate: a, Assessment: a.AssessPublication()})
}

func (h *Handler) ResolveFindings(w http.ResponseWriter, r *http.Request) {
	var input workflow.ResolveFindingsInput
	if err := decodeJSONLimit(w, r, &input, 256<<10); err != nil {
		writeError(w, err)
		return
	}
	if len(input.Decisions) > 200 {
		writeError(w, domain.NewRuleError("finding_batch_too_large", "单次最多处置 200 个敏感候选"))
		return
	}
	a, err := h.service.ResolveFindings(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mutationResponse{Aggregate: a, Assessment: a.AssessPublication()})
}
func (h *Handler) ResolveFinding(w http.ResponseWriter, r *http.Request) {
	var in workflow.ResolveFindingInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	a, err := h.service.ResolveFinding(r.Context(), r.PathValue("id"), r.PathValue("findingID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, a)
}
func (h *Handler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	var in workflow.CommandMeta
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	a, err := h.service.SubmitReview(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, a)
}
func (h *Handler) DecideReview(w http.ResponseWriter, r *http.Request) {
	var in workflow.ReviewInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	a, err := h.service.DecideReview(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, a)
}
func (h *Handler) IssueCredential(w http.ResponseWriter, r *http.Request) {
	var in workflow.CommandMeta
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	a, err := h.service.IssueCredential(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, a)
}
