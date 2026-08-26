package workflow

import (
	"context"
	"dialectrelease/internal/audit"
	"dialectrelease/internal/domain"
	"dialectrelease/internal/repository"
	"encoding/json"
	"strings"
	"time"
)

type Service struct {
	store    repository.Store
	ids      IDSource
	clock    func() time.Time
	recorder *audit.Recorder
}

func New(store repository.Store, ids IDSource, clock func() time.Time) *Service {
	return &Service{store: store, ids: ids, clock: clock, recorder: audit.NewRecorder(ids, clock)}
}

type CommandMeta struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Actor           string `json:"actor"`
}

func (s *Service) validateMeta(meta CommandMeta) error {
	if meta.ExpectedVersion < 1 {
		return domain.NewRuleError("missing_version", "expectedVersion 必须为正整数")
	}
	if strings.TrimSpace(meta.IdempotencyKey) == "" {
		return domain.NewRuleError("missing_idempotency_key", "idempotencyKey 不能为空")
	}
	if strings.TrimSpace(meta.Actor) == "" {
		return domain.NewRuleError("missing_actor", "操作者代号不能为空")
	}
	return nil
}

func (s *Service) replay(ctx context.Context, key, caseID string) (*domain.Aggregate, bool, error) {
	a, ok, err := s.store.FindIdempotent(ctx, key)
	if err != nil || !ok {
		return a, ok, err
	}
	if a.Case.ID != caseID {
		return nil, false, domain.NewRuleError("idempotency_mismatch", "幂等键已被另一个案件使用")
	}
	return a, true, nil
}

func (s *Service) mutate(ctx context.Context, caseID string, meta CommandMeta, action, target, details string, fn func(*domain.Aggregate, time.Time) error) (*domain.Aggregate, error) {
	return s.mutateDetailed(ctx, caseID, meta, action, target, func(*domain.Aggregate) string { return details }, fn)
}

func (s *Service) mutateDetailed(ctx context.Context, caseID string, meta CommandMeta, action, target string, details func(*domain.Aggregate) string, fn func(*domain.Aggregate, time.Time) error) (*domain.Aggregate, error) {
	if err := s.validateMeta(meta); err != nil {
		return nil, err
	}
	if a, ok, err := s.replay(ctx, meta.IdempotencyKey, caseID); err != nil || ok {
		return a, err
	}
	a, err := s.store.Get(ctx, caseID)
	if err != nil {
		return nil, err
	}
	if a.Case.Version != meta.ExpectedVersion {
		return nil, &domain.VersionConflict{Expected: meta.ExpectedVersion, Current: a.Case.Version, Status: a.Case.Status}
	}
	before := a.Case.Version
	now := s.clock().UTC()
	if err = fn(a, now); err != nil {
		return nil, err
	}
	event := s.recorder.Event(caseID, meta.Actor, action, target, details(a), before, a.Case.Version)
	if err = s.store.Update(ctx, a, before, event, meta.IdempotencyKey); err == repository.ErrDuplicate {
		if replay, ok, e := s.replay(ctx, meta.IdempotencyKey, caseID); ok || e != nil {
			return replay, e
		}
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func auditDetails(value any) string {
	b, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Aggregate, error) {
	return s.store.Get(ctx, id)
}
func (s *Service) Timeline(ctx context.Context, id string) ([]domain.AuditEvent, error) {
	return s.store.Events(ctx, id)
}
