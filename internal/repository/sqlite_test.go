package repository

import (
	"context"
	"dialectrelease/internal/domain"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLitePersistsAggregateAuditAndIdempotency(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cases.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 26, 1, 2, 3, 0, time.UTC)
	a, err := domain.NewAggregate("case-1", "C-1", "采集背景", []string{"赣语"}, "研究", now)
	if err != nil {
		t.Fatal(err)
	}
	a.ScanHistory = []domain.ScanRun{{RuleVersion: "rules-2", ExecutedAt: now, FindingSetDigest: "digest", AddedCount: 1}}
	a.RevisionTasks = []domain.RevisionTask{{ID: "task-1", ReviewRoundID: "review-1", RoundNumber: 1, FindingID: "finding-1", BeforeDisposition: domain.DispositionMask, BeforeRationale: "原理由", Completed: true, AfterDisposition: domain.DispositionGeneralize, AfterRationale: "新理由"}}
	a.Consent = &domain.ConsentGrant{ID: "consent-1", CaseID: a.Case.ID, ScopeDigest: "scope", ResearchAllowed: true, ConfirmedBy: "C-1", ConfirmedAt: now, FrozenScope: []domain.ConsentScopeItem{{SegmentID: "segment-1", Sequence: 1, TranscriptSummary: "摘要"}}, ConsentDigest: "consent-digest"}
	event := domain.AuditEvent{ID: "event-1", CaseID: a.Case.ID, Actor: "A", OccurredAt: now, BeforeVersion: 0, AfterVersion: 1, Action: "case.created", TargetID: a.Case.ID, Details: "创建"}
	if err = store.Create(ctx, a, event, "key-create"); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	loaded, err := store.Get(ctx, a.Case.ID)
	if err != nil || loaded.Case.ContributorCode != "C-1" || len(loaded.ScanHistory) != 1 || len(loaded.RevisionTasks) != 1 || loaded.Consent == nil || len(loaded.Consent.FrozenScope) != 1 {
		t.Fatalf("重启恢复失败: %#v, %v", loaded, err)
	}
	replayed, ok, err := store.FindIdempotent(ctx, "key-create")
	if err != nil || !ok || replayed.Case.ID != a.Case.ID {
		t.Fatalf("幂等结果恢复失败: %#v %v %v", replayed, ok, err)
	}
	events, err := store.Events(ctx, a.Case.ID)
	if err != nil || len(events) != 1 || !events[0].OccurredAt.Equal(now) {
		t.Fatalf("审计恢复失败: %#v %v", events, err)
	}
}

func TestOptimisticConflictDoesNotAppendAudit(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	a, _ := domain.NewAggregate("case-conflict", "C", "背景", []string{"粤语"}, "公开", now)
	created := domain.AuditEvent{ID: "e1", CaseID: a.Case.ID, Actor: "A", OccurredAt: now, AfterVersion: 1, Action: "case.created", TargetID: a.Case.ID}
	if err = store.Create(ctx, a, created, "create"); err != nil {
		t.Fatal(err)
	}
	a.Case.Version = 2
	update := domain.AuditEvent{ID: "e2", CaseID: a.Case.ID, Actor: "A", OccurredAt: now, BeforeVersion: 0, AfterVersion: 2, Action: "bad.update", TargetID: a.Case.ID}
	err = store.Update(ctx, a, 0, update, "bad")
	var conflict *domain.VersionConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("预期版本冲突，得到 %v", err)
	}
	events, _ := store.Events(ctx, a.Case.ID)
	if len(events) != 1 {
		t.Fatalf("冲突事务产生了部分审计写入: %#v", events)
	}
}
