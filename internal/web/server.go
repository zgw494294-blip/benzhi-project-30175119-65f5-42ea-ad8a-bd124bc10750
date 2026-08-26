package web

import (
	"dialectrelease/internal/workflow"
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var assets embed.FS

type Handler struct {
	service *workflow.Service
	mux     *http.ServeMux
	static  http.Handler
}

func New(service *workflow.Service) *Handler {
	root, _ := fs.Sub(assets, "static")
	h := &Handler{service: service, mux: http.NewServeMux(), static: http.FileServer(http.FS(root))}
	h.routes()
	return h
}

func (h *Handler) routes() {
	h.mux.HandleFunc("GET /", h.Workbench)
	h.mux.Handle("GET /assets/", http.StripPrefix("/assets/", h.static))
	h.mux.HandleFunc("POST /api/cases", h.CreateCase)
	h.mux.HandleFunc("GET /api/cases/{id}", h.GetCase)
	h.mux.HandleFunc("PATCH /api/cases/{id}", h.UpdateCase)
	h.mux.HandleFunc("GET /api/cases/{id}/views", h.CaseViews)
	h.mux.HandleFunc("POST /api/cases/{id}/segments", h.AddSegment)
	h.mux.HandleFunc("POST /api/cases/{id}/segments/batch", h.AddSegments)
	h.mux.HandleFunc("POST /api/cases/{id}/segments/revoke", h.RevokeSegments)
	h.mux.HandleFunc("POST /api/cases/{id}/segments/reorder", h.ReorderSegments)
	h.mux.HandleFunc("PUT /api/cases/{id}/segments/{segmentID}", h.UpdateSegment)
	h.mux.HandleFunc("POST /api/cases/{id}/request-consent", h.RequestConsent)
	h.mux.HandleFunc("POST /api/cases/{id}/confirm-consent", h.ConfirmConsent)
	h.mux.HandleFunc("POST /api/cases/{id}/scan", h.ScanSensitivity)
	h.mux.HandleFunc("POST /api/cases/{id}/findings/{findingID}/resolve", h.ResolveFinding)
	h.mux.HandleFunc("POST /api/cases/{id}/findings/batch", h.ResolveFindings)
	h.mux.HandleFunc("POST /api/cases/{id}/submit-review", h.SubmitReview)
	h.mux.HandleFunc("POST /api/cases/{id}/review", h.DecideReview)
	h.mux.HandleFunc("POST /api/cases/{id}/release", h.IssueCredential)
	h.mux.HandleFunc("GET /api/cases/{id}/verify", h.VerifyCredential)
	h.mux.HandleFunc("POST /api/credentials/verify", h.VerifyPresentedCredential)
	h.mux.HandleFunc("GET /api/cases/{id}/audit", h.AuditTimeline)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'")
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) Workbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := assets.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "页面资源不可用", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
