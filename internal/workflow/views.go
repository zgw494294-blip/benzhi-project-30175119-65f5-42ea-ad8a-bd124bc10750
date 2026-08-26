package workflow

import (
	"context"
	"dialectrelease/internal/audit"
	"dialectrelease/internal/domain"
)

type CaseViews struct {
	Checklist  domain.ConsentChecklist      `json:"checklist"`
	Assessment domain.PublicationAssessment `json:"assessment"`
}

func (s *Service) Views(ctx context.Context, caseID string) (CaseViews, error) {
	a, err := s.store.Get(ctx, caseID)
	if err != nil {
		return CaseViews{}, err
	}
	return CaseViews{Checklist: a.Checklist(audit.ScopeDigest(a)), Assessment: a.AssessPublication()}, nil
}
