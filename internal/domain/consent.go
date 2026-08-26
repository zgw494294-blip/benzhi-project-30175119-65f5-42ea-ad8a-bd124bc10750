package domain

import "strings"

func transcriptSummary(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= 80 {
		return string(runes)
	}
	return string(runes[:80]) + "…"
}

func (a *Aggregate) ConsentScope() []ConsentScopeItem {
	result := make([]ConsentScopeItem, 0, len(a.Segments))
	for _, segment := range a.Segments {
		result = append(result, ConsentScopeItem{
			SegmentID: segment.ID, Sequence: segment.Sequence, SpeakerCode: segment.SpeakerCode,
			StartMillis: segment.StartMillis, EndMillis: segment.EndMillis, Category: segment.Category,
			TranscriptSummary: transcriptSummary(segment.Transcript),
		})
	}
	return result
}

func ConsentImpacts(research, teaching, public bool) []ConsentPurposeImpact {
	allowedText := func(allowed bool) string {
		if allowed {
			return "允许纳入该用途的后续处理"
		}
		return "拒绝该用途，后续流程不得用于此目的"
	}
	result := []ConsentPurposeImpact{
		{Code: "research", Label: "研究用途", Allowed: research, Conclusion: allowedText(research)},
		{Code: "teaching", Label: "教学用途", Allowed: teaching, Conclusion: allowedText(teaching)},
		{Code: "public_display", Label: "公开展示", Allowed: public, Conclusion: allowedText(public), Blocking: !public},
	}
	if !public {
		result[2].Conclusion = "拒绝公开展示，将阻断公开清单、伦理批准后的发布凭据签发"
	}
	return result
}
