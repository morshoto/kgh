//go:build smoke && unix

package kaggle

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"os/exec"
)

func TestSmokeKaggleAdapterLive(t *testing.T) {
	competition := strings.TrimSpace(os.Getenv("KGH_KAGGLE_SMOKE_COMPETITION"))
	if competition == "" {
		t.Fatal("KGH_KAGGLE_SMOKE_COMPETITION is required")
	}
	if os.Getenv("KGH_KAGGLE_SMOKE") != "1" {
		t.Fatal("set KGH_KAGGLE_SMOKE=1 to run the Kaggle smoke test")
	}
	requireExplicitSmokeCredentials(t)

	client := NewClientWithDeps(
		nil,
		osEnvSource{},
		exec.LookPath,
		currentEnv,
		2*time.Minute,
		nil,
	)
	adapter := NewAdapter(client)

	resp, err := adapter.ListCompetitionSubmissions(context.Background(), CompetitionSubmissionsRequest{
		Competition: competition,
	})
	if err != nil {
		t.Fatalf("smoke test failed: %v", err)
	}

	t.Logf("retrieved %d submission rows for %s", len(resp.Submissions), competition)
}

func requireExplicitSmokeCredentials(t *testing.T) {
	t.Helper()

	token := strings.TrimSpace(os.Getenv(envKaggleAPIToken))
	username := strings.TrimSpace(os.Getenv(envKaggleUsername))
	key := strings.TrimSpace(os.Getenv(envKaggleKey))

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
