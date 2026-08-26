package domain

import (
	"strings"
	"time"
)

func (a *Aggregate) SubmitReview(id, digest string, now time.Time) error {
	return a.SubmitReviewBy(id, digest, "", now)
}

func (a *Aggregate) SubmitReviewBy(id, digest, actor string, now time.Time) error {
	if err := a.ValidateReviewSubmission(); err != nil {
		return err
	}
	if a.Case.Status == StatusRevision {
		latest := a.latestReturnedRoundNumber()
		resubmitted := now.UTC()
		for index := range a.RevisionTasks {
			task := &a.RevisionTasks[index]
			if task.RoundNumber == latest && task.ResubmittedAt == nil {
				task.ResubmittedAt = &resubmitted
				task.ResubmittedToRound = len(a.Reviews) + 1
				if task.CompletedBy == "" {
					task.CompletedBy = strings.TrimSpace(actor)
				}
			}
		}
	}
	a.Reviews = append(a.Reviews, ReviewRound{ID: id, CaseID: a.Case.ID, RoundNumber: len(a.Reviews) + 1, SubmissionDigest: digest, Decision: ReviewPending})
	a.Case.Status = StatusPendingReview
	a.bump(now)
	return nil
}

func (a *Aggregate) ValidateReviewSubmission() error {
	if err := requireStatus(a.Case.Status, StatusConsented, StatusRevision); err != nil {
		return err
	}
	if !a.ScanCompleted {
		return NewRuleError("scan_required", "提交伦理复核前必须执行本地敏感信息扫描")
	}
	if a.Consent == nil {
		return NewRuleError("missing_consent", "缺少已确认的同意基线")
	}
	if a.Case.Status == StatusRevision {
		issues := make([]FieldIssue, 0)
		latest := a.latestReturnedRoundNumber()
		for _, task := range a.RevisionTasks {
			if task.RoundNumber == latest && task.ResubmittedAt == nil && !task.Completed {
				reason := task.IncompleteReason
				if reason == "" {
					reason = "尚未提交有效的新处置和非空理由"
				}
				issues = append(issues, FieldIssue{ItemID: task.FindingID, Field: "revisionTask", Reason: reason})
			}
		}
		if len(issues) > 0 {
			return &ValidationError{Code: "incomplete_revision_tasks", Message: "仍有未完成的定向整改任务，不能重新提交复核", Issues: issues}
		}
	}
	if len(a.PendingFindings()) > 0 {
		return NewRuleError("unresolved_findings", "仍有未处置的敏感候选，不能提交复核")
	}
	return nil
}

func (a *Aggregate) DecideReview(decision ReviewDecision, comments string, targetIDs []string, reviewer string, approvalManifest []ManifestEntry, now time.Time) error {
	if err := requireStatus(a.Case.Status, StatusPendingReview); err != nil {
		return err
	}
	if len(a.Reviews) == 0 || a.Reviews[len(a.Reviews)-1].Decision != ReviewPending {
		return NewRuleError("missing_review", "没有待决定的复核轮次")
	}
	if strings.TrimSpace(reviewer) == "" {
		return NewRuleError("missing_reviewer", "复核员代号不能为空")
	}
	r := &a.Reviews[len(a.Reviews)-1]
	t := now.UTC()
	r.ReviewerCode = reviewer
	r.Comments = strings.TrimSpace(comments)
	r.DecidedAt = &t
	switch decision {
	case ReviewReturned:
		if len(targetIDs) == 0 || r.Comments == "" {
			return NewRuleError("invalid_return", "退回时必须选择具体敏感项并填写意见")
		}
		seen := map[string]bool{}
		for _, id := range targetIDs {
			if seen[id] {
				return NewRuleError("duplicate_target", "退回目标不得包含重复敏感项")
			}
			seen[id] = true
			if !a.hasFinding(id) {
				return NewRuleError("invalid_target", "退回目标包含不存在的敏感项")
			}
		}
		r.Decision = decision
		r.TargetFindingIDs = append([]string(nil), targetIDs...)
		for index := range a.Findings {
			for _, targetID := range targetIDs {
				if a.Findings[index].ID == targetID {
					finding := a.Findings[index]
					a.RevisionTasks = append(a.RevisionTasks, RevisionTask{ID: "task-" + r.ID + "-" + finding.ID, ReviewRoundID: r.ID, RoundNumber: r.RoundNumber, FindingID: finding.ID, ReviewComment: r.Comments, BeforeDisposition: finding.Disposition, BeforeRationale: finding.Rationale, IncompleteReason: "尚未提交有效的新处置和非空理由"})
					a.Findings[index].Disposition = DispositionPending
					a.Findings[index].Rationale = ""
					a.Findings[index].ResolvedAt = nil
				}
			}
		}
		a.Case.Status = StatusRevision
	case ReviewApproved:
		if len(a.PendingFindings()) > 0 {
			return NewRuleError("unresolved_findings", "仍有未决阻断项")
		}
		if a.Consent == nil || !a.Consent.PublicDisplayAllowed {
			return NewRuleError("consent_coverage", "公开发布未被贡献者允许")
		}
		r.Decision = decision
		a.FrozenManifest = append([]ManifestEntry(nil), approvalManifest...)
		a.Case.Status = StatusApproved
	default:
		return NewRuleError("invalid_decision", "复核结论必须是退回或批准")
	}
	a.bump(now)
	return nil
}

func (a *Aggregate) hasFinding(id string) bool {
	for _, f := range a.Findings {
		if f.ID == id {
			return true
		}
	}
	return false
}

func (a *Aggregate) Publish(c ReleaseCredential, now time.Time) error {
	if err := requireStatus(a.Case.Status, StatusApproved); err != nil {
		return err
	}
	if a.Credential != nil {
		return NewRuleError("credential_exists", "当前批准版本已签发发布凭据")
	}
	if len(a.FrozenManifest) == 0 {
		return NewRuleError("empty_manifest", "冻结发布清单不能为空")
	}
	a.Credential = &c
	a.Case.Status = StatusPublished
	a.bump(now)
	return nil
}

func (a *Aggregate) LatestApproval() *ReviewRound {
	for i := len(a.Reviews) - 1; i >= 0; i-- {
		if a.Reviews[i].Decision == ReviewApproved {
			return &a.Reviews[i]
		}
	}
	return nil
}
