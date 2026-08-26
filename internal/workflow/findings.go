package workflow

import (
	"context"
	"dialectrelease/internal/domain"
	"time"
)

type FindingDecisionInput struct {
	FindingID   string             `json:"findingId"`
	Disposition domain.Disposition `json:"disposition"`
	Rationale   string             `json:"rationale"`
}

type ResolveFindingsInput struct {
	CommandMeta
	Decisions []FindingDecisionInput `json:"decisions"`
}

func (s *Service) ResolveFindings(ctx context.Context, caseID string, input ResolveFindingsInput) (*domain.Aggregate, error) {
	decisions := make([]domain.FindingDecisionInput, len(input.Decisions))
	summary := make([]map[string]any, len(input.Decisions))
	for index, decision := range input.Decisions {
		decisions[index] = domain.FindingDecisionInput{FindingID: decision.FindingID, Disposition: decision.Disposition, Rationale: decision.Rationale}
		summary[index] = map[string]any{"findingId": decision.FindingID, "disposition": decision.Disposition}
	}
	return s.mutateDetailed(ctx, caseID, input.CommandMeta, "findings.batch_resolved", caseID, func(*domain.Aggregate) string {
		return auditDetails(map[string]any{"resolvedCount": len(decisions), "decisions": summary})
	}, func(a *domain.Aggregate, now time.Time) error { return a.ResolveFindings(decisions, input.Actor, now) })
}
