package repository

import (
	"context"
	"database/sql"
	"dialectrelease/internal/domain"
	"encoding/json"
	"errors"
	"time"
)

func (s *SQLiteStore) Get(ctx context.Context, id string) (*domain.Aggregate, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT aggregate_json FROM donation_cases WHERE id=?", id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var a domain.Aggregate
	if err = json.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *SQLiteStore) FindIdempotent(ctx context.Context, key string) (*domain.Aggregate, bool, error) {
	if key == "" {
		return nil, false, nil
	}
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT aggregate_json FROM idempotency_keys WHERE key=?", key).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var a domain.Aggregate
	if err = json.Unmarshal(b, &a); err != nil {
		return nil, false, err
	}
	return &a, true, nil
}

func (s *SQLiteStore) Events(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,case_id,actor,occurred_at,before_version,after_version,action,target_id,details FROM audit_events WHERE case_id=? ORDER BY seq`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.AuditEvent{}
	for rows.Next() {
		var e domain.AuditEvent
		var occurredAt string
		if err = rows.Scan(&e.ID, &e.CaseID, &e.Actor, &occurredAt, &e.BeforeVersion, &e.AfterVersion, &e.Action, &e.TargetID, &e.Details); err != nil {
			return nil, err
		}
		e.OccurredAt, err = time.Parse(time.RFC3339Nano, occurredAt)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
