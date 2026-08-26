package repository

import (
	"context"
	"database/sql"
	"fmt"
)

const schemaVersion = 1

var migrations = []string{`
CREATE TABLE IF NOT EXISTS schema_version(version INTEGER NOT NULL);
INSERT INTO schema_version(version) SELECT 0 WHERE NOT EXISTS(SELECT 1 FROM schema_version);
CREATE TABLE IF NOT EXISTS donation_cases(
 id TEXT PRIMARY KEY, contributor_code TEXT NOT NULL, collection_context TEXT NOT NULL,
 language_tags TEXT NOT NULL, intended_audience TEXT NOT NULL, status TEXT NOT NULL,
 version INTEGER NOT NULL CHECK(version>0), created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 aggregate_json BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS corpus_segments(
 id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES donation_cases(id) ON DELETE CASCADE,
 sequence_no INTEGER NOT NULL, speaker_code TEXT NOT NULL, start_millis INTEGER NOT NULL,
 end_millis INTEGER NOT NULL, transcript TEXT NOT NULL, category TEXT NOT NULL, revision INTEGER NOT NULL,
 UNIQUE(case_id,sequence_no)
);
CREATE TABLE IF NOT EXISTS consent_grants(
 id TEXT PRIMARY KEY, case_id TEXT NOT NULL UNIQUE REFERENCES donation_cases(id) ON DELETE CASCADE,
 scope_digest TEXT NOT NULL, research_allowed INTEGER NOT NULL, teaching_allowed INTEGER NOT NULL,
 public_display_allowed INTEGER NOT NULL, confirmed_by TEXT NOT NULL, confirmed_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sensitivity_findings(
 id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES donation_cases(id) ON DELETE CASCADE,
 segment_id TEXT NOT NULL, finding_type TEXT NOT NULL, range_start INTEGER NOT NULL, range_end INTEGER NOT NULL,
 evidence TEXT NOT NULL, rule_version TEXT NOT NULL, disposition TEXT NOT NULL, rationale TEXT NOT NULL, resolved_at TEXT
);
CREATE TABLE IF NOT EXISTS review_rounds(
 id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES donation_cases(id) ON DELETE CASCADE,
 round_number INTEGER NOT NULL, submission_digest TEXT NOT NULL, decision TEXT NOT NULL, comments TEXT NOT NULL,
 target_finding_ids TEXT NOT NULL, reviewer_code TEXT NOT NULL, decided_at TEXT, UNIQUE(case_id,round_number)
);
CREATE TABLE IF NOT EXISTS release_credentials(
 id TEXT PRIMARY KEY, case_id TEXT NOT NULL UNIQUE REFERENCES donation_cases(id) ON DELETE CASCADE,
 case_version INTEGER NOT NULL, manifest_digest TEXT NOT NULL, consent_digest TEXT NOT NULL,
 approval_digest TEXT NOT NULL, issued_at TEXT NOT NULL, integrity_hash TEXT NOT NULL,
 UNIQUE(case_id,case_version)
);
CREATE TABLE IF NOT EXISTS audit_events(
 seq INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL UNIQUE, case_id TEXT NOT NULL REFERENCES donation_cases(id),
 actor TEXT NOT NULL, occurred_at TEXT NOT NULL, before_version INTEGER NOT NULL, after_version INTEGER NOT NULL,
 action TEXT NOT NULL, target_id TEXT NOT NULL, details TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS idempotency_keys(
 key TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES donation_cases(id), aggregate_json BLOB NOT NULL, created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_case ON audit_events(case_id,seq);
CREATE INDEX IF NOT EXISTS idx_finding_case ON sensitivity_findings(case_id);
UPDATE schema_version SET version=1;
`}

func migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	err = tx.QueryRowContext(ctx, "SELECT version FROM schema_version LIMIT 1").Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		if _, execErr := tx.ExecContext(ctx, migrations[0]); execErr != nil {
			return fmt.Errorf("初始化数据库模式: %w", execErr)
		}
		return tx.Commit()
	}
	if version > schemaVersion {
		return fmt.Errorf("数据库模式版本 %d 高于程序支持版本 %d", version, schemaVersion)
	}
	if version < schemaVersion {
		if _, err = tx.ExecContext(ctx, migrations[0]); err != nil {
			return fmt.Errorf("迁移数据库: %w", err)
		}
	}
	return tx.Commit()
}
