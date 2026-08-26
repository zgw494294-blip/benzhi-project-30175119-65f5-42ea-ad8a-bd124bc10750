package domain

import (
	"strings"
	"time"
)

type Aggregate struct {
	Case           DonationCase         `json:"case"`
	Segments       []CorpusSegment      `json:"segments"`
	Consent        *ConsentGrant        `json:"consent,omitempty"`
	Findings       []SensitivityFinding `json:"findings"`
	ScanCompleted  bool                 `json:"scanCompleted"`
	ScanHistory    []ScanRun            `json:"scanHistory"`
	Reviews        []ReviewRound        `json:"reviews"`
	RevisionTasks  []RevisionTask       `json:"revisionTasks"`
	Credential     *ReleaseCredential   `json:"credential,omitempty"`
	FrozenManifest []ManifestEntry      `json:"frozenManifest,omitempty"`
}

func NewAggregate(id, contributor, context string, tags []string, audience string, now time.Time) (*Aggregate, error) {
	contributor, context, audience = strings.TrimSpace(contributor), strings.TrimSpace(context), strings.TrimSpace(audience)
	if contributor == "" || context == "" || audience == "" || len(tags) == 0 {
		return nil, NewRuleError("invalid_case", "贡献者代号、采集背景、语种标签和预期公开范围均为必填项")
	}
	cleanTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag = strings.TrimSpace(tag); tag != "" {
			cleanTags = append(cleanTags, tag)
		}
	}
	if len(cleanTags) == 0 {
		return nil, NewRuleError("invalid_case", "至少需要一个有效语种标签")
	}
	return &Aggregate{Case: DonationCase{ID: id, ContributorCode: contributor, CollectionContext: context, LanguageTags: cleanTags, IntendedAudience: audience, Status: StatusDraft, Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}}, nil
}

func (a *Aggregate) bump(now time.Time) { a.Case.Version++; a.Case.UpdatedAt = now.UTC() }

func (a *Aggregate) AddSegment(s CorpusSegment, now time.Time) error {
	return a.AddSegments([]CorpusSegment{s}, now)
}

func (a *Aggregate) RequestConsent(now time.Time) error {
	if err := requireStatus(a.Case.Status, StatusDraft); err != nil {
		return err
	}
	if len(a.Segments) == 0 {
		return NewRuleError("empty_scope", "提交同意前至少需要一个语料片段")
	}
	for _, s := range a.Segments {
		if s.EndMillis <= s.StartMillis || strings.TrimSpace(s.Transcript) == "" {
			return NewRuleError("invalid_scope", "存在边界或元数据不完整的片段")
		}
	}
	a.Case.Status = StatusAwaitingConsent
	a.bump(now)
	return nil
}

func (a *Aggregate) ConfirmConsent(grant ConsentGrant, scopeDigest string, now time.Time) error {
	return a.ConfirmConsentScope(grant, scopeDigest, scopeDigest, now)
}

func (a *Aggregate) ConfirmConsentScope(grant ConsentGrant, providedDigest, currentDigest string, now time.Time) error {
	if err := requireStatus(a.Case.Status, StatusAwaitingConsent); err != nil {
		return err
	}
	if strings.TrimSpace(providedDigest) == "" {
		return &ScopeConflict{Provided: providedDigest, Current: a.Checklist(currentDigest)}
	}
	if providedDigest != currentDigest {
		return &ScopeConflict{Provided: providedDigest, Current: a.Checklist(currentDigest)}
	}
	if strings.TrimSpace(grant.ConfirmedBy) == "" {
		return NewRuleError("invalid_consent", "确认人不能为空")
	}
	if !grant.ResearchAllowed && !grant.TeachingAllowed && !grant.PublicDisplayAllowed {
		return NewRuleError("empty_consent", "至少允许一种用途")
	}
	grant.CaseID = a.Case.ID
	grant.ScopeDigest = currentDigest
	grant.ConfirmedAt = now.UTC()
	grant.FrozenScope = a.ConsentScope()
	grant.PurposeImpact = ConsentImpacts(grant.ResearchAllowed, grant.TeachingAllowed, grant.PublicDisplayAllowed)
	a.Consent = &grant
	a.Case.Status = StatusConsented
	a.bump(now)
	return nil
}

func (a *Aggregate) SetFindings(findings []SensitivityFinding, now time.Time) error {
	_, err := a.ApplyScan(findings, "sensitivity-rules-1", "", now)
	return err
}

func (a *Aggregate) PendingFindings() []SensitivityFinding {
	result := []SensitivityFinding{}
	for _, f := range a.Findings {
		if f.Disposition == DispositionPending {
			result = append(result, f)
		}
	}
	return result
}

func (a *Aggregate) ResolveFinding(id string, disposition Disposition, rationale string, now time.Time) error {
	return a.ResolveFindingBy(id, disposition, rationale, "", now)
}

func (a *Aggregate) returnedFinding(id string) bool {
	if len(a.Reviews) == 0 {
		return false
	}
	r := a.Reviews[len(a.Reviews)-1]
	if r.Decision != ReviewReturned {
		return false
	}
	for _, target := range r.TargetFindingIDs {
		if target == id {
			return true
		}
	}
	return false
}
