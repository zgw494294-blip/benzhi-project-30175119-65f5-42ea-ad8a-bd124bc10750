package domain

import (
	"sort"
	"strconv"
	"time"
)

func findingMatchKey(f SensitivityFinding) string {
	return f.SegmentID + "\x00" + string(f.FindingType) + "\x00" + strconv.Itoa(f.Start) + "\x00" + strconv.Itoa(f.End) + "\x00" + f.Evidence
}

func (a *Aggregate) ApplyScan(findings []SensitivityFinding, ruleVersion, setDigest string, now time.Time) (ScanRun, error) {
	if err := requireStatus(a.Case.Status, StatusConsented); err != nil {
		return ScanRun{}, err
	}
	old := make(map[string]SensitivityFinding, len(a.Findings))
	for _, finding := range a.Findings {
		old[findingMatchKey(finding)] = finding
	}
	current := make([]SensitivityFinding, len(findings))
	run := ScanRun{RuleVersion: ruleVersion, ExecutedAt: now.UTC(), FindingSetDigest: setDigest, AddedFindingIDs: []string{}, UnchangedFindingIDs: []string{}, RemovedFindings: []SensitivityFinding{}}
	matched := make(map[string]bool, len(findings))
	for index, finding := range findings {
		key := findingMatchKey(finding)
		finding.RuleVersion = ruleVersion
		if previous, ok := old[key]; ok {
			finding.Disposition = previous.Disposition
			finding.Rationale = previous.Rationale
			finding.ResolvedAt = previous.ResolvedAt
			run.UnchangedFindingIDs = append(run.UnchangedFindingIDs, finding.ID)
			matched[key] = true
		} else {
			finding.Disposition = DispositionPending
			run.AddedFindingIDs = append(run.AddedFindingIDs, finding.ID)
		}
		current[index] = finding
	}
	for key, previous := range old {
		if !matched[key] {
			run.RemovedFindings = append(run.RemovedFindings, previous)
		}
	}
	sort.Strings(run.AddedFindingIDs)
	sort.Strings(run.UnchangedFindingIDs)
	sort.Slice(run.RemovedFindings, func(i, j int) bool { return run.RemovedFindings[i].ID < run.RemovedFindings[j].ID })
	run.AddedCount = len(run.AddedFindingIDs)
	run.UnchangedCount = len(run.UnchangedFindingIDs)
	run.RemovedCount = len(run.RemovedFindings)
	a.Findings = current
	a.ScanCompleted = true
	a.ScanHistory = append(a.ScanHistory, run)
	a.bump(now)
	return run, nil
}

func (a *Aggregate) LatestScan() *ScanRun {
	if len(a.ScanHistory) == 0 {
		return nil
	}
	return &a.ScanHistory[len(a.ScanHistory)-1]
}
