package audit

import (
	"dialectrelease/internal/domain"
	"testing"
	"time"
)

func TestCredentialVerificationReportsTamperedComponent(t *testing.T) {
	now := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	a := &domain.Aggregate{Case: domain.DonationCase{ID: "case", Version: 9, Status: domain.StatusPublished}}
	a.Consent = &domain.ConsentGrant{ScopeDigest: "scope", PublicDisplayAllowed: true, ConfirmedBy: "C"}
	a.Reviews = []domain.ReviewRound{{RoundNumber: 1, SubmissionDigest: "sub", Decision: domain.ReviewApproved, ReviewerCode: "R"}}
	a.FrozenManifest = []domain.ManifestEntry{{SegmentID: "s", Sequence: 1, PublishedText: "公开内容"}}
	a.Credential = &domain.ReleaseCredential{CaseID: "case", CaseVersion: 9, ManifestDigest: MustDigest(a.FrozenManifest), ConsentDigest: ConsentDigest(a.Consent), ApprovalDigest: ApprovalDigest(a.LatestApproval()), IssuedAt: now}
	a.Credential.IntegrityHash = IntegrityHash(a.Credential.ManifestDigest, a.Credential.ConsentDigest, a.Credential.ApprovalDigest, 9, "case")
	if result := VerifyCredential(a); !result.Valid {
		t.Fatalf("原始凭据应有效: %#v", result)
	}
	a.FrozenManifest[0].PublishedText = "被篡改"
	result := VerifyCredential(a)
	if result.Valid || len(result.MismatchedComponents) == 0 || result.MismatchedComponents[0] != "manifestDigest" {
		t.Fatalf("应明确报告 manifestDigest: %#v", result)
	}
}

func TestCanonicalDigestIsStable(t *testing.T) {
	value := []domain.ManifestEntry{{SegmentID: "b", Sequence: 2}, {SegmentID: "a", Sequence: 1}}
	if DigestA, DigestB := MustDigest(value), MustDigest(value); DigestA != DigestB {
		t.Fatalf("相同规范化输入摘要不稳定: %s != %s", DigestA, DigestB)
	}
}
