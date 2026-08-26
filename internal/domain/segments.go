package domain

import (
	"sort"
	"strings"
	"time"
)

func (a *Aggregate) AddSegments(segments []CorpusSegment, now time.Time) error {
	if err := requireStatus(a.Case.Status, StatusDraft); err != nil {
		return err
	}
	if len(segments) == 0 {
		return NewRuleError("empty_segment_batch", "批量录入至少需要一条片段")
	}
	existingSequences := make(map[int]bool, len(a.Segments))
	for _, segment := range a.Segments {
		existingSequences[segment.Sequence] = true
	}
	counts := make(map[int]int, len(segments))
	for _, segment := range segments {
		counts[segment.Sequence]++
	}
	issues := make([]FieldIssue, 0)
	clean := make([]CorpusSegment, len(segments))
	for index, segment := range segments {
		row := index + 1
		segment.SpeakerCode = strings.TrimSpace(segment.SpeakerCode)
		segment.Transcript = strings.TrimSpace(segment.Transcript)
		segment.Category = strings.TrimSpace(segment.Category)
		if segment.Sequence < 1 {
			issues = append(issues, FieldIssue{Row: row, Field: "sequence", Reason: "顺序号必须为正数"})
		}
		if counts[segment.Sequence] > 1 {
			issues = append(issues, FieldIssue{Row: row, Field: "sequence", Reason: "顺序号与本批次其他行重复"})
		}
		if existingSequences[segment.Sequence] {
			issues = append(issues, FieldIssue{Row: row, Field: "sequence", Reason: "顺序号与案件现有片段重复"})
		}
		if segment.SpeakerCode == "" {
			issues = append(issues, FieldIssue{Row: row, Field: "speakerCode", Reason: "说话者代号不能为空"})
		}
		if segment.Transcript == "" {
			issues = append(issues, FieldIssue{Row: row, Field: "transcript", Reason: "文字转写不能为空"})
		}
		if segment.Category == "" {
			issues = append(issues, FieldIssue{Row: row, Field: "category", Reason: "内容类别不能为空"})
		}
		if segment.StartMillis < 0 {
			issues = append(issues, FieldIssue{Row: row, Field: "startMillis", Reason: "开始时间不能为负数"})
		}
		if segment.EndMillis <= segment.StartMillis {
			issues = append(issues, FieldIssue{Row: row, Field: "endMillis", Reason: "结束时间必须大于开始时间"})
		}
		segment.CaseID = a.Case.ID
		segment.Revision = 1
		clean[index] = segment
	}
	if len(issues) > 0 {
		return &ValidationError{Code: "invalid_segment_batch", Message: "批量片段校验失败，请一次修正全部标注字段", Issues: issues}
	}
	a.Segments = append(a.Segments, clean...)
	sort.Slice(a.Segments, func(i, j int) bool { return a.Segments[i].Sequence < a.Segments[j].Sequence })
	a.bump(now)
	return nil
}

func (a *Aggregate) RevokeSegments(ids []string, now time.Time) error {
	if err := requireStatus(a.Case.Status, StatusDraft); err != nil {
		return err
	}
	if len(ids) == 0 {
		return NewRuleError("empty_segment_selection", "至少选择一个需要撤销的片段")
	}
	known := make(map[string]bool, len(a.Segments))
	for _, segment := range a.Segments {
		known[segment.ID] = true
	}
	selected := make(map[string]bool, len(ids))
	issues := make([]FieldIssue, 0)
	for index, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			issues = append(issues, FieldIssue{Row: index + 1, Field: "segmentIds", Reason: "片段标识不能为空"})
		} else if selected[id] {
			issues = append(issues, FieldIssue{Row: index + 1, ItemID: id, Field: "segmentIds", Reason: "片段标识重复"})
		} else if !known[id] {
			issues = append(issues, FieldIssue{Row: index + 1, ItemID: id, Field: "segmentIds", Reason: "片段不属于当前案件"})
		}
		selected[id] = true
	}
	if len(issues) > 0 {
		return &ValidationError{Code: "invalid_segment_selection", Message: "撤销片段选择无效", Issues: issues}
	}
	remaining := make([]CorpusSegment, 0, len(a.Segments)-len(selected))
	for _, segment := range a.Segments {
		if !selected[segment.ID] {
			remaining = append(remaining, segment)
		}
	}
	sort.Slice(remaining, func(i, j int) bool { return remaining[i].Sequence < remaining[j].Sequence })
	for index := range remaining {
		newSequence := index + 1
		if remaining[index].Sequence != newSequence {
			remaining[index].Sequence = newSequence
			remaining[index].Revision++
		}
	}
	a.Segments = remaining
	a.bump(now)
	return nil
}

func (a *Aggregate) ReorderSegments(ids []string, now time.Time) error {
	if err := requireStatus(a.Case.Status, StatusDraft); err != nil {
		return err
	}
	if len(a.Segments) == 0 {
		return NewRuleError("empty_segment_order", "没有可重排的片段")
	}
	byID := make(map[string]CorpusSegment, len(a.Segments))
	for _, segment := range a.Segments {
		byID[segment.ID] = segment
	}
	issues := make([]FieldIssue, 0)
	seen := make(map[string]bool, len(ids))
	if len(ids) != len(a.Segments) {
		issues = append(issues, FieldIssue{Field: "segmentIds", Reason: "排序列表必须完整包含当前案件的全部片段"})
	}
	for index, id := range ids {
		if seen[id] {
			issues = append(issues, FieldIssue{Row: index + 1, ItemID: id, Field: "segmentIds", Reason: "片段标识重复"})
		} else if _, ok := byID[id]; !ok {
			issues = append(issues, FieldIssue{Row: index + 1, ItemID: id, Field: "segmentIds", Reason: "片段不属于当前案件"})
		}
		seen[id] = true
	}
	for id := range byID {
		if !seen[id] {
			issues = append(issues, FieldIssue{ItemID: id, Field: "segmentIds", Reason: "排序列表遗漏当前片段"})
		}
	}
	if len(issues) > 0 {
		return &ValidationError{Code: "invalid_segment_order", Message: "片段顺序校验失败", Issues: issues}
	}
	reordered := make([]CorpusSegment, len(ids))
	for index, id := range ids {
		segment := byID[id]
		if segment.Sequence != index+1 {
			segment.Sequence = index + 1
			segment.Revision++
		}
		reordered[index] = segment
	}
	a.Segments = reordered
	a.bump(now)
	return nil
}
