package workflow

import (
	"context"
	"dialectrelease/internal/domain"
	"time"
)

type UpdateCaseInput struct {
	CommandMeta
	ContributorCode   string   `json:"contributorCode"`
	CollectionContext string   `json:"collectionContext"`
	LanguageTags      []string `json:"languageTags"`
	IntendedAudience  string   `json:"intendedAudience"`
}

func (s *Service) UpdateCase(ctx context.Context, caseID string, input UpdateCaseInput) (*domain.Aggregate, error) {
	metadata := domain.CaseMetadata{ContributorCode: input.ContributorCode, CollectionContext: input.CollectionContext, LanguageTags: input.LanguageTags, IntendedAudience: input.IntendedAudience}
	return s.mutate(ctx, caseID, input.CommandMeta, "case.updated", caseID, "维护贡献者、采集背景、语种标签和预期公开范围", func(a *domain.Aggregate, now time.Time) error {
		return a.UpdateMetadata(metadata, now)
	})
}

type UpdateSegmentInput struct {
	CommandMeta
	Sequence    int    `json:"sequence"`
	SpeakerCode string `json:"speakerCode"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	Transcript  string `json:"transcript"`
	Category    string `json:"category"`
}

func (s *Service) UpdateSegment(ctx context.Context, caseID, segmentID string, input UpdateSegmentInput) (*domain.Aggregate, error) {
	segment := domain.CorpusSegment{Sequence: input.Sequence, SpeakerCode: input.SpeakerCode, StartMillis: input.StartMillis, EndMillis: input.EndMillis, Transcript: input.Transcript, Category: input.Category}
	return s.mutate(ctx, caseID, input.CommandMeta, "segment.updated", segmentID, "维护语料片段边界、转写和元数据", func(a *domain.Aggregate, now time.Time) error {
		return a.UpdateSegment(segmentID, segment, now)
	})
}
