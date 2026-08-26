package workflow

import (
	"context"
	"dialectrelease/internal/audit"
	"dialectrelease/internal/domain"
	"time"
)

func (s *Service) RequestConsent(ctx context.Context, caseID string, meta CommandMeta) (*domain.Aggregate, error) {
	return s.mutate(ctx, caseID, meta, "consent.requested", caseID, "冻结当前语料范围并生成知情同意清单", func(a *domain.Aggregate, now time.Time) error { return a.RequestConsent(now) })
}

type ConfirmConsentInput struct {
	CommandMeta
	ResearchAllowed      bool   `json:"researchAllowed"`
	TeachingAllowed      bool   `json:"teachingAllowed"`
	PublicDisplayAllowed bool   `json:"publicDisplayAllowed"`
	ConfirmedBy          string `json:"confirmedBy"`
	ScopeDigest          string `json:"scopeDigest"`
}

func (s *Service) ConfirmConsent(ctx context.Context, caseID string, in ConfirmConsentInput) (*domain.Aggregate, error) {
	id := s.ids.New()
	return s.mutateDetailed(ctx, caseID, in.CommandMeta, "consent.confirmed", id, func(a *domain.Aggregate) string {
		return auditDetails(map[string]any{"scopeDigest": a.Consent.ScopeDigest, "segmentCount": len(a.Consent.FrozenScope), "purposeImpact": a.Consent.PurposeImpact, "consentDigest": a.Consent.ConsentDigest})
	}, func(a *domain.Aggregate, now time.Time) error {
		currentDigest := audit.ScopeDigest(a)
		grant := domain.ConsentGrant{ID: id, ScopeDigest: in.ScopeDigest, ResearchAllowed: in.ResearchAllowed, TeachingAllowed: in.TeachingAllowed, PublicDisplayAllowed: in.PublicDisplayAllowed, ConfirmedBy: in.ConfirmedBy}
		if err := a.ConfirmConsentScope(grant, in.ScopeDigest, currentDigest, now); err != nil {
			return err
		}
		a.Consent.ConsentDigest = audit.ConsentDigest(a.Consent)
		return nil
	})
}
