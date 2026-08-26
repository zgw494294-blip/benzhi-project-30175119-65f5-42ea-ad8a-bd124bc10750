package domain

import (
	"sort"
	"strings"
)

type ManifestEntry struct {
	SegmentID     string `json:"segmentId"`
	Sequence      int    `json:"sequence"`
	SpeakerCode   string `json:"speakerCode"`
	Category      string `json:"category"`
	PublishedText string `json:"publishedText"`
	StartMillis   int64  `json:"startMillis"`
	EndMillis     int64  `json:"endMillis"`
}

func (a *Aggregate) BuildManifest() ([]ManifestEntry, error) {
	if a.Consent == nil || !a.Consent.PublicDisplayAllowed {
		return nil, NewRuleError("consent_coverage", "公开展示用途未获允许")
	}
	excluded := map[string]bool{}
	bySegment := map[string][]SensitivityFinding{}
	for _, f := range a.Findings {
		if f.Disposition == DispositionPending {
			return nil, NewRuleError("unresolved_findings", "存在未处置的敏感候选")
		}
		if f.Disposition == DispositionExclude {
			excluded[f.SegmentID] = true
		}
		bySegment[f.SegmentID] = append(bySegment[f.SegmentID], f)
	}
	entries := make([]ManifestEntry, 0, len(a.Segments))
	for _, s := range a.Segments {
		if excluded[s.ID] {
			continue
		}
		text := s.Transcript
		fs := bySegment[s.ID]
		sort.Slice(fs, func(i, j int) bool { return fs[i].Start > fs[j].Start })
		for _, f := range fs {
			if f.Start < 0 || f.End > len([]rune(text)) || f.Start >= f.End {
				continue
			}
			r := []rune(text)
			replacement := ""
			switch f.Disposition {
			case DispositionMask:
				replacement = "[已遮蔽]"
			case DispositionGeneralize:
				replacement = "[已泛化]"
			case DispositionKeep:
				continue
			default:
				continue
			}
			text = string(r[:f.Start]) + replacement + string(r[f.End:])
		}
		entries = append(entries, ManifestEntry{SegmentID: s.ID, Sequence: s.Sequence, SpeakerCode: s.SpeakerCode, Category: s.Category, PublishedText: strings.TrimSpace(text), StartMillis: s.StartMillis, EndMillis: s.EndMillis})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Sequence == entries[j].Sequence {
			return entries[i].SegmentID < entries[j].SegmentID
		}
		return entries[i].Sequence < entries[j].Sequence
	})
	if len(entries) == 0 {
		return nil, NewRuleError("empty_manifest", "处置后没有可发布片段")
	}
	return entries, nil
}
