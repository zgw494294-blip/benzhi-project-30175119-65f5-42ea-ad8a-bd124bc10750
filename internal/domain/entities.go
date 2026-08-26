package domain

import "time"

type DonationCase struct {
	ID                string     `json:"id"`
	ContributorCode   string     `json:"contributorCode"`
	CollectionContext string     `json:"collectionContext"`
	LanguageTags      []string   `json:"languageTags"`
	IntendedAudience  string     `json:"intendedAudience"`
	Status            CaseStatus `json:"status"`
	Version           int64      `json:"version"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type CorpusSegment struct {
	ID          string `json:"id"`
	CaseID      string `json:"caseId"`
	Sequence    int    `json:"sequence"`
	SpeakerCode string `json:"speakerCode"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	Transcript  string `json:"transcript"`
	Category    string `json:"category"`
	Revision    int    `json:"revision"`
}

type ConsentGrant struct {
	ID                   string                 `json:"id"`
	CaseID               string                 `json:"caseId"`
	ScopeDigest          string                 `json:"scopeDigest"`
	ResearchAllowed      bool                   `json:"researchAllowed"`
	TeachingAllowed      bool                   `json:"teachingAllowed"`
	PublicDisplayAllowed bool                   `json:"publicDisplayAllowed"`
	ConfirmedBy          string                 `json:"confirmedBy"`
	ConfirmedAt          time.Time              `json:"confirmedAt"`
	FrozenScope          []ConsentScopeItem     `json:"frozenScope"`
	PurposeImpact        []ConsentPurposeImpact `json:"purposeImpact"`
	ConsentDigest        string                 `json:"consentDigest"`
}

type ConsentScopeItem struct {
	SegmentID         string `json:"segmentId"`
	Sequence          int    `json:"sequence"`
	SpeakerCode       string `json:"speakerCode"`
	StartMillis       int64  `json:"startMillis"`
	EndMillis         int64  `json:"endMillis"`
	Category          string `json:"category"`
	TranscriptSummary string `json:"transcriptSummary"`
}

type ConsentPurposeImpact struct {
	Code       string `json:"code"`
	Label      string `json:"label"`
	Allowed    bool   `json:"allowed"`
	Conclusion string `json:"conclusion"`
	Blocking   bool   `json:"blocking"`
}

type FindingType string

const (
	FindingPerson     FindingType = "person_name"
	FindingLocation   FindingType = "precise_location"
	FindingContact    FindingType = "contact"
	FindingThirdParty FindingType = "third_party"
)

type Disposition string

const (
	DispositionPending    Disposition = "pending"
	DispositionMask       Disposition = "mask"
	DispositionGeneralize Disposition = "generalize"
	DispositionKeep       Disposition = "keep"
	DispositionExclude    Disposition = "exclude"
)

type SensitivityFinding struct {
	ID          string      `json:"id"`
	SegmentID   string      `json:"segmentId"`
	FindingType FindingType `json:"findingType"`
	Start       int         `json:"start"`
	End         int         `json:"end"`
	Evidence    string      `json:"evidence"`
	RuleVersion string      `json:"ruleVersion"`
	Disposition Disposition `json:"disposition"`
	Rationale   string      `json:"rationale"`
	ResolvedAt  *time.Time  `json:"resolvedAt,omitempty"`
}

type ScanRun struct {
	RuleVersion         string               `json:"ruleVersion"`
	ExecutedAt          time.Time            `json:"executedAt"`
	FindingSetDigest    string               `json:"findingSetDigest"`
	AddedCount          int                  `json:"addedCount"`
	UnchangedCount      int                  `json:"unchangedCount"`
	RemovedCount        int                  `json:"removedCount"`
	AddedFindingIDs     []string             `json:"addedFindingIds"`
	UnchangedFindingIDs []string             `json:"unchangedFindingIds"`
	RemovedFindings     []SensitivityFinding `json:"removedFindings"`
}

type ReviewDecision string

const (
	ReviewPending  ReviewDecision = "pending"
	ReviewReturned ReviewDecision = "returned"
	ReviewApproved ReviewDecision = "approved"
)

type ReviewRound struct {
	ID               string         `json:"id"`
	CaseID           string         `json:"caseId"`
	RoundNumber      int            `json:"roundNumber"`
	SubmissionDigest string         `json:"submissionDigest"`
	Decision         ReviewDecision `json:"decision"`
	Comments         string         `json:"comments"`
	TargetFindingIDs []string       `json:"targetFindingIds"`
	ReviewerCode     string         `json:"reviewerCode"`
	DecidedAt        *time.Time     `json:"decidedAt,omitempty"`
}

type RevisionTask struct {
	ID                 string      `json:"id"`
	ReviewRoundID      string      `json:"reviewRoundId"`
	RoundNumber        int         `json:"roundNumber"`
	FindingID          string      `json:"findingId"`
	ReviewComment      string      `json:"reviewComment"`
	BeforeDisposition  Disposition `json:"beforeDisposition"`
	BeforeRationale    string      `json:"beforeRationale"`
	Completed          bool        `json:"completed"`
	IncompleteReason   string      `json:"incompleteReason,omitempty"`
	AfterDisposition   Disposition `json:"afterDisposition,omitempty"`
	AfterRationale     string      `json:"afterRationale,omitempty"`
	CompletedBy        string      `json:"completedBy,omitempty"`
	CompletedAt        *time.Time  `json:"completedAt,omitempty"`
	ResubmittedToRound int         `json:"resubmittedToRound,omitempty"`
	ResubmittedAt      *time.Time  `json:"resubmittedAt,omitempty"`
}

type ReleaseCredential struct {
	ID             string    `json:"id"`
	CaseID         string    `json:"caseId"`
	CaseVersion    int64     `json:"caseVersion"`
	ManifestDigest string    `json:"manifestDigest"`
	ConsentDigest  string    `json:"consentDigest"`
	ApprovalDigest string    `json:"approvalDigest"`
	IssuedAt       time.Time `json:"issuedAt"`
	IntegrityHash  string    `json:"integrityHash"`
}

type AuditEvent struct {
	ID            string    `json:"id"`
	CaseID        string    `json:"caseId"`
	Actor         string    `json:"actor"`
	OccurredAt    time.Time `json:"occurredAt"`
	BeforeVersion int64     `json:"beforeVersion"`
	AfterVersion  int64     `json:"afterVersion"`
	Action        string    `json:"action"`
	TargetID      string    `json:"targetId"`
	Details       string    `json:"details"`
}
