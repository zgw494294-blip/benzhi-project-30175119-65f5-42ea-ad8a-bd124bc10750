package web

import (
	"dialectrelease/internal/domain"
	"dialectrelease/internal/workflow"
	"net/http"
)

const maxSegmentBatchItems = 100

func (h *Handler) CreateCase(w http.ResponseWriter, r *http.Request) {
	var in workflow.CreateCaseInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	a, err := h.service.CreateCase(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}
func (h *Handler) GetCase(w http.ResponseWriter, r *http.Request) {
	a, err := h.service.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) UpdateCase(w http.ResponseWriter, r *http.Request) {
	var input workflow.UpdateCaseInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	a, err := h.service.UpdateCase(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}
func (h *Handler) AddSegment(w http.ResponseWriter, r *http.Request) {
	var in workflow.AddSegmentInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	a, err := h.service.AddSegment(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) UpdateSegment(w http.ResponseWriter, r *http.Request) {
	var input workflow.UpdateSegmentInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	a, err := h.service.UpdateSegment(r.Context(), r.PathValue("id"), r.PathValue("segmentID"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) AddSegments(w http.ResponseWriter, r *http.Request) {
	var input workflow.AddSegmentsInput
	if err := decodeJSONLimit(w, r, &input, 512<<10); err != nil {
		writeError(w, err)
		return
	}
	if len(input.Segments) > maxSegmentBatchItems {
		writeError(w, domain.NewRuleError("segment_batch_too_large", "单次最多录入 100 条片段"))
		return
	}
	a, err := h.service.AddSegments(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) RevokeSegments(w http.ResponseWriter, r *http.Request) {
	var input workflow.SegmentIDsInput
	if err := decodeJSONLimit(w, r, &input, 64<<10); err != nil {
		writeError(w, err)
		return
	}
	if len(input.SegmentIDs) > maxSegmentBatchItems {
		writeError(w, domain.NewRuleError("segment_batch_too_large", "单次最多撤销 100 条片段"))
		return
	}
	a, err := h.service.RevokeSegments(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) ReorderSegments(w http.ResponseWriter, r *http.Request) {
	var input workflow.SegmentIDsInput
	if err := decodeJSONLimit(w, r, &input, 64<<10); err != nil {
		writeError(w, err)
		return
	}
	if len(input.SegmentIDs) > maxSegmentBatchItems {
		writeError(w, domain.NewRuleError("segment_batch_too_large", "单次最多重排 100 条片段"))
		return
	}
	a, err := h.service.ReorderSegments(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}
