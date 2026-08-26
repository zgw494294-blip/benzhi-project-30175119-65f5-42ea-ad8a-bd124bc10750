package domain

import (
	"sort"
	"strings"
	"time"
)

type CaseMetadata struct {
	ContributorCode   string   `json:"contributorCode"`
	CollectionContext string   `json:"collectionContext"`
	LanguageTags      []string `json:"languageTags"`
	IntendedAudience  string   `json:"intendedAudience"`
}

func validateMetadata(metadata CaseMetadata) (CaseMetadata, error) {
	metadata.ContributorCode = strings.TrimSpace(metadata.ContributorCode)
	metadata.CollectionContext = strings.TrimSpace(metadata.CollectionContext)
	metadata.IntendedAudience = strings.TrimSpace(metadata.IntendedAudience)
	cleanTags := make([]string, 0, len(metadata.LanguageTags))
	seen := map[string]bool{}
	for _, tag := range metadata.LanguageTags {
		tag = strings.TrimSpace(tag)
		if tag != "" && !seen[tag] {
			cleanTags = append(cleanTags, tag)
			seen[tag] = true
		}
	}
	metadata.LanguageTags = cleanTags
	if metadata.ContributorCode == "" || metadata.CollectionContext == "" || metadata.IntendedAudience == "" || len(cleanTags) == 0 {
		return metadata, NewRuleError("invalid_case", "贡献者代号、采集背景、语种标签和预期公开范围均为必填项")
	}
	return metadata, nil
}

func (a *Aggregate) UpdateMetadata(metadata CaseMetadata, now time.Time) error {
	if err := requireStatus(a.Case.Status, StatusDraft); err != nil {
		return err
	}
	metadata, err := validateMetadata(metadata)
	if err != nil {
		return err
	}
	a.Case.ContributorCode = metadata.ContributorCode
	a.Case.CollectionContext = metadata.CollectionContext
	a.Case.LanguageTags = metadata.LanguageTags
	a.Case.IntendedAudience = metadata.IntendedAudience
	a.bump(now)
	return nil
}

func (a *Aggregate) UpdateSegment(segmentID string, update CorpusSegment, now time.Time) error {
	if err := requireStatus(a.Case.Status, StatusDraft); err != nil {
		return err
	}
	if strings.TrimSpace(update.SpeakerCode) == "" || strings.TrimSpace(update.Transcript) == "" || strings.TrimSpace(update.Category) == "" {
		return NewRuleError("invalid_segment", "说话者代号、转写和内容类别均为必填项")
	}
	if update.StartMillis < 0 || update.EndMillis <= update.StartMillis || update.Sequence < 1 {
		return NewRuleError("invalid_boundary", "顺序号必须为正数，且片段结束时间必须大于开始时间")
	}
	for _, existing := range a.Segments {
		if existing.ID != segmentID && existing.Sequence == update.Sequence {
			return NewRuleError("duplicate_sequence", "片段顺序号不得重复")
		}
	}
	for index := range a.Segments {
		if a.Segments[index].ID != segmentID {
			continue
		}
		update.ID = segmentID
		update.CaseID = a.Case.ID
		update.Revision = a.Segments[index].Revision + 1
		a.Segments[index] = update
		sort.Slice(a.Segments, func(i, j int) bool { return a.Segments[i].Sequence < a.Segments[j].Sequence })
		a.bump(now)
		return nil
	}
	return NewRuleError("segment_not_found", "未找到指定语料片段")
}
