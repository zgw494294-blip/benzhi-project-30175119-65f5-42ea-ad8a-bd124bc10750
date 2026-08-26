package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func Digest(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func MustDigest(value any) string {
	d, err := Digest(value)
	if err != nil {
		panic(err)
	}
	return d
}

type digestComponent struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func IntegrityHash(manifest, consent, approval string, caseVersion int64, caseID string) string {
	return MustDigest(struct {
		CaseID      string            `json:"caseId"`
		CaseVersion int64             `json:"caseVersion"`
		Components  []digestComponent `json:"components"`
	}{
		CaseID: caseID, CaseVersion: caseVersion, Components: []digestComponent{{"manifest", manifest}, {"consent", consent}, {"approval", approval}},
	})
}
