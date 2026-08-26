package workflow

import (
	"context"
	"crypto/sha256"
	"dialectrelease/internal/audit"
	"dialectrelease/internal/domain"
	"encoding/hex"
	"regexp"
	"sort"
	"strconv"
	"time"
)

type scanRule struct {
	kind       domain.FindingType
	expression *regexp.Regexp
}

var localRules = []scanRule{
	{domain.FindingContact, regexp.MustCompile(`1[3-9][0-9]{9}|[0-9]{3,4}-[0-9]{7,8}`)},
	{domain.FindingPerson, regexp.MustCompile(`(?:姓名|叫作|叫做)[：:]?[\p{Han}]{2,4}`)},
	{domain.FindingLocation, regexp.MustCompile(`[\p{Han}]{2,12}(?:村|街|路|巷)[0-9一二三四五六七八九十甲乙丙丁-]*号?`)},
	{domain.FindingThirdParty, regexp.MustCompile(`(?:他说|她说|他们说|邻居说)`)},
}

func stableFindingID(segmentID string, kind domain.FindingType, start, end int, evidence string) string {
	sum := sha256.Sum256([]byte(segmentID + "|" + string(kind) + "|" + evidence + "|" + strconv.Itoa(start) + "|" + strconv.Itoa(end)))
	return "f-" + hex.EncodeToString(sum[:8])
}

func scan(a *domain.Aggregate) []domain.SensitivityFinding {
	result := []domain.SensitivityFinding{}
	for _, segment := range a.Segments {
		runes := []rune(segment.Transcript)
		for _, rule := range localRules {
			locations := rule.expression.FindAllStringIndex(segment.Transcript, -1)
			for _, loc := range locations {
				start := len([]rune(segment.Transcript[:loc[0]]))
				end := start + len([]rune(segment.Transcript[loc[0]:loc[1]]))
				if end > len(runes) {
					continue
				}
				evidence := string(runes[start:end])
				result = append(result, domain.SensitivityFinding{ID: stableFindingID(segment.ID, rule.kind, start, end, evidence), SegmentID: segment.ID, FindingType: rule.kind, Start: start, End: end, Evidence: evidence})
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SegmentID == result[j].SegmentID {
			return result[i].Start < result[j].Start
		}
		return result[i].SegmentID < result[j].SegmentID
	})
	return result
}

func (s *Service) Scan(ctx context.Context, caseID string, meta CommandMeta) (*domain.Aggregate, error) {
	return s.mutateDetailed(ctx, caseID, meta, "sensitivity.scanned", caseID, func(a *domain.Aggregate) string {
		run := a.LatestScan()
		return auditDetails(map[string]any{"ruleVersion": run.RuleVersion, "findingSetDigest": run.FindingSetDigest, "addedCount": run.AddedCount, "unchangedCount": run.UnchangedCount, "removedCount": run.RemovedCount})
	}, func(a *domain.Aggregate, now time.Time) error {
		findings := scan(a)
		_, err := a.ApplyScan(findings, "sensitivity-rules-1", findingSetDigest(findings), now)
		return err
	})
}

func findingSetDigest(findings []domain.SensitivityFinding) string {
	type item struct {
		SegmentID   string             `json:"segmentId"`
		FindingType domain.FindingType `json:"findingType"`
		Start       int                `json:"start"`
		End         int                `json:"end"`
		Evidence    string             `json:"evidence"`
	}
	values := make([]item, 0, len(findings))
	for _, finding := range findings {
		values = append(values, item{finding.SegmentID, finding.FindingType, finding.Start, finding.End, finding.Evidence})
	}
	return audit.MustDigest(values)
}

type ResolveFindingInput struct {
	CommandMeta
	Disposition domain.Disposition `json:"disposition"`
	Rationale   string             `json:"rationale"`
}

func (s *Service) ResolveFinding(ctx context.Context, caseID, findingID string, in ResolveFindingInput) (*domain.Aggregate, error) {
	return s.mutate(ctx, caseID, in.CommandMeta, "finding.resolved", findingID, "逐项处置敏感候选", func(a *domain.Aggregate, now time.Time) error {
		return a.ResolveFindingBy(findingID, in.Disposition, in.Rationale, in.Actor, now)
	})
}
