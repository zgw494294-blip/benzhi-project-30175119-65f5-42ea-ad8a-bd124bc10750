package domain

import (
	"strings"
	"time"
)

type FindingDecisionInput struct {
	FindingID   string      `json:"findingId"`
	Disposition Disposition `json:"disposition"`
	Rationale   string      `json:"rationale"`
}

func validDisposition(value Disposition) bool {
	return value == DispositionMask || value == DispositionGeneralize || value == DispositionKeep || value == DispositionExclude
}

func (a *Aggregate) validateFindingDecision(decision FindingDecisionInput) error {
	if !validDisposition(decision.Disposition) {
		return NewRuleError("invalid_disposition", "处置方式必须是遮蔽、泛化、保留或排除")
	}
	if strings.TrimSpace(decision.Rationale) == "" {
		return NewRuleError("missing_rationale", "敏感候选处置必须填写理由")
	}
	if a.Case.Status == StatusRevision && !a.returnedFinding(decision.FindingID) {
		return NewRuleError("revision_scope", "整改中只能修改本轮被退回的敏感项")
	}
	if !a.hasFinding(decision.FindingID) {
		return NewRuleError("finding_not_found", "未找到指定敏感候选")
	}
	return nil
}

func (a *Aggregate) applyFindingDecision(decision FindingDecisionInput, actor string, now time.Time) {
	for index := range a.Findings {
		if a.Findings[index].ID == decision.FindingID {
			a.Findings[index].Disposition = decision.Disposition
			a.Findings[index].Rationale = strings.TrimSpace(decision.Rationale)
			resolved := now.UTC()
			a.Findings[index].ResolvedAt = &resolved
			break
		}
	}
	if a.Case.Status != StatusRevision {
		return
	}
	for index := range a.RevisionTasks {
		task := &a.RevisionTasks[index]
		if task.FindingID != decision.FindingID || task.RoundNumber != a.latestReturnedRoundNumber() || task.ResubmittedAt != nil {
			continue
		}
		task.AfterDisposition = decision.Disposition
		task.AfterRationale = strings.TrimSpace(decision.Rationale)
		if task.BeforeDisposition == task.AfterDisposition && task.BeforeRationale == task.AfterRationale {
			task.Completed = false
			task.IncompleteReason = "处置和理由与退回前完全相同，尚未形成实际整改"
			task.CompletedBy = ""
			task.CompletedAt = nil
		} else {
			task.Completed = true
			task.IncompleteReason = ""
			task.CompletedBy = strings.TrimSpace(actor)
			completed := now.UTC()
			task.CompletedAt = &completed
		}
	}
}

func (a *Aggregate) ResolveFindingBy(id string, disposition Disposition, rationale, actor string, now time.Time) error {
	if err := requireStatus(a.Case.Status, StatusConsented, StatusRevision); err != nil {
		return err
	}
	decision := FindingDecisionInput{FindingID: id, Disposition: disposition, Rationale: rationale}
	if err := a.validateFindingDecision(decision); err != nil {
		return err
	}
	a.applyFindingDecision(decision, actor, now)
	a.bump(now)
	return nil
}

func (a *Aggregate) ResolveFindings(decisions []FindingDecisionInput, actor string, now time.Time) error {
	if err := requireStatus(a.Case.Status, StatusConsented, StatusRevision); err != nil {
		return err
	}
	if len(decisions) == 0 {
		return NewRuleError("empty_finding_batch", "批量处置至少需要一个候选")
	}
	issues := make([]FieldIssue, 0)
	seen := make(map[string]bool, len(decisions))
	for index, decision := range decisions {
		row := index + 1
		if strings.TrimSpace(decision.FindingID) == "" {
			issues = append(issues, FieldIssue{Row: row, Field: "findingId", Reason: "候选标识不能为空"})
		} else if seen[decision.FindingID] {
			issues = append(issues, FieldIssue{Row: row, ItemID: decision.FindingID, Field: "findingId", Reason: "候选标识重复"})
		}
		seen[decision.FindingID] = true
		if !a.hasFinding(decision.FindingID) {
			issues = append(issues, FieldIssue{Row: row, ItemID: decision.FindingID, Field: "findingId", Reason: "候选不属于当前案件"})
		}
		if !validDisposition(decision.Disposition) {
			issues = append(issues, FieldIssue{Row: row, ItemID: decision.FindingID, Field: "disposition", Reason: "处置必须是遮蔽、泛化、保留或排除"})
		}
		if strings.TrimSpace(decision.Rationale) == "" {
			issues = append(issues, FieldIssue{Row: row, ItemID: decision.FindingID, Field: "rationale", Reason: "处置理由不能为空"})
		}
		if a.Case.Status == StatusRevision && !a.returnedFinding(decision.FindingID) {
			issues = append(issues, FieldIssue{Row: row, ItemID: decision.FindingID, Field: "findingId", Reason: "整改中只能包含本轮退回目标"})
		}
	}
	if len(issues) > 0 {
		return &ValidationError{Code: "invalid_finding_batch", Message: "批量候选处置校验失败，未保存任何决定", Issues: issues}
	}
	for _, decision := range decisions {
		a.applyFindingDecision(decision, actor, now)
	}
	a.bump(now)
	return nil
}

func (a *Aggregate) latestReturnedRoundNumber() int {
	if len(a.Reviews) == 0 || a.Reviews[len(a.Reviews)-1].Decision != ReviewReturned {
		return 0
	}
	return a.Reviews[len(a.Reviews)-1].RoundNumber
}
