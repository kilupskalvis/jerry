package citools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kilupskalvis/jerry/internal/trigger"
)

func testClient(t *testing.T, srvURL string, td *trigger.TriggerData) *Client {
	t.Helper()
	c, err := NewClient(td, Config{Token: "tok", BaseURL: srvURL})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestPostPRComment(t *testing.T) {
	var gotPath, gotAuth string
	var payload map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &payload)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := testClient(t, srv.URL, &trigger.TriggerData{RepoOwner: "o", RepoName: "r", Number: 7})
	msg, err := c.PostPRComment("hello world")
	if err != nil {
		t.Fatalf("PostPRComment: %v", err)
	}
	if gotPath != "/repos/o/r/issues/7/comments" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth = %q", gotAuth)
	}
	if payload["body"] != "hello world" {
		t.Errorf("payload body = %v", payload)
	}
	if msg == "" {
		t.Error("empty result message")
	}
}

func TestPostPRCommentNoNumber(t *testing.T) {
	c := testClient(t, "http://unused", &trigger.TriggerData{RepoOwner: "o", RepoName: "r"})
	if _, err := c.PostPRComment("x"); err == nil {
		t.Fatal("want error when trigger has no number")
	}
}

func TestPostReviewComment(t *testing.T) {
	var gotPath string
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &payload)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := testClient(t, srv.URL, &trigger.TriggerData{RepoOwner: "o", RepoName: "r", Number: 7, HeadSHA: "deadbeef"})
	if _, err := c.PostReviewComment("main.go", 12, "nit"); err != nil {
		t.Fatalf("PostReviewComment: %v", err)
	}
	if gotPath != "/repos/o/r/pulls/7/comments" {
		t.Errorf("path = %q", gotPath)
	}
	if payload["commit_id"] != "deadbeef" || payload["path"] != "main.go" || payload["line"].(float64) != 12 {
		t.Errorf("payload = %v", payload)
	}
}

func TestAddCheckStatus(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &payload)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := testClient(t, srv.URL, &trigger.TriggerData{RepoOwner: "o", RepoName: "r", HeadSHA: "sha1"})
	if _, err := c.AddCheckStatus("Jerry Review", "success", "all good"); err != nil {
		t.Fatalf("AddCheckStatus: %v", err)
	}
	if payload["status"] != "completed" || payload["conclusion"] != "success" || payload["head_sha"] != "sha1" {
		t.Errorf("payload = %v", payload)
	}
}

func TestAddCheckStatusInvalidStatus(t *testing.T) {
	c := testClient(t, "http://unused", &trigger.TriggerData{RepoOwner: "o", RepoName: "r", HeadSHA: "s"})
	if _, err := c.AddCheckStatus("x", "maybe", "y"); err == nil {
		t.Fatal("want error for invalid status")
	}
}

func TestNewClientErrors(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_REPOSITORY", "")

	if _, err := NewClient(&trigger.TriggerData{RepoOwner: "o", RepoName: "r"}, Config{}); err == nil {
		t.Error("want error with no token")
	}
	if _, err := NewClient(nil, Config{Token: "t"}); err == nil {
		t.Error("want error with nil trigger")
	}
	if _, err := NewClient(&trigger.TriggerData{}, Config{Token: "t"}); err == nil {
		t.Error("want error with no repo")
	}
}

func TestNewClientEnvRepoFallback(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "acme/widgets")
	c, err := NewClient(&trigger.TriggerData{}, Config{Token: "t"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.owner != "acme" || c.repo != "widgets" {
		t.Errorf("owner/repo = %q/%q", c.owner, c.repo)
	}
}

func TestOpenPR(t *testing.T) {
	var payload map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &payload)
		_, _ = w.Write([]byte(`{"html_url":"https://github.com/o/r/pull/9","number":9}`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL, &trigger.TriggerData{RepoOwner: "o", RepoName: "r"})
	url, num, err := c.openPR("jerry/x", "main", "Title", "Body")
	if err != nil {
		t.Fatalf("openPR: %v", err)
	}
	if num != 9 || url != "https://github.com/o/r/pull/9" {
		t.Errorf("got %d, %q", num, url)
	}
	if payload["head"] != "jerry/x" || payload["base"] != "main" || payload["title"] != "Title" {
		t.Errorf("payload = %v", payload)
	}
}

func TestSanitizeBranch(t *testing.T) {
	cases := map[string]string{
		"Add pagination": "add-pagination",
		"Fix: the/bug":   "fix-the-bug",
	}
	for in, want := range cases {
		if got := sanitizeBranch(in); got != want {
			t.Errorf("sanitizeBranch(%q) = %q, want %q", in, got, want)
		}
	}
}
