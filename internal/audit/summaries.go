package audit

import (
	"dialectrelease/internal/domain"
	"sort"
)

func ScopeDigest(a *domain.Aggregate) string {
	segments := append([]domain.CorpusSegment(nil), a.Segments...)
	sort.Slice(segments, func(i, j int) bool { return segments[i].Sequence < segments[j].Sequence })
	type scopeItem struct {
		ID         string `json:"id"`
		Sequence   int    `json:"sequence"`
		Speaker    string `json:"speaker"`
		Start      int64  `json:"start"`
		End        int64  `json:"end"`
		Transcript string `json:"transcript"`
		Category   string `json:"category"`
	}
	items := make([]scopeItem, 0, len(segments))
	for _, s := range segments {
		items = append(items, scopeItem{s.ID, s.Sequence, s.SpeakerCode, s.StartMillis, s.EndMillis, s.Transcript, s.Category})
	}
	return MustDigest(items)
}

func ConsentDigest(c *domain.ConsentGrant) string {
	if c == nil {
		return ""
	}
	return MustDigest(struct {
		Scope       string `json:"scope"`
		Research    bool   `json:"research"`
		Teaching    bool   `json:"teaching"`
		Public      bool   `json:"public"`
		ConfirmedBy string `json:"confirmedBy"`
	}{c.ScopeDigest, c.ResearchAllowed, c.TeachingAllowed, c.PublicDisplayAllowed, c.ConfirmedBy})
}

func SubmissionDigest(a *domain.Aggregate, manifest []domain.ManifestEntry) string {
	type decision struct {
		ID          string             `json:"id"`
		Disposition domain.Disposition `json:"disposition"`
		Rationale   string             `json:"rationale"`
	}
	d := make([]decision, 0, len(a.Findings))
	for _, f := range a.Findings {
		d = append(d, decision{f.ID, f.Disposition, f.Rationale})
	}
	sort.Slice(d, func(i, j int) bool { return d[i].ID < d[j].ID })
	return MustDigest(struct {
		CaseID    string                 `json:"caseId"`
		Consent   string                 `json:"consent"`
		Manifest  []domain.ManifestEntry `json:"manifest"`
		Decisions []decision             `json:"decisions"`
	}{a.Case.ID, ConsentDigest(a.Consent), manifest, d})
}

func ApprovalDigest(r *domain.ReviewRound) string {
	if r == nil {
		return ""
	}
	return MustDigest(struct {
		Round      int                   `json:"round"`
		Submission string                `json:"submission"`
		Reviewer   string                `json:"reviewer"`
		Decision   domain.ReviewDecision `json:"decision"`
		Comments   string                `json:"comments"`
	}{r.RoundNumber, r.SubmissionDigest, r.ReviewerCode, r.Decision, r.Comments})
}
