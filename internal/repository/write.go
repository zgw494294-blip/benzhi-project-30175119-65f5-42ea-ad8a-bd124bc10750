package repository

import (
	"context"
	"database/sql"
	"dialectrelease/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *SQLiteStore) Create(ctx context.Context, a *domain.Aggregate, event domain.AuditEvent, key string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	tags, _ := json.Marshal(a.Case.LanguageTags)
	_, err = tx.ExecContext(ctx, `INSERT INTO donation_cases(id,contributor_code,collection_context,language_tags,intended_audience,status,version,created_at,updated_at,aggregate_json) VALUES(?,?,?,?,?,?,?,?,?,?)`, a.Case.ID, a.Case.ContributorCode, a.Case.CollectionContext, string(tags), a.Case.IntendedAudience, a.Case.Status, a.Case.Version, a.Case.CreatedAt, a.Case.UpdatedAt, b)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ErrDuplicate
		}
		return err
	}
	if err = writeChildren(ctx, tx, a); err != nil {
		return err
	}
	if err = appendEvent(ctx, tx, event); err != nil {
		return err
	}
	if err = saveKey(ctx, tx, key, a); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) Update(ctx context.Context, a *domain.Aggregate, expected int64, event domain.AuditEvent, key string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	tags, _ := json.Marshal(a.Case.LanguageTags)
	res, err := tx.ExecContext(ctx, `UPDATE donation_cases SET contributor_code=?,collection_context=?,language_tags=?,intended_audience=?,status=?,version=?,updated_at=?,aggregate_json=? WHERE id=? AND version=?`, a.Case.ContributorCode, a.Case.CollectionContext, string(tags), a.Case.IntendedAudience, a.Case.Status, a.Case.Version, a.Case.UpdatedAt, b, a.Case.ID, expected)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		var current int64
		var status domain.CaseStatus
		if scanErr := tx.QueryRowContext(ctx, "SELECT version,status FROM donation_cases WHERE id=?", a.Case.ID).Scan(&current, &status); errors.Is(scanErr, sql.ErrNoRows) {
			return ErrNotFound
		} else if scanErr != nil {
			return scanErr
		}
		return &domain.VersionConflict{Expected: expected, Current: current, Status: status}
	}
	if err = clearChildren(ctx, tx, a.Case.ID); err != nil {
		return err
	}
	if err = writeChildren(ctx, tx, a); err != nil {
		return err
	}
	if err = appendEvent(ctx, tx, event); err != nil {
		return err
	}
	if err = saveKey(ctx, tx, key, a); err != nil {
		return err
	}
	return tx.Commit()
}

func clearChildren(ctx context.Context, tx *sql.Tx, caseID string) error {
	for _, table := range []string{"release_credentials", "review_rounds", "sensitivity_findings", "consent_grants", "corpus_segments"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE case_id=?", caseID); err != nil {
			return err
		}
	}
	return nil
}

func writeChildren(ctx context.Context, tx *sql.Tx, a *domain.Aggregate) error {
	for _, x := range a.Segments {
		if _, err := tx.ExecContext(ctx, `INSERT INTO corpus_segments(id,case_id,sequence_no,speaker_code,start_millis,end_millis,transcript,category,revision) VALUES(?,?,?,?,?,?,?,?,?)`, x.ID, a.Case.ID, x.Sequence, x.SpeakerCode, x.StartMillis, x.EndMillis, x.Transcript, x.Category, x.Revision); err != nil {
			return err
		}
	}
	if x := a.Consent; x != nil {
		if _, err := tx.ExecContext(ctx, `INSERT INTO consent_grants VALUES(?,?,?,?,?,?,?,?)`, x.ID, a.Case.ID, x.ScopeDigest, x.ResearchAllowed, x.TeachingAllowed, x.PublicDisplayAllowed, x.ConfirmedBy, x.ConfirmedAt); err != nil {
			return err
		}
	}
	for _, x := range a.Findings {
		var resolved any
		if x.ResolvedAt != nil {
			resolved = *x.ResolvedAt
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO sensitivity_findings VALUES(?,?,?,?,?,?,?,?,?,?,?)`, x.ID, a.Case.ID, x.SegmentID, x.FindingType, x.Start, x.End, x.Evidence, x.RuleVersion, x.Disposition, x.Rationale, resolved); err != nil {
			return err
		}
	}
	for _, x := range a.Reviews {
		targets, _ := json.Marshal(x.TargetFindingIDs)
		var decided any
		if x.DecidedAt != nil {
			decided = *x.DecidedAt
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO review_rounds VALUES(?,?,?,?,?,?,?,?,?)`, x.ID, a.Case.ID, x.RoundNumber, x.SubmissionDigest, x.Decision, x.Comments, string(targets), x.ReviewerCode, decided); err != nil {
			return err
		}
	}
	if x := a.Credential; x != nil {
		if _, err := tx.ExecContext(ctx, `INSERT INTO release_credentials VALUES(?,?,?,?,?,?,?,?)`, x.ID, a.Case.ID, x.CaseVersion, x.ManifestDigest, x.ConsentDigest, x.ApprovalDigest, x.IssuedAt, x.IntegrityHash); err != nil {
			return fmt.Errorf("保存唯一发布凭据: %w", err)
		}
	}
	return nil
}

func appendEvent(ctx context.Context, tx *sql.Tx, e domain.AuditEvent) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_events(id,case_id,actor,occurred_at,before_version,after_version,action,target_id,details) VALUES(?,?,?,?,?,?,?,?,?)`, e.ID, e.CaseID, e.Actor, e.OccurredAt.Format(time.RFC3339Nano), e.BeforeVersion, e.AfterVersion, e.Action, e.TargetID, e.Details)
	return err
}
func saveKey(ctx context.Context, tx *sql.Tx, key string, a *domain.Aggregate) error {
	if key == "" {
		return nil
	}
	b, _ := json.Marshal(a)
	_, err := tx.ExecContext(ctx, `INSERT INTO idempotency_keys(key,case_id,aggregate_json,created_at) VALUES(?,?,?,?)`, key, a.Case.ID, b, a.Case.UpdatedAt)
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return ErrDuplicate
	}
	return err
}
