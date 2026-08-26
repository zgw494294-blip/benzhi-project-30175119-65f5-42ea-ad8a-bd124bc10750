package workflow

import (
	"context"
	"dialectrelease/internal/audit"
	"dialectrelease/internal/domain"
	"time"
)

func (s *Service) SubmitReview(ctx context.Context, caseID string, meta CommandMeta) (*domain.Aggregate, error) {
	id := s.ids.New()
	wasRevision := false
	return s.mutateDetailed(ctx, caseID, meta, "review.submitted", id, func(a *domain.Aggregate) string {
		details := map[string]any{"roundNumber": len(a.Reviews), "submissionDigest": a.Reviews[len(a.Reviews)-1].SubmissionDigest}
		if wasRevision {
			tasks := []map[string]any{}
			for _, task := range a.RevisionTasks {
				if task.ResubmittedToRound == len(a.Reviews) {
					tasks = append(tasks, map[string]any{"findingId": task.FindingID, "beforeDisposition": task.BeforeDisposition, "afterDisposition": task.AfterDisposition, "completedBy": task.CompletedBy})
				}
			}
			details["revisionTasks"] = tasks
		}
		return auditDetails(details)
	}, func(a *domain.Aggregate, now time.Time) error {
		wasRevision = a.Case.Status == domain.StatusRevision
		if err := a.ValidateReviewSubmission(); err != nil {
			return err
		}
		manifest, err := a.BuildManifest()
		if err != nil {
			return err
		}
		return a.SubmitReviewBy(id, audit.SubmissionDigest(a, manifest), meta.Actor, now)
	})
}

type ReviewInput struct {
	CommandMeta
	Decision         domain.ReviewDecision `json:"decision"`
	Comments         string                `json:"comments"`
	TargetFindingIDs []string              `json:"targetFindingIds"`
	ReviewerCode     string                `json:"reviewerCode"`
}

func (s *Service) DecideReview(ctx context.Context, caseID string, in ReviewInput) (*domain.Aggregate, error) {
	return s.mutateDetailed(ctx, caseID, in.CommandMeta, "review.decided", caseID, func(a *domain.Aggregate) string {
		round := a.Reviews[len(a.Reviews)-1]
		details := map[string]any{"roundNumber": round.RoundNumber, "decision": round.Decision, "reviewerCode": round.ReviewerCode, "comments": round.Comments, "targetFindingIds": round.TargetFindingIDs}
		if round.Decision == domain.ReviewReturned {
			tasks := []map[string]any{}
			for _, task := range a.RevisionTasks {
				if task.RoundNumber == round.RoundNumber {
					tasks = append(tasks, map[string]any{"findingId": task.FindingID, "beforeDisposition": task.BeforeDisposition, "beforeRationale": task.BeforeRationale})
				}
			}
			details["revisionTasks"] = tasks
		}
		return auditDetails(details)
	}, func(a *domain.Aggregate, now time.Time) error {
		var manifest []domain.ManifestEntry
		if in.Decision == domain.ReviewApproved {
			var err error
			manifest, err = a.BuildManifest()
			if err != nil {
				return err
			}
		}
		return a.DecideReview(in.Decision, in.Comments, in.TargetFindingIDs, in.ReviewerCode, manifest, now)
	})
}
