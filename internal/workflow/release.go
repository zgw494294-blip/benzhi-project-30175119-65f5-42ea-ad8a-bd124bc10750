package workflow

import (
	"context"
	"dialectrelease/internal/audit"
	"dialectrelease/internal/domain"
	"dialectrelease/internal/repository"
	"errors"
	"time"
)

func (s *Service) IssueCredential(ctx context.Context, caseID string, meta CommandMeta) (*domain.Aggregate, error) {
	id := s.ids.New()
	return s.mutate(ctx, caseID, meta, "release.issued", id, "签发一次性发布凭据", func(a *domain.Aggregate, now time.Time) error {
		if a.LatestApproval() == nil {
			return domain.NewRuleError("missing_approval", "缺少批准记录")
		}
		manifestDigest := audit.MustDigest(a.FrozenManifest)
		consentDigest := audit.ConsentDigest(a.Consent)
		approvalDigest := audit.ApprovalDigest(a.LatestApproval())
		credentialVersion := a.Case.Version + 1
		c := domain.ReleaseCredential{ID: id, CaseID: a.Case.ID, CaseVersion: credentialVersion, ManifestDigest: manifestDigest, ConsentDigest: consentDigest, ApprovalDigest: approvalDigest, IssuedAt: now.UTC()}
		c.IntegrityHash = audit.IntegrityHash(c.ManifestDigest, c.ConsentDigest, c.ApprovalDigest, c.CaseVersion, c.CaseID)
		return a.Publish(c, now)
	})
}

func (s *Service) Verify(ctx context.Context, caseID string) (audit.Verification, error) {
	a, err := s.store.Get(ctx, caseID)
	if err != nil {
		return audit.Verification{}, err
	}
	return audit.VerifyCredential(a), nil
}

func (s *Service) VerifyPresentedCredential(ctx context.Context, credential domain.ReleaseCredential) (audit.IndependentVerification, error) {
	a, err := s.store.Get(ctx, credential.CaseID)
	if errors.Is(err, repository.ErrNotFound) {
		return audit.VerificationFailure("case_not_found", "凭据所指案件不存在"), nil
	}
	if err != nil {
		return audit.IndependentVerification{}, err
	}
	if a.Credential == nil {
		return audit.VerificationFailure("credential_not_issued", "案件尚未签发发布凭据"), nil
	}
	events, err := s.store.Events(ctx, a.Case.ID)
	if err != nil {
		return audit.IndependentVerification{}, err
	}
	return audit.VerifyPresentedCredential(a, credential, events), nil
}
