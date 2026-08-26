package workflow

import (
	"context"
	"dialectrelease/internal/audit"
	"dialectrelease/internal/domain"
	"dialectrelease/internal/repository"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type sequenceIDs struct{ next int }

func (s *sequenceIDs) New() string { s.next++; return fmt.Sprintf("id-%03d", s.next) }

func testService(t *testing.T) (*Service, func()) {
	t.Helper()
	store, err := repository.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	ids := &sequenceIDs{}
	clockValue := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	return New(store, ids, func() time.Time { clockValue = clockValue.Add(time.Second); return clockValue }), func() { store.Close() }
}

func command(version int64, key string) CommandMeta {
	return CommandMeta{ExpectedVersion: version, IdempotencyKey: key, Actor: "TEST"}
}

func TestWorkflowReturnRevisionApprovePublish(t *testing.T) {
	ctx := context.Background()
	svc, closeStore := testService(t)
	defer closeStore()
	a, err := svc.CreateCase(ctx, CreateCaseInput{ContributorCode: "C", CollectionContext: "社区访谈", LanguageTags: []string{"闽语"}, IntendedAudience: "公开", IdempotencyKey: "k1", Actor: "A"})
	if err != nil {
		t.Fatal(err)
	}
	a, err = svc.AddSegment(ctx, a.Case.ID, AddSegmentInput{CommandMeta: command(a.Case.Version, "k2"), Sequence: 1, SpeakerCode: "S", StartMillis: 0, EndMillis: 5000, Transcript: "姓名：张三住在河西村12号，电话13800138000，他说这是真的", Category: "叙事"})
	if err != nil {
		t.Fatal(err)
	}
	caseID := a.Case.ID
	a, err = svc.RequestConsent(ctx, caseID, command(a.Case.Version, "k3"))
	if err != nil {
		t.Fatal(err)
	}
	views, err := svc.Views(ctx, caseID)
	if err != nil {
		t.Fatal(err)
	}
	a, err = svc.ConfirmConsent(ctx, caseID, ConfirmConsentInput{CommandMeta: command(a.Case.Version, "k4"), ResearchAllowed: true, PublicDisplayAllowed: true, ConfirmedBy: "C", ScopeDigest: views.Checklist.ScopeDigest})
	if err != nil {
		t.Fatal(err)
	}
	a, err = svc.Scan(ctx, caseID, command(a.Case.Version, "k5"))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Findings) < 4 {
		t.Fatalf("扫描规则覆盖不足: %#v", a.Findings)
	}
	for index, finding := range a.Findings {
		a, err = svc.ResolveFinding(ctx, caseID, finding.ID, ResolveFindingInput{CommandMeta: command(a.Case.Version, fmt.Sprintf("resolve-%d", index)), Disposition: domain.DispositionMask, Rationale: "保护敏感信息"})
		if err != nil {
			t.Fatal(err)
		}
	}
	a, err = svc.SubmitReview(ctx, caseID, command(a.Case.Version, "submit-1"))
	if err != nil {
		t.Fatal(err)
	}
	target := a.Findings[0].ID
	a, err = svc.DecideReview(ctx, caseID, ReviewInput{CommandMeta: command(a.Case.Version, "return"), Decision: domain.ReviewReturned, Comments: "需要泛化", TargetFindingIDs: []string{target}, ReviewerCode: "R"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Case.Status != domain.StatusRevision {
		t.Fatalf("状态 = %s", a.Case.Status)
	}
	if len(a.Findings) > 1 {
		_, err = svc.ResolveFinding(ctx, caseID, a.Findings[1].ID, ResolveFindingInput{CommandMeta: command(a.Case.Version, "outside"), Disposition: domain.DispositionKeep, Rationale: "越界"})
		if err == nil {
			t.Fatal("整改范围外修改应失败")
		}
	}
	a, err = svc.ResolveFinding(ctx, caseID, target, ResolveFindingInput{CommandMeta: command(a.Case.Version, "same-as-before"), Disposition: domain.DispositionMask, Rationale: "保护敏感信息"})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.RevisionTasks) != 1 || a.RevisionTasks[0].Completed {
		t.Fatalf("相同处置不应完成整改任务: %#v", a.RevisionTasks)
	}
	beforeIncomplete := a.Case.Version
	_, err = svc.SubmitReview(ctx, caseID, command(a.Case.Version, "submit-incomplete"))
	var validation *domain.ValidationError
	if !errors.As(err, &validation) || validation.Issues[0].ItemID != target {
		t.Fatalf("重提应返回候选级任务缺口: %#v", err)
	}
	current, _ := svc.Get(ctx, caseID)
	if current.Case.Version != beforeIncomplete {
		t.Fatal("失败的重提不应递增版本")
	}
	a, err = svc.ResolveFinding(ctx, caseID, target, ResolveFindingInput{CommandMeta: command(a.Case.Version, "fix"), Disposition: domain.DispositionGeneralize, Rationale: "按意见泛化"})
	if err != nil {
		t.Fatal(err)
	}
	a, err = svc.SubmitReview(ctx, caseID, command(a.Case.Version, "submit-2"))
	if err != nil {
		t.Fatal(err)
	}
	a, err = svc.DecideReview(ctx, caseID, ReviewInput{CommandMeta: command(a.Case.Version, "approve"), Decision: domain.ReviewApproved, Comments: "批准", ReviewerCode: "R"})
	if err != nil {
		t.Fatal(err)
	}
	a, err = svc.IssueCredential(ctx, caseID, command(a.Case.Version, "release"))
	if err != nil {
		t.Fatal(err)
	}
	verification, err := svc.Verify(ctx, caseID)
	if err != nil || !verification.Valid {
		t.Fatalf("凭据无效: %#v %v", verification, err)
	}
	independent, err := svc.VerifyPresentedCredential(ctx, *a.Credential)
	if err != nil || !independent.Valid {
		t.Fatalf("独立凭据校验失败: %#v %v", independent, err)
	}
	tampered := *a.Credential
	tampered.ManifestDigest = "a" + tampered.ManifestDigest[1:]
	if tampered.ManifestDigest == a.Credential.ManifestDigest {
		tampered.ManifestDigest = "b" + tampered.ManifestDigest[1:]
	}
	independent, err = svc.VerifyPresentedCredential(ctx, tampered)
	if err != nil || independent.Valid || componentOK(independent, "manifestDigest") || componentOK(independent, "integrityHash") {
		t.Fatalf("篡改清单应同时定位摘要和完整性哈希: %#v %v", independent, err)
	}
	events, err := svc.Timeline(ctx, caseID)
	if err != nil || len(events) < 12 {
		t.Fatalf("审计轨迹不完整: %d %v", len(events), err)
	}
}

func componentOK(result audit.IndependentVerification, name string) bool {
	for _, component := range result.Components {
		if component.Name == name {
			return component.Consistent
		}
	}
	return false
}

func TestBatchCommandsIncrementOnceAndFailuresLeaveNoAudit(t *testing.T) {
	ctx := context.Background()
	svc, closeStore := testService(t)
	defer closeStore()
	a, err := svc.CreateCase(ctx, CreateCaseInput{ContributorCode: "C", CollectionContext: "批量访谈", LanguageTags: []string{"吴语"}, IntendedAudience: "公开", IdempotencyKey: "batch-create", Actor: "A"})
	if err != nil {
		t.Fatal(err)
	}
	before := a.Case.Version
	a, err = svc.AddSegments(ctx, a.Case.ID, AddSegmentsInput{CommandMeta: command(before, "batch-add"), Segments: []SegmentDraftInput{
		{Sequence: 1, SpeakerCode: "S1", StartMillis: 0, EndMillis: 1000, Transcript: "姓名：张三", Category: "叙事"},
		{Sequence: 2, SpeakerCode: "S2", StartMillis: 1000, EndMillis: 2000, Transcript: "电话13800138000", Category: "叙事"},
		{Sequence: 3, SpeakerCode: "S3", StartMillis: 2000, EndMillis: 3000, Transcript: "普通内容", Category: "叙事"},
	}})
	if err != nil || a.Case.Version != before+1 || len(a.Segments) != 3 {
		t.Fatalf("批量录入未原子递增一次: %#v %v", a, err)
	}
	events, _ := svc.Timeline(ctx, a.Case.ID)
	if events[len(events)-1].Action != "segments.batch_added" || !strings.Contains(events[len(events)-1].Details, a.Segments[2].ID) {
		t.Fatalf("批量审计未记录全部标识: %#v", events[len(events)-1])
	}
	beforeEvents := len(events)
	before = a.Case.Version
	_, err = svc.AddSegments(ctx, a.Case.ID, AddSegmentsInput{CommandMeta: command(before, "batch-invalid"), Segments: []SegmentDraftInput{{Sequence: 4, SpeakerCode: "", StartMillis: 0, EndMillis: 1, Transcript: "缺说话者", Category: "叙事"}, {Sequence: 2, SpeakerCode: "S", StartMillis: 0, EndMillis: 1, Transcript: "冲突", Category: "叙事"}}})
	var validation *domain.ValidationError
	if !errors.As(err, &validation) || len(validation.Issues) < 2 {
		t.Fatalf("批量字段错误未汇总: %#v", err)
	}
	current, _ := svc.Get(ctx, a.Case.ID)
	events, _ = svc.Timeline(ctx, a.Case.ID)
	if current.Case.Version != before || len(current.Segments) != 3 || len(events) != beforeEvents {
		t.Fatal("失败批次改变了案件或审计")
	}
	a, err = svc.RevokeSegments(ctx, a.Case.ID, SegmentIDsInput{CommandMeta: command(a.Case.Version, "revoke"), SegmentIDs: []string{a.Segments[1].ID}})
	if err != nil {
		t.Fatal(err)
	}
	a, err = svc.ReorderSegments(ctx, a.Case.ID, SegmentIDsInput{CommandMeta: command(a.Case.Version, "reorder"), SegmentIDs: []string{a.Segments[1].ID, a.Segments[0].ID}})
	if err != nil || a.Segments[0].Sequence != 1 || a.Segments[0].Transcript != "普通内容" {
		t.Fatalf("撤销与重排错误: %#v %v", a.Segments, err)
	}
}

func TestIdempotencyReplaysResultAndStaleVersionConflicts(t *testing.T) {
	ctx := context.Background()
	svc, closeStore := testService(t)
	defer closeStore()
	a, _ := svc.CreateCase(ctx, CreateCaseInput{ContributorCode: "C", CollectionContext: "背景", LanguageTags: []string{"客家话"}, IntendedAudience: "研究", IdempotencyKey: "create", Actor: "A"})
	input := AddSegmentInput{CommandMeta: command(a.Case.Version, "same-key"), Sequence: 1, SpeakerCode: "S", StartMillis: 0, EndMillis: 1, Transcript: "内容", Category: "叙事"}
	first, err := svc.AddSegment(ctx, a.Case.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := svc.AddSegment(ctx, a.Case.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Case.Version != first.Case.Version || len(replayed.Segments) != 1 {
		t.Fatalf("幂等重放不一致: %#v", replayed)
	}
	_, err = svc.RequestConsent(ctx, a.Case.ID, command(a.Case.Version, "stale"))
	if err == nil {
		t.Fatal("陈旧版本应冲突")
	}
}
