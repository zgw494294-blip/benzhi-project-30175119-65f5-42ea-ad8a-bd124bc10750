package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type selfAggregate struct {
	Case struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	} `json:"case"`
	Segments []struct {
		ID string `json:"id"`
	} `json:"segments"`
	Findings []struct {
		ID string `json:"id"`
	} `json:"findings"`
	Credential any `json:"credential"`
}

func executeSelfcheck(server *http.Server, addr string, serveErr <-chan error) error {
	client := &http.Client{Timeout: 3 * time.Second}
	base := "http://" + normalizeAddress(addr)
	sequence := int64(0)
	key := func() string { sequence++; return fmt.Sprintf("selfcheck-%d", sequence) }
	var a selfAggregate
	if err := selfRequest(client, "POST", base+"/api/cases", map[string]any{"contributorCode": "SELF-C01", "collectionContext": "自检方言口述采集", "languageTags": []string{"吴语"}, "intendedAudience": "自检公开展示", "actor": "SELF-ARCHIVIST", "idempotencyKey": key()}, &a); err != nil {
		return shutdownSelfcheck(server, err)
	}
	caseID := a.Case.ID
	meta := func() map[string]any {
		return map[string]any{"expectedVersion": a.Case.Version, "idempotencyKey": key(), "actor": "SELF-OP"}
	}
	segments := meta()
	segments["segments"] = []map[string]any{
		{"sequence": 1, "speakerCode": "SPK-S1", "startMillis": 0, "endMillis": 5000, "transcript": "姓名：张三住在河西村12号，电话13800138000。", "category": "口述史"},
		{"sequence": 2, "speakerCode": "SPK-S2", "startMillis": 5000, "endMillis": 8000, "transcript": "第二段自检转写。", "category": "叙事"},
		{"sequence": 3, "speakerCode": "SPK-S3", "startMillis": 8000, "endMillis": 11000, "transcript": "第三段自检转写。", "category": "叙事"},
	}
	if err := selfRequest(client, "POST", base+"/api/cases/"+caseID+"/segments/batch", segments, &a); err != nil {
		return shutdownSelfcheck(server, err)
	}
	if len(a.Segments) != 3 {
		return shutdownSelfcheck(server, fmt.Errorf("批量片段未完整写入：%d", len(a.Segments)))
	}
	originalSecond := a.Segments[1].ID
	reorder := meta()
	reorder["segmentIds"] = []string{a.Segments[2].ID, a.Segments[0].ID, a.Segments[1].ID}
	if err := selfRequest(client, "POST", base+"/api/cases/"+caseID+"/segments/reorder", reorder, &a); err != nil {
		return shutdownSelfcheck(server, err)
	}
	revoke := meta()
	revoke["segmentIds"] = []string{originalSecond}
	if err := selfRequest(client, "POST", base+"/api/cases/"+caseID+"/segments/revoke", revoke, &a); err != nil {
		return shutdownSelfcheck(server, err)
	}
	if err := selfRequest(client, "POST", base+"/api/cases/"+caseID+"/request-consent", meta(), &a); err != nil {
		return shutdownSelfcheck(server, err)
	}
	var views struct {
		Checklist struct {
			ScopeDigest string `json:"scopeDigest"`
		} `json:"checklist"`
	}
	if err := selfRequest(client, "GET", base+"/api/cases/"+caseID+"/views", nil, &views); err != nil {
		return shutdownSelfcheck(server, err)
	}
	consent := meta()
	consent["researchAllowed"] = true
	consent["teachingAllowed"] = true
	consent["publicDisplayAllowed"] = true
	consent["confirmedBy"] = "SELF-C01"
	consent["scopeDigest"] = views.Checklist.ScopeDigest
	if err := selfRequest(client, "POST", base+"/api/cases/"+caseID+"/confirm-consent", consent, &a); err != nil {
		return shutdownSelfcheck(server, err)
	}
	if err := selfRequest(client, "POST", base+"/api/cases/"+caseID+"/scan", meta(), &a); err != nil {
		return shutdownSelfcheck(server, err)
	}
	if len(a.Findings) < 3 {
		return shutdownSelfcheck(server, fmt.Errorf("敏感扫描候选不足：得到 %d 项", len(a.Findings)))
	}
	resolution := meta()
	decisions := make([]map[string]any, 0, len(a.Findings))
	for _, finding := range a.Findings {
		decisions = append(decisions, map[string]any{"findingId": finding.ID, "disposition": "mask", "rationale": "自检批量遮蔽敏感信息"})
	}
	resolution["decisions"] = decisions
	if err := selfRequest(client, "POST", base+"/api/cases/"+caseID+"/findings/batch", resolution, &a); err != nil {
		return shutdownSelfcheck(server, err)
	}
	if err := selfRequest(client, "POST", base+"/api/cases/"+caseID+"/submit-review", meta(), &a); err != nil {
		return shutdownSelfcheck(server, err)
	}
	review := meta()
	review["decision"] = "approved"
	review["comments"] = "自检批准"
	review["targetFindingIds"] = []string{}
	review["reviewerCode"] = "SELF-ETHICS"
	if err := selfRequest(client, "POST", base+"/api/cases/"+caseID+"/review", review, &a); err != nil {
		return shutdownSelfcheck(server, err)
	}
	if err := selfRequest(client, "POST", base+"/api/cases/"+caseID+"/release", meta(), &a); err != nil {
		return shutdownSelfcheck(server, err)
	}
	if a.Credential == nil {
		return shutdownSelfcheck(server, errors.New("未签发发布凭据"))
	}
	var verification struct {
		Valid   bool   `json:"valid"`
		Message string `json:"message"`
	}
	if err := selfRequest(client, "GET", base+"/api/cases/"+caseID+"/verify", nil, &verification); err != nil {
		return shutdownSelfcheck(server, err)
	}
	if !verification.Valid {
		return shutdownSelfcheck(server, fmt.Errorf("凭据校验未通过：%s", verification.Message))
	}
	if err := selfRequest(client, "POST", base+"/api/credentials/verify", a.Credential, &verification); err != nil {
		return shutdownSelfcheck(server, err)
	}
	if !verification.Valid {
		return shutdownSelfcheck(server, fmt.Errorf("独立凭据校验未通过：%s", verification.Message))
	}
	var timeline struct {
		Events []any `json:"events"`
	}
	if err := selfRequest(client, "GET", base+"/api/cases/"+caseID+"/audit", nil, &timeline); err != nil {
		return shutdownSelfcheck(server, err)
	}
	if len(timeline.Events) < 10 {
		return shutdownSelfcheck(server, fmt.Errorf("审计事件不完整：%d", len(timeline.Events)))
	}
	if err := shutdownSelfcheck(server, nil); err != nil {
		return err
	}
	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-time.After(time.Second):
		return errors.New("自检 HTTP 服务未按时关闭")
	}
	fmt.Println("自检通过：真实 HTTP 流程完成批量片段、同意回执、扫描、批量处置、批准、发布及独立凭据校验")
	return nil
}

func normalizeAddress(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}
func selfRequest(client *http.Client, method, url string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%s %s 返回 %d：%s", method, url, res.StatusCode, string(b))
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}
func shutdownSelfcheck(server *http.Server, cause error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	shutdownErr := server.Shutdown(ctx)
	if cause != nil {
		return cause
	}
	return shutdownErr
}
