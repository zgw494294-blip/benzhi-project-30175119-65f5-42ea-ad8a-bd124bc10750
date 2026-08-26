package domain

import (
	"errors"
	"testing"
	"time"
)

var testNow = time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)

func draftAggregate(t *testing.T) *Aggregate {
	t.Helper()
	a, err := NewAggregate("case-1", "C-01", "社区访谈", []string{"吴语"}, "研究与公开展示", testNow)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.AddSegment(CorpusSegment{ID: "segment-1", Sequence: 1, SpeakerCode: "S-01", StartMillis: 0, EndMillis: 3000, Transcript: "张三住在河西村12号", Category: "叙事"}, testNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestLifecycleFreezesManifestAndPublishes(t *testing.T) {
	a := draftAggregate(t)
	if err := a.RequestConsent(testNow.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := a.ConfirmConsent(ConsentGrant{ID: "consent-1", ResearchAllowed: true, PublicDisplayAllowed: true, ConfirmedBy: "C-01"}, "scope", testNow.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	findings := []SensitivityFinding{{ID: "finding-1", SegmentID: "segment-1", FindingType: FindingPerson, Start: 0, End: 2, Evidence: "张三"}}
	if err := a.SetFindings(findings, testNow.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := a.ResolveFinding("finding-1", DispositionMask, "保护真实姓名", testNow.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	manifest, err := a.BuildManifest()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := manifest[0].PublishedText, "[已遮蔽]住在河西村12号"; got != want {
		t.Fatalf("发布预览 = %q, want %q", got, want)
	}
	if err = a.SubmitReview("review-1", "submission", testNow.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err = a.DecideReview(ReviewApproved, "符合要求", nil, "ETHICS", manifest, testNow.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	credential := ReleaseCredential{ID: "credential-1", CaseID: a.Case.ID, CaseVersion: a.Case.Version + 1}
	if err = a.Publish(credential, testNow.Add(8*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if a.Case.Status != StatusPublished || len(a.FrozenManifest) != 1 {
		t.Fatalf("发布状态或冻结清单不正确: %#v", a)
	}
	if err = a.ResolveFinding("finding-1", DispositionKeep, "修改", testNow); err == nil {
		t.Fatal("发布后不应允许修改处置")
	}
}

func TestReturnedReviewRestrictsRevisionScope(t *testing.T) {
	a := draftAggregate(t)
	_ = a.RequestConsent(testNow)
	_ = a.ConfirmConsent(ConsentGrant{ID: "c", PublicDisplayAllowed: true, ConfirmedBy: "C"}, "scope", testNow)
	_ = a.SetFindings([]SensitivityFinding{{ID: "f1", SegmentID: "segment-1", Start: 0, End: 1}, {ID: "f2", SegmentID: "segment-1", Start: 1, End: 2}}, testNow)
	_ = a.ResolveFinding("f1", DispositionMask, "理由一", testNow)
	_ = a.ResolveFinding("f2", DispositionKeep, "理由二", testNow)
	manifest, _ := a.BuildManifest()
	_ = a.SubmitReview("r1", "digest", testNow)
	if err := a.DecideReview(ReviewReturned, "需要重新泛化", []string{"f1"}, "R", manifest, testNow); err != nil {
		t.Fatal(err)
	}
	if a.Findings[0].Disposition != DispositionPending {
		t.Fatal("被退回项必须恢复为未决状态")
	}
	var rule *RuleError
	err := a.ResolveFinding("f2", DispositionMask, "越界修改", testNow)
	if !errors.As(err, &rule) || rule.Code != "revision_scope" {
		t.Fatalf("预期 revision_scope，得到 %v", err)
	}
	if err = a.ResolveFinding("f1", DispositionGeneralize, "按意见泛化", testNow); err != nil {
		t.Fatal(err)
	}
}

func TestDraftEditingAndAssessment(t *testing.T) {
	a := draftAggregate(t)
	before := a.Case.Version
	if err := a.UpdateMetadata(CaseMetadata{ContributorCode: " C-02 ", CollectionContext: "补充背景", LanguageTags: []string{"吴语", "吴语", "口述"}, IntendedAudience: "研究"}, testNow); err != nil {
		t.Fatal(err)
	}
	if a.Case.Version != before+1 || len(a.Case.LanguageTags) != 2 {
		t.Fatal("案件维护未清理标签或递增版本")
	}
	segment := a.Segments[0]
	segment.Transcript = "修订后的转写"
	if err := a.UpdateSegment(segment.ID, segment, testNow); err != nil {
		t.Fatal(err)
	}
	if a.Segments[0].Revision != 2 {
		t.Fatal("片段 revision 未递增")
	}
	assessment := a.AssessPublication()
	if assessment.Ready || len(assessment.Blockers) == 0 {
		t.Fatal("未同意案件不应达到发布就绪")
	}
}

func TestBatchSegmentsAreAtomicAndReorderKeepsIDs(t *testing.T) {
	a := draftAggregate(t)
	beforeVersion := a.Case.Version
	err := a.AddSegments([]CorpusSegment{
		{ID: "segment-2", Sequence: 2, SpeakerCode: "", StartMillis: 3000, EndMillis: 4000, Transcript: "第二条", Category: "叙事"},
		{ID: "segment-3", Sequence: 1, SpeakerCode: "S-03", StartMillis: 4000, EndMillis: 5000, Transcript: "第三条", Category: "叙事"},
	}, testNow)
	var validation *ValidationError
	if !errors.As(err, &validation) || len(validation.Issues) < 2 {
		t.Fatalf("应汇总缺少说话者和既有顺序冲突，得到 %#v", err)
	}
	if a.Case.Version != beforeVersion || len(a.Segments) != 1 {
		t.Fatal("失败批次不应改变片段或版本")
	}
	err = a.AddSegments([]CorpusSegment{
		{ID: "segment-2", Sequence: 2, SpeakerCode: "S-02", StartMillis: 3000, EndMillis: 4000, Transcript: "第二条", Category: "叙事"},
		{ID: "segment-3", Sequence: 3, SpeakerCode: "S-03", StartMillis: 4000, EndMillis: 5000, Transcript: "第三条", Category: "叙事"},
	}, testNow)
	if err != nil || a.Case.Version != beforeVersion+1 || len(a.Segments) != 3 {
		t.Fatalf("合法批次应只递增一次版本: %#v, %v", a.Case, err)
	}
	if err = a.RevokeSegments([]string{"segment-2"}, testNow); err != nil {
		t.Fatal(err)
	}
	if len(a.Segments) != 2 || a.Segments[1].ID != "segment-3" || a.Segments[1].Sequence != 2 {
		t.Fatalf("撤销后应连续编号并保留标识: %#v", a.Segments)
	}
	if err = a.ReorderSegments([]string{"segment-3", "segment-1"}, testNow); err != nil {
		t.Fatal(err)
	}
	if a.Segments[0].ID != "segment-3" || a.Segments[0].Sequence != 1 || a.Segments[1].ID != "segment-1" {
		t.Fatalf("重排结果错误: %#v", a.Segments)
	}
}

func TestConsentScopeConflictAndScanInheritance(t *testing.T) {
	a := draftAggregate(t)
	if err := a.RequestConsent(testNow); err != nil {
		t.Fatal(err)
	}
	before := a.Case.Version
	err := a.ConfirmConsentScope(ConsentGrant{ID: "c", ResearchAllowed: true, ConfirmedBy: "C"}, "changed", "current", testNow)
	var conflict *ScopeConflict
	if !errors.As(err, &conflict) || conflict.Current.ScopeDigest != "current" || a.Case.Version != before || a.Consent != nil {
		t.Fatalf("范围冲突应返回当前清单且零写入: %#v, %v", conflict, err)
	}
	if err = a.ConfirmConsentScope(ConsentGrant{ID: "c", ResearchAllowed: true, ConfirmedBy: "C"}, "current", "current", testNow); err != nil {
		t.Fatal(err)
	}
	first := []SensitivityFinding{
		{ID: "f-person", SegmentID: "segment-1", FindingType: FindingPerson, Start: 0, End: 2, Evidence: "张三"},
		{ID: "f-place", SegmentID: "segment-1", FindingType: FindingLocation, Start: 4, End: 11, Evidence: "河西村12号"},
	}
	if _, err = a.ApplyScan(first, "rules-1", "set-1", testNow); err != nil {
		t.Fatal(err)
	}
	if err = a.ResolveFindingBy("f-person", DispositionMask, "保护姓名", "A", testNow); err != nil {
		t.Fatal(err)
	}
	resolvedAt := a.Findings[0].ResolvedAt
	second := []SensitivityFinding{
		{ID: "f-person", SegmentID: "segment-1", FindingType: FindingPerson, Start: 0, End: 2, Evidence: "张三"},
		{ID: "f-contact", SegmentID: "segment-1", FindingType: FindingContact, Start: 12, End: 23, Evidence: "13800138000"},
	}
	run, err := a.ApplyScan(second, "rules-2", "set-2", testNow)
	if err != nil {
		t.Fatal(err)
	}
	if run.AddedCount != 1 || run.UnchangedCount != 1 || run.RemovedCount != 1 {
		t.Fatalf("扫描差异错误: %#v", run)
	}
	if a.Findings[0].Disposition != DispositionMask || a.Findings[0].Rationale != "保护姓名" || a.Findings[0].ResolvedAt != resolvedAt {
		t.Fatalf("完全匹配候选未继承处置: %#v", a.Findings[0])
	}
	if a.Findings[1].Disposition != DispositionPending || run.RemovedFindings[0].ID != "f-place" {
		t.Fatalf("新增或消失候选处理错误: %#v %#v", a.Findings, run)
	}
}

func TestBatchFindingValidationAndRevisionTasks(t *testing.T) {
	a := draftAggregate(t)
	_ = a.RequestConsent(testNow)
	_ = a.ConfirmConsent(ConsentGrant{ID: "c", PublicDisplayAllowed: true, ConfirmedBy: "C"}, "scope", testNow)
	_ = a.SetFindings([]SensitivityFinding{{ID: "f1", SegmentID: "segment-1", Start: 0, End: 1}, {ID: "f2", SegmentID: "segment-1", Start: 1, End: 2}}, testNow)
	before := a.Case.Version
	err := a.ResolveFindings([]FindingDecisionInput{{FindingID: "f1", Disposition: DispositionMask}, {FindingID: "missing", Disposition: DispositionKeep, Rationale: "理由"}}, "A", testNow)
	var validation *ValidationError
	if !errors.As(err, &validation) || len(validation.Issues) < 2 || a.Case.Version != before || a.Findings[0].Disposition != DispositionPending {
		t.Fatalf("无效批量处置必须零写入并汇总错误: %#v, %v", a, err)
	}
	if err = a.ResolveFindings([]FindingDecisionInput{{FindingID: "f1", Disposition: DispositionMask, Rationale: "理由一"}, {FindingID: "f2", Disposition: DispositionKeep, Rationale: "理由二"}}, "A", testNow); err != nil {
		t.Fatal(err)
	}
	manifest, _ := a.BuildManifest()
	_ = a.SubmitReview("r1", "digest", testNow)
	if err = a.DecideReview(ReviewReturned, "请形成实际整改", []string{"f1"}, "R", manifest, testNow); err != nil {
		t.Fatal(err)
	}
	if len(a.RevisionTasks) != 1 || a.RevisionTasks[0].BeforeDisposition != DispositionMask {
		t.Fatalf("退回任务未保存原决定: %#v", a.RevisionTasks)
	}
	if err = a.ResolveFindingBy("f1", DispositionMask, "理由一", "A2", testNow); err != nil {
		t.Fatal(err)
	}
	if a.RevisionTasks[0].Completed {
		t.Fatal("与退回前完全相同的决定不应完成任务")
	}
	err = a.ValidateReviewSubmission()
	if !errors.As(err, &validation) || validation.Issues[0].ItemID != "f1" {
		t.Fatalf("重提应准确列出未完成候选: %#v", err)
	}
	if err = a.ResolveFindingBy("f1", DispositionGeneralize, "按意见泛化", "A2", testNow); err != nil {
		t.Fatal(err)
	}
	if !a.RevisionTasks[0].Completed || a.RevisionTasks[0].CompletedBy != "A2" {
		t.Fatalf("有效整改未完成任务: %#v", a.RevisionTasks[0])
	}
}
