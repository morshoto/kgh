//go:build smoke && unix

package execution

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shotomorisk/kgh/internal/kaggle"
	"github.com/shotomorisk/kgh/internal/spec"
)

const (
	smokeCompetitionEnv = "KGH_KAGGLE_SMOKE_COMPETITION"
	smokeSubmitEnv      = "KGH_KAGGLE_SMOKE_SUBMIT"
	smokeTargetName     = "smoke-submit"
	smokeKernelRef      = "smoke/local"
	smokeListTimeout    = 2 * time.Minute
	smokeListInterval   = 5 * time.Second
)

func TestSmokeLiveCompetitionSubmissionScoreMatching(t *testing.T) {
	competition := strings.TrimSpace(os.Getenv(smokeCompetitionEnv))
	if competition == "" {
		t.Fatalf("%s is required", smokeCompetitionEnv)
	}
	if os.Getenv(smokeSubmitEnv) != "1" {
		t.Fatalf("set %s=1 to run the Kaggle submit smoke test", smokeSubmitEnv)
	}
	requireExplicitSmokeCredentials(t)

	csvPath := writeSmokeSubmissionFile(t)
	adapter := kaggle.NewAdapter(kaggle.NewClientWithDeps(
		nil,
		osEnvSource{},
		exec.LookPath,
		currentEnv,
		2*time.Minute,
		nil,
	))

	execSpec := spec.ExecutionSpec{
		TargetName:  smokeTargetName,
		Competition: competition,
		Submit:      true,
	}
	attemptedAt := time.Now().UTC()
	message := buildCompetitionSubmitMessage(execSpec, smokeKernelRef)

	submitResp, err := adapter.SubmitCompetition(context.Background(), kaggle.CompetitionSubmitRequest{
		Competition: competition,
		FilePath:    csvPath,
		Message:     message,
	})
	if err != nil {
		t.Fatalf("submit competition: %v", err)
	}
	if !submitResp.Submitted {
		t.Fatalf("expected submission to be accepted, got %+v", submitResp)
	}

	submission := &SubmissionResult{
		Attempted:   true,
		Submitted:   submitResp.Submitted,
		Competition: submitResp.Competition,
		FilePath:    csvPath,
		FileName:    filepath.Base(csvPath),
		Message:     message,
		AttemptedAt: attemptedAt,
	}

	var score *ScoreResult
	deadline := time.Now().Add(smokeListTimeout)
	for {
		resp, err := adapter.ListCompetitionSubmissions(context.Background(), kaggle.CompetitionSubmissionsRequest{
			Competition: competition,
		})
		if err != nil {
			t.Fatalf("list competition submissions: %v", err)
		}

		score = resolveScoreResult(competition, submission, resp.Submissions)
		if score.State == ScoreStateReady || score.State == ScoreStatePending {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for submitted row to appear in Kaggle submissions: %+v", score)
		}

		t.Logf("submission row not visible yet; retrying in %s", smokeListInterval)
		time.Sleep(smokeListInterval)
	}

	if score.State != ScoreStateReady && score.State != ScoreStatePending {
		t.Fatalf("unexpected score state %+v", score)
	}
	if score.FileName != filepath.Base(csvPath) {
		t.Fatalf("unexpected matched file name %q", score.FileName)
	}
	if score.Message != message {
		t.Fatalf("unexpected matched message %q", score.Message)
	}

	t.Logf("resolved live score state=%s status=%s public_score=%q submitted_at=%s", score.State, score.Status, score.PublicScore, score.SubmittedAt.Format(time.RFC3339))
}

func writeSmokeSubmissionFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "submission.csv")
	if err := os.WriteFile(path, []byte("id,label\n1,0\n"), 0o600); err != nil {
		t.Fatalf("write smoke submission file: %v", err)
	}
	return path
}

func requireExplicitSmokeCredentials(t *testing.T) {
	t.Helper()

	token := strings.TrimSpace(os.Getenv("KAGGLE_API_TOKEN"))
	username := strings.TrimSpace(os.Getenv("KAGGLE_USERNAME"))
	key := strings.TrimSpace(os.Getenv("KAGGLE_KEY"))

	switch {
	case token != "":
		return
	case username != "" && key != "":
		return
	case username != "" || key != "":
		t.Fatal("set both KAGGLE_USERNAME and KAGGLE_KEY, or set KAGGLE_API_TOKEN")
	default:
		t.Fatal("set KAGGLE_API_TOKEN, or set KAGGLE_USERNAME and KAGGLE_KEY")
	}
}

type osEnvSource struct{}

func (osEnvSource) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

func currentEnv() []string {
	return os.Environ()
}
