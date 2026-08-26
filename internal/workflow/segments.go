package workflow

import (
	"context"
	"dialectrelease/internal/domain"
	"time"
)

type SegmentDraftInput struct {
	Sequence    int    `json:"sequence"`
	SpeakerCode string `json:"speakerCode"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	Transcript  string `json:"transcript"`
	Category    string `json:"category"`
}

type AddSegmentsInput struct {
	CommandMeta
	Segments []SegmentDraftInput `json:"segments"`
}

func (s *Service) AddSegments(ctx context.Context, caseID string, input AddSegmentsInput) (*domain.Aggregate, error) {
	segments := make([]domain.CorpusSegment, len(input.Segments))
	ids := make([]string, len(input.Segments))
	for index, draft := range input.Segments {
		ids[index] = s.ids.New()
		segments[index] = domain.CorpusSegment{ID: ids[index], Sequence: draft.Sequence, SpeakerCode: draft.SpeakerCode, StartMillis: draft.StartMillis, EndMillis: draft.EndMillis, Transcript: draft.Transcript, Category: draft.Category}
	}
	return s.mutateDetailed(ctx, caseID, input.CommandMeta, "segments.batch_added", caseID, func(*domain.Aggregate) string {
		return auditDetails(map[string]any{"addedCount": len(ids), "segmentIds": ids})
	}, func(a *domain.Aggregate, now time.Time) error { return a.AddSegments(segments, now) })
}

type SegmentIDsInput struct {
	CommandMeta
	SegmentIDs []string `json:"segmentIds"`
}

func (s *Service) RevokeSegments(ctx context.Context, caseID string, input SegmentIDsInput) (*domain.Aggregate, error) {
	ids := append([]string(nil), input.SegmentIDs...)
	return s.mutateDetailed(ctx, caseID, input.CommandMeta, "segments.revoked", caseID, func(*domain.Aggregate) string {
		return auditDetails(map[string]any{"revokedCount": len(ids), "segmentIds": ids})
	}, func(a *domain.Aggregate, now time.Time) error { return a.RevokeSegments(ids, now) })
}

func (s *Service) ReorderSegments(ctx context.Context, caseID string, input SegmentIDsInput) (*domain.Aggregate, error) {
	var before []string
	return s.mutateDetailed(ctx, caseID, input.CommandMeta, "segments.reordered", caseID, func(*domain.Aggregate) string {
		return auditDetails(map[string]any{"before": before, "after": input.SegmentIDs})
	}, func(a *domain.Aggregate, now time.Time) error {
		before = make([]string, len(a.Segments))
		for index, segment := range a.Segments {
			before[index] = segment.ID
		}
		return a.ReorderSegments(input.SegmentIDs, now)
	})
}
