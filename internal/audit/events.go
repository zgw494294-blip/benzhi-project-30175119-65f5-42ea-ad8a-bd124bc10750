package audit

import (
	"dialectrelease/internal/domain"
	"fmt"
	"strconv"
	"time"
)

type IDGenerator interface{ New() string }

type Recorder struct {
	ids IDGenerator
	now func() time.Time
}

type ComponentVerification struct {
	Name            string `json:"name"`
	Label           string `json:"label"`
	Consistent      bool   `json:"consistent"`
	CredentialValue string `json:"credentialValue"`
	RecomputedValue string `json:"recomputedValue"`
	Message         string `json:"message"`
}

type IndependentVerification struct {
	Valid              bool                    `json:"valid"`
	Code               string                  `json:"code"`
	Message            string                  `json:"message"`
	Components         []ComponentVerification `json:"components"`
	ManifestCount      int                     `json:"manifestCount"`
	ApprovalRound      int                     `json:"approvalRound"`
	RelatedAuditEvents []domain.AuditEvent     `json:"relatedAuditEvents"`
}

func VerificationFailure(code, message string) IndependentVerification {
	return IndependentVerification{Code: code, Message: message, Components: []ComponentVerification{}, RelatedAuditEvents: []domain.AuditEvent{}}
}

func VerifyPresentedCredential(a *domain.Aggregate, presented domain.ReleaseCredential, events []domain.AuditEvent) IndependentVerification {
	stored := a.Credential
	manifest := MustDigest(a.FrozenManifest)
	consent := ConsentDigest(a.Consent)
	approval := ApprovalDigest(a.LatestApproval())
	presentedHash := IntegrityHash(presented.ManifestDigest, presented.ConsentDigest, presented.ApprovalDigest, presented.CaseVersion, presented.CaseID)
	component := func(name, label, got, want, mismatch string) ComponentVerification {
		match := got == want
		message := "一致"
		if !match {
			message = mismatch
		}
		return ComponentVerification{Name: name, Label: label, Consistent: match, CredentialValue: got, RecomputedValue: want, Message: message}
	}
	identityGot := presented.CaseID + "/" + presented.ID
	identityWant := a.Case.ID + "/" + stored.ID
	components := []ComponentVerification{
		component("credentialIdentity", "凭据身份", identityGot, identityWant, "案件标识或凭据标识不匹配"),
		component("caseVersion", "案件版本", strconv.FormatInt(presented.CaseVersion, 10), strconv.FormatInt(stored.CaseVersion, 10), "凭据版本与已签发批准版本不匹配"),
		component("manifestDigest", "发布清单摘要", presented.ManifestDigest, manifest, "发布清单摘要被改动或与冻结清单不一致"),
		component("consentDigest", "同意摘要", presented.ConsentDigest, consent, "同意摘要被改动或与冻结基线不一致"),
		component("approvalDigest", "批准摘要", presented.ApprovalDigest, approval, "批准摘要被改动或与批准记录不一致"),
		component("issuedAt", "签发时间", presented.IssuedAt.UTC().Format(time.RFC3339Nano), stored.IssuedAt.UTC().Format(time.RFC3339Nano), "签发时间与已签发凭据不一致"),
		component("integrityHash", "完整性哈希", presented.IntegrityHash, presentedHash, "完整性哈希无法覆盖所提交的凭据组成"),
	}
	result := IndependentVerification{Valid: true, Code: "valid", Message: "凭据身份、冻结范围、同意、批准和完整性哈希全部一致", Components: components, ManifestCount: len(a.FrozenManifest), RelatedAuditEvents: []domain.AuditEvent{}}
	if approvalRound := a.LatestApproval(); approvalRound != nil {
		result.ApprovalRound = approvalRound.RoundNumber
	}
	for _, event := range events {
		if event.Action == "consent.confirmed" || event.Action == "review.decided" || event.Action == "release.issued" {
			result.RelatedAuditEvents = append(result.RelatedAuditEvents, event)
		}
	}
	for _, value := range components {
		if !value.Consistent {
			result.Valid = false
		}
	}
	if !result.Valid {
		result.Code = "credential_mismatch"
		result.Message = "凭据校验未通过，请查看不一致组成项定位异常"
		if !components[0].Consistent {
			result.Code = "credential_identity_mismatch"
		} else if !components[1].Consistent {
			result.Code = "version_mismatch"
		}
	}
	return result
}

func NewRecorder(ids IDGenerator, now func() time.Time) *Recorder {
	return &Recorder{ids: ids, now: now}
}

func (r *Recorder) Event(caseID, actor, action, target, details string, before, after int64) domain.AuditEvent {
	return domain.AuditEvent{ID: r.ids.New(), CaseID: caseID, Actor: actor, OccurredAt: r.now().UTC(), BeforeVersion: before, AfterVersion: after, Action: action, TargetID: target, Details: details}
}

type Verification struct {
	Valid                bool     `json:"valid"`
	Message              string   `json:"message"`
	MismatchedComponents []string `json:"mismatchedComponents"`
	RecomputedHash       string   `json:"recomputedHash"`
}

func VerifyCredential(a *domain.Aggregate) Verification {
	if a.Credential == nil {
		return Verification{Message: "案件尚未签发发布凭据", MismatchedComponents: []string{"credential"}}
	}
	c := a.Credential
	mismatches := []string{}
	manifest := MustDigest(a.FrozenManifest)
	if manifest != c.ManifestDigest {
		mismatches = append(mismatches, "manifestDigest")
	}
	consent := ConsentDigest(a.Consent)
	if consent != c.ConsentDigest {
		mismatches = append(mismatches, "consentDigest")
	}
	approval := ApprovalDigest(a.LatestApproval())
	if approval != c.ApprovalDigest {
		mismatches = append(mismatches, "approvalDigest")
	}
	hash := IntegrityHash(c.ManifestDigest, c.ConsentDigest, c.ApprovalDigest, c.CaseVersion, c.CaseID)
	if hash != c.IntegrityHash {
		mismatches = append(mismatches, "integrityHash")
	}
	if len(mismatches) > 0 {
		return Verification{Message: fmt.Sprintf("凭据校验失败：%v", mismatches), MismatchedComponents: mismatches, RecomputedHash: hash}
	}
	return Verification{Valid: true, Message: "发布凭据与冻结清单、同意基线及批准记录一致", MismatchedComponents: []string{}, RecomputedHash: hash}
}
