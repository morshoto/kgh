package github

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/shotomorisk/kgh/internal/execution"
)

const workflowRunsPageSize = 100

type baselineResolver interface {
	Resolve(context.Context, ReportContext, execution.Result) (*execution.ComparisonResult, error)
}

type BaselineResolver struct {
	Getenv     func(string) string
	HTTPClient httpDoer
}

func NewBaselineResolver() BaselineResolver {
	return BaselineResolver{
		Getenv:     os.Getenv,
		HTTPClient: http.DefaultClient,
	}
}

func (r BaselineResolver) Resolve(ctx context.Context, reportCtx ReportContext, current execution.Result) (*execution.ComparisonResult, error) {
	if current.Score == nil {
		return nil, nil
	}

	currentScore := strings.TrimSpace(current.Score.PublicScore)
	if currentScore == "" {
		return nil, nil
	}
	if reportCtx.RepositoryOwner == "" || reportCtx.RepositoryName == "" || reportCtx.BaseBranch == "" || reportCtx.WorkflowPath == "" {
		return nil, nil
	}
	if strings.TrimSpace(r.getenv("GITHUB_TOKEN")) == "" {
		return nil, nil
	}

	run, err := r.findLatestSuccessfulRun(ctx, reportCtx)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, nil
	}

	baselineScore, err := r.resolveBaselineScore(ctx, reportCtx, run.ID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(baselineScore) == "" {
		return nil, nil
	}

	delta, err := formatScoreDelta(currentScore, baselineScore)
	if err != nil {
		return nil, nil
	}

	return &execution.ComparisonResult{
		Label:         reportCtx.BaseBranch,
		BaselineScore: baselineScore,
		CurrentScore:  currentScore,
		Delta:         delta,
	}, nil
}

func (r BaselineResolver) findLatestSuccessfulRun(ctx context.Context, reportCtx ReportContext) (*workflowRun, error) {
	for page := 1; ; page++ {
		runs, err := r.listWorkflowRuns(ctx, reportCtx, page)
		if err != nil {
			return nil, err
		}
		if len(runs) == 0 {
			return nil, nil
		}
		for _, run := range runs {
			if strings.TrimSpace(run.Conclusion) != "success" {
				continue
			}
			if normalizeWorkflowPath(run.Path) != reportCtx.WorkflowPath {
				continue
			}
			return &run, nil
		}
		if len(runs) < workflowRunsPageSize {
			return nil, nil
		}
	}
}

func (r BaselineResolver) listWorkflowRuns(ctx context.Context, reportCtx ReportContext, page int) ([]workflowRun, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.apiURL()+"/repos/"+reportCtx.RepositoryOwner+"/"+reportCtx.RepositoryName+"/actions/runs?branch="+url.QueryEscape(reportCtx.BaseBranch)+"&status=completed&per_page="+fmt.Sprintf("%d", workflowRunsPageSize)+"&page="+fmt.Sprintf("%d", page), nil)
	if err != nil {
		return nil, fmt.Errorf("list GitHub workflow runs: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(r.getenv("GITHUB_TOKEN")))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	var response struct {
		WorkflowRuns []workflowRun `json:"workflow_runs"`
	}
	if err := r.doJSON(req, &response); err != nil {
		return nil, fmt.Errorf("list GitHub workflow runs: %w", err)
	}
	return response.WorkflowRuns, nil
}

func (r BaselineResolver) resolveBaselineScore(ctx context.Context, reportCtx ReportContext, runID int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.apiURL()+"/repos/"+reportCtx.RepositoryOwner+"/"+reportCtx.RepositoryName+"/actions/runs/"+fmt.Sprintf("%d", runID)+"/logs", nil)
	if err != nil {
		return "", fmt.Errorf("download GitHub workflow logs: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(r.getenv("GITHUB_TOKEN")))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := r.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("download GitHub workflow logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", httpStatusError(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read GitHub workflow logs: %w", err)
	}

	result, err := extractResultFromLogsArchive(body)
	if err != nil {
		return "", err
	}
	if result.Score == nil {
		return "", nil
	}
	return strings.TrimSpace(result.Score.PublicScore), nil
}

func (r BaselineResolver) doJSON(req *http.Request, out any) error {
	resp, err := r.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpStatusError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func extractResultFromLogsArchive(body []byte) (execution.Result, error) {
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return execution.Result{}, fmt.Errorf("decode GitHub workflow logs archive: %w", err)
	}

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		open, err := file.Open()
		if err != nil {
			return execution.Result{}, fmt.Errorf("read GitHub workflow log %q: %w", file.Name, err)
		}
		content, readErr := io.ReadAll(open)
		closeErr := open.Close()
		if readErr != nil {
			return execution.Result{}, fmt.Errorf("read GitHub workflow log %q: %w", file.Name, readErr)
		}
		if closeErr != nil {
			return execution.Result{}, fmt.Errorf("close GitHub workflow log %q: %w", file.Name, closeErr)
		}

		if result, ok := extractResultFromLogText(string(content)); ok {
			return result, nil
		}
	}

	return execution.Result{}, nil
}

func extractResultFromLogText(logText string) (execution.Result, bool) {
	for _, marker := range []string{"{\n  \"mode\"", "{\r\n  \"mode\"", "{\"mode\""} {
		index := strings.Index(logText, marker)
		if index < 0 {
			continue
		}
		decoder := json.NewDecoder(strings.NewReader(logText[index:]))
		decoder.UseNumber()
		var result execution.Result
		if err := decoder.Decode(&result); err == nil {
			return result, true
		}
	}
	return execution.Result{}, false
}

func formatScoreDelta(currentScore, baselineScore string) (string, error) {
	current, ok := new(big.Rat).SetString(strings.TrimSpace(currentScore))
	if !ok {
		return "", fmt.Errorf("invalid current score %q", currentScore)
	}
	baseline, ok := new(big.Rat).SetString(strings.TrimSpace(baselineScore))
	if !ok {
		return "", fmt.Errorf("invalid baseline score %q", baselineScore)
	}

	diff := new(big.Rat).Sub(current, baseline)
	precision := maxDecimalPrecision(currentScore, baselineScore)
	value := diff.FloatString(precision)
	value = strings.TrimRight(value, "0")
	value = strings.TrimRight(value, ".")
	if value == "" || value == "-" {
		value = "0"
	}
	if strings.HasPrefix(value, "-") {
		return value, nil
	}
	return "+" + value, nil
}

func maxDecimalPrecision(values ...string) int {
	max := 0
	for _, value := range values {
		value = strings.TrimSpace(value)
		if i := strings.IndexByte(value, '.'); i >= 0 {
			if precision := len(value) - i - 1; precision > max {
				max = precision
			}
		}
	}
	return max
}

func normalizeWorkflowPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if base, _, ok := strings.Cut(path, "@"); ok {
		return strings.TrimSpace(base)
	}
	return path
}

func (r BaselineResolver) getenv(key string) string {
	if r.Getenv != nil {
		return r.Getenv(key)
	}
	return os.Getenv(key)
}

func (r BaselineResolver) client() httpDoer {
	if r.HTTPClient != nil {
		return r.HTTPClient
	}
	return http.DefaultClient
}

func (r BaselineResolver) apiURL() string {
	base := strings.TrimSpace(r.getenv("GITHUB_API_URL"))
	if base == "" {
		base = defaultGitHubAPIURL
	}
	return strings.TrimRight(base, "/")
}

type workflowRun struct {
	ID           int64  `json:"id"`
	Path         string `json:"path"`
	Conclusion   string `json:"conclusion"`
	HeadBranch   string `json:"head_branch"`
	RunAttempt   int    `json:"run_attempt"`
	WorkflowID   int64  `json:"workflow_id"`
	DisplayTitle string `json:"display_title"`
}
