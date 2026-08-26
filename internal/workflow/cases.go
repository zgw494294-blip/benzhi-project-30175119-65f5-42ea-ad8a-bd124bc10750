package workflow

import (
	"context"
	"dialectrelease/internal/domain"
	"dialectrelease/internal/repository"
	"strings"
	"time"
)

type CreateCaseInput struct {
	ContributorCode   string   `json:"contributorCode"`
	CollectionContext string   `json:"collectionContext"`
	LanguageTags      []string `json:"languageTags"`
	IntendedAudience  string   `json:"intendedAudience"`
	IdempotencyKey    string   `json:"idempotencyKey"`
	Actor             string   `json:"actor"`
}

func (s *Service) CreateCase(ctx context.Context, in CreateCaseInput) (*domain.Aggregate, error) {
	if strings.TrimSpace(in.IdempotencyKey) == "" || strings.TrimSpace(in.Actor) == "" {
		return nil, domain.NewRuleError("invalid_command", "actor 和 idempotencyKey 均为必填项")
	}
	if replay, ok, err := s.store.FindIdempotent(ctx, in.IdempotencyKey); err != nil || ok {
		return replay, err
	}
	now := s.clock().UTC()
	a, err := domain.NewAggregate(s.ids.New(), in.ContributorCode, in.CollectionContext, in.LanguageTags, in.IntendedAudience, now)
	if err != nil {
		return nil, err
	}
	e := s.recorder.Event(a.Case.ID, in.Actor, "case.created", a.Case.ID, "创建方言语料捐赠案", 0, a.Case.Version)
	if err = s.store.Create(ctx, a, e, in.IdempotencyKey); err == repository.ErrDuplicate {
		if replay, ok, e := s.store.FindIdempotent(ctx, in.IdempotencyKey); ok || e != nil {
			return replay, e
		}
	}
	return a, err
}

type AddSegmentInput struct {
	CommandMeta
	Sequence    int    `json:"sequence"`
	SpeakerCode string `json:"speakerCode"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	Transcript  string `json:"transcript"`
	Category    string `json:"category"`
}

func (s *Service) AddSegment(ctx context.Context, caseID string, in AddSegmentInput) (*domain.Aggregate, error) {
	id := s.ids.New()
	return s.mutate(ctx, caseID, in.CommandMeta, "segment.added", id, "录入有序语料片段", func(a *domain.Aggregate, nowTime time.Time) error {
		return a.AddSegment(domain.CorpusSegment{ID: id, Sequence: in.Sequence, SpeakerCode: in.SpeakerCode, StartMillis: in.StartMillis, EndMillis: in.EndMillis, Transcript: in.Transcript, Category: in.Category}, nowTime)
	})
}
