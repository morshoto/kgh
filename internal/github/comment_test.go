package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestPRCommentWriterCreatesCommentWhenMarkerMissing(t *testing.T) {
	t.Parallel()

	var requests []string
	writer := PRCommentWriter{
		Getenv: envMap(map[string]string{
			"GITHUB_TOKEN": "token",
		}),
		HTTPClient: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests = append(requests, req.Method+" "+req.URL.Path)
			switch req.Method {
			case http.MethodGet:
				return jsonResponse(http.StatusOK, `[]`), nil
			case http.MethodPost:
				var payload struct {
					Body string `json:"body"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				if !strings.Contains(payload.Body, commentMarker) {
					t.Fatalf("expected marker in body, got %s", payload.Body)
				}
				return jsonResponse(http.StatusCreated, `{}`), nil
			default:
				t.Fatalf("unexpected method %s", req.Method)
				return nil, nil
			}
		}),
	}

	err := writer.Write(context.Background(), ReportContext{
		RepositoryOwner:   "shotomorisk",
		RepositoryName:    "kgh",
		PullRequestNumber: 17,
	}, commentMarker+"\nbody")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %v", requests)
	}
}

func TestPRCommentWriterUpdatesExistingMarkerComment(t *testing.T) {
	t.Parallel()

	var patched bool
	writer := PRCommentWriter{
		Getenv: envMap(map[string]string{
			"GITHUB_TOKEN": "token",
		}),
		HTTPClient: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.Method {
			case http.MethodGet:
				return jsonResponse(http.StatusOK, `[{"id":123,"body":"`+commentMarker+` old"}]`), nil
			case http.MethodPatch:
				patched = true
				if got := req.URL.Path; got != "/repos/shotomorisk/kgh/issues/comments/123" {
					t.Fatalf("unexpected patch path %q", got)
				}
				return jsonResponse(http.StatusOK, `{}`), nil
			default:
				t.Fatalf("unexpected method %s", req.Method)
				return nil, nil
			}
		}),
	}

	err := writer.Write(context.Background(), ReportContext{
		RepositoryOwner:   "shotomorisk",
		RepositoryName:    "kgh",
		PullRequestNumber: 17,
	}, commentMarker+"\nbody")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !patched {
		t.Fatal("expected patch request")
	}
}

func TestPRCommentWriterFindsExistingMarkerCommentAcrossPages(t *testing.T) {
	t.Parallel()

	var getPages []string
	var patched bool
	writer := PRCommentWriter{
		Getenv: envMap(map[string]string{
			"GITHUB_TOKEN": "token",
		}),
		HTTPClient: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.Method {
			case http.MethodGet:
				getPages = append(getPages, req.URL.RawQuery)
				page := req.URL.Query().Get("page")
				switch page {
				case "1":
					comments := make([]map[string]any, commentsPageSize)
					for i := range comments {
						comments[i] = map[string]any{"id": i + 1, "body": "other"}
					}
					payload, err := json.Marshal(comments)
					if err != nil {
						t.Fatalf("marshal page 1 comments: %v", err)
					}
					return jsonResponse(http.StatusOK, string(payload)), nil
				case "2":
					return jsonResponse(http.StatusOK, `[{"id":123,"body":"`+commentMarker+` old"}]`), nil
				default:
					t.Fatalf("unexpected page %q", page)
					return nil, nil
				}
			case http.MethodPatch:
				patched = true
				if got := req.URL.Path; got != "/repos/shotomorisk/kgh/issues/comments/123" {
					t.Fatalf("unexpected patch path %q", got)
				}
				return jsonResponse(http.StatusOK, `{}`), nil
			default:
				t.Fatalf("unexpected method %s", req.Method)
				return nil, nil
			}
		}),
	}

	err := writer.Write(context.Background(), ReportContext{
		RepositoryOwner:   "shotomorisk",
		RepositoryName:    "kgh",
		PullRequestNumber: 17,
	}, commentMarker+"\nbody")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !patched {
		t.Fatal("expected patch request")
	}
	if len(getPages) != 2 {
		t.Fatalf("expected 2 comment list requests, got %v", getPages)
	}
	for i, rawQuery := range getPages {
		values := mustParseQuery(t, rawQuery)
		if values.Get("per_page") != strconv.Itoa(commentsPageSize) {
			t.Fatalf("expected per_page=%d for request %d, got %q", commentsPageSize, i+1, values.Get("per_page"))
		}
	}
}

func TestPRCommentWriterRejectsMissingToken(t *testing.T) {
	t.Parallel()

	err := PRCommentWriter{
		Getenv: envMap(map[string]string{}),
	}.Write(context.Background(), ReportContext{
		RepositoryOwner:   "shotomorisk",
		RepositoryName:    "kgh",
		PullRequestNumber: 17,
	}, "body")
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); got != "upsert GitHub PR comment: GITHUB_TOKEN is required" {
		t.Fatalf("unexpected error %q", got)
	}
}

func TestPRCommentWriterWrapsHTTPError(t *testing.T) {
	t.Parallel()

	writer := PRCommentWriter{
		Getenv: envMap(map[string]string{
			"GITHUB_TOKEN": "token",
		}),
		HTTPClient: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		}),
	}

	err := writer.Write(context.Background(), ReportContext{
		RepositoryOwner:   "shotomorisk",
		RepositoryName:    "kgh",
		PullRequestNumber: 17,
	}, "body")
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); got != "upsert GitHub PR comment: boom" {
		t.Fatalf("unexpected error %q", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func mustParseQuery(t *testing.T, rawQuery string) mapQuery {
	t.Helper()
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatalf("parse query %q: %v", rawQuery, err)
	}
	return mapQuery(values)
}

type mapQuery map[string][]string

func (q mapQuery) Get(key string) string {
	values := map[string][]string(q)
	if len(values[key]) == 0 {
		return ""
	}
	return values[key][0]
}
