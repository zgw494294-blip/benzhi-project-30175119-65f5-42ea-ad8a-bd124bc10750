package domain

type ConsentPurpose struct {
	Code    string `json:"code"`
	Label   string `json:"label"`
	Allowed *bool  `json:"allowed,omitempty"`
}

type ConsentChecklist struct {
	CaseID        string                 `json:"caseId"`
	ScopeDigest   string                 `json:"scopeDigest"`
	SegmentCount  int                    `json:"segmentCount"`
	SegmentIDs    []string               `json:"segmentIds"`
	Segments      []ConsentScopeItem     `json:"segments"`
	Purposes      []ConsentPurpose       `json:"purposes"`
	PurposeImpact []ConsentPurposeImpact `json:"purposeImpact"`
	BaselineFixed bool                   `json:"baselineFixed"`
	Receipt       *ConsentReceipt        `json:"receipt,omitempty"`
}

type ConsentReceipt struct {
	ConsentID      string                 `json:"consentId"`
	ScopeDigest    string                 `json:"scopeDigest"`
	ConsentDigest  string                 `json:"consentDigest"`
	ConfirmedBy    string                 `json:"confirmedBy"`
	ConfirmedAt    string                 `json:"confirmedAt"`
	FrozenSegments []ConsentScopeItem     `json:"frozenSegments"`
	PurposeImpact  []ConsentPurposeImpact `json:"purposeImpact"`
}

func (a *Aggregate) Checklist(scopeDigest string) ConsentChecklist {
	ids := make([]string, 0, len(a.Segments))
	for _, segment := range a.Segments {
		ids = append(ids, segment.ID)
	}
	result := ConsentChecklist{CaseID: a.Case.ID, ScopeDigest: scopeDigest, SegmentCount: len(ids), SegmentIDs: ids, Segments: a.ConsentScope()}
	result.Purposes = []ConsentPurpose{{Code: "research", Label: "研究用途"}, {Code: "teaching", Label: "教学用途"}, {Code: "public_display", Label: "公开展示"}}
	result.PurposeImpact = ConsentImpacts(false, false, false)
	if a.Consent != nil {
		research, teaching, public := a.Consent.ResearchAllowed, a.Consent.TeachingAllowed, a.Consent.PublicDisplayAllowed
		result.Purposes[0].Allowed, result.Purposes[1].Allowed, result.Purposes[2].Allowed = &research, &teaching, &public
		result.ScopeDigest = a.Consent.ScopeDigest
		result.Segments = append([]ConsentScopeItem(nil), a.Consent.FrozenScope...)
		result.SegmentCount = len(result.Segments)
		result.SegmentIDs = make([]string, 0, len(result.Segments))
		for _, item := range result.Segments {
			result.SegmentIDs = append(result.SegmentIDs, item.SegmentID)
		}
		result.PurposeImpact = append([]ConsentPurposeImpact(nil), a.Consent.PurposeImpact...)
		result.BaselineFixed = true
		result.Receipt = &ConsentReceipt{ConsentID: a.Consent.ID, ScopeDigest: a.Consent.ScopeDigest, ConsentDigest: a.Consent.ConsentDigest, ConfirmedBy: a.Consent.ConfirmedBy, ConfirmedAt: a.Consent.ConfirmedAt.Format("2006-01-02T15:04:05.999999999Z07:00"), FrozenSegments: append([]ConsentScopeItem(nil), a.Consent.FrozenScope...), PurposeImpact: append([]ConsentPurposeImpact(nil), a.Consent.PurposeImpact...)}
	}
	return result
}

type PublicationBlocker struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	FindingID string `json:"findingId,omitempty"`
}

type PublicationAssessment struct {
	Ready        bool                 `json:"ready"`
	Frozen       bool                 `json:"frozen"`
	Manifest     []ManifestEntry      `json:"manifest"`
	Blockers     []PublicationBlocker `json:"blockers"`
	PendingCount int                  `json:"pendingCount"`
}

func (a *Aggregate) AssessPublication() PublicationAssessment {
	result := PublicationAssessment{Manifest: []ManifestEntry{}, Blockers: []PublicationBlocker{}}
	if a.Consent == nil {
		result.Blockers = append(result.Blockers, PublicationBlocker{Code: "missing_consent", Message: "尚未确认贡献者同意"})
	} else if !a.Consent.PublicDisplayAllowed {
		result.Blockers = append(result.Blockers, PublicationBlocker{Code: "public_not_allowed", Message: "贡献者未允许公开展示用途"})
	}
	if !a.ScanCompleted {
		result.Blockers = append(result.Blockers, PublicationBlocker{Code: "scan_required", Message: "尚未执行本地敏感信息扫描"})
	}
	for _, finding := range a.Findings {
		if finding.Disposition == DispositionPending {
			result.PendingCount++
			result.Blockers = append(result.Blockers, PublicationBlocker{Code: "finding_pending", Message: "敏感候选尚未处置", FindingID: finding.ID})
		}
	}
	if len(result.Blockers) == 0 {
		manifest, err := a.BuildManifest()
		if err != nil {
			result.Blockers = append(result.Blockers, PublicationBlocker{Code: "manifest_invalid", Message: err.Error()})
		} else {
			result.Manifest = manifest
			result.Ready = true
		}
	}
	if a.Case.Status == StatusApproved || a.Case.Status == StatusPublished {
		result.Frozen = true
		result.Manifest = append([]ManifestEntry(nil), a.FrozenManifest...)
	}
	return result
}
