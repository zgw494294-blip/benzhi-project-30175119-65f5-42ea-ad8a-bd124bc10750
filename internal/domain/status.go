package domain

import "fmt"

type CaseStatus string

const (
	StatusDraft           CaseStatus = "draft"
	StatusAwaitingConsent CaseStatus = "awaiting_consent"
	StatusConsented       CaseStatus = "consented"
	StatusPendingReview   CaseStatus = "pending_review"
	StatusRevision        CaseStatus = "revision"
	StatusApproved        CaseStatus = "approved"
	StatusPublished       CaseStatus = "published"
)

func (s CaseStatus) Valid() bool {
	switch s {
	case StatusDraft, StatusAwaitingConsent, StatusConsented, StatusPendingReview, StatusRevision, StatusApproved, StatusPublished:
		return true
	default:
		return false
	}
}

func (s CaseStatus) Chinese() string {
	switch s {
	case StatusDraft:
		return "草拟"
	case StatusAwaitingConsent:
		return "待同意"
	case StatusConsented:
		return "已同意"
	case StatusPendingReview:
		return "待复核"
	case StatusRevision:
		return "整改中"
	case StatusApproved:
		return "已批准"
	case StatusPublished:
		return "已发布"
	default:
		return string(s)
	}
}

func requireStatus(got CaseStatus, allowed ...CaseStatus) error {
	for _, want := range allowed {
		if got == want {
			return nil
		}
	}
	return &RuleError{Code: "invalid_status", Message: fmt.Sprintf("当前状态“%s”不允许此操作", got), Current: map[string]any{"status": got}}
}
