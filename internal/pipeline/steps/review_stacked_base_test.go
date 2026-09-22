package steps

// Local patch (not upstream): a stacked run must be reviewed against the
// run's selected PR base branch, not the repository default branch. Kept in
// its own file so it survives upstream churn in review_test.go.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kunchenguid/no-mistakes/internal/agent"
	"github.com/kunchenguid/no-mistakes/internal/config"
)

func TestReviewStep_UsesRunPRBaseBranchForDiff(t *testing.T) {
	dir, mainBaseSHA, _ := setupGitRepo(t)
	gitCmd(t, dir, "checkout", "-b", "stacked-base", mainBaseSHA)
	if err := os.WriteFile(filepath.Join(dir, "stacked.txt"), []byte("stacked base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "stacked.txt")
	gitCmd(t, dir, "commit", "-m", "stacked base")
	stackedBaseSHA := gitCmd(t, dir, "rev-parse", "HEAD")
	gitCmd(t, dir, "checkout", "-B", "feature")
	if err := os.WriteFile(filepath.Join(dir, "stacked-feature.txt"), []byte("feature change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "stacked-feature.txt")
	gitCmd(t, dir, "commit", "-m", "feature change")
	headSHA := gitCmd(t, dir, "rev-parse", "HEAD")

	ag := &mockAgent{name: "test", runFn: func(_ context.Context, opts agent.RunOpts) (*agent.Result, error) {
		return &agent.Result{Output: json.RawMessage(`{"findings":[],"risk_level":"low","risk_rationale":"clean","risk_scope":"source-or-external"}`)}, nil
	}}
	sctx := newTestContextWithDBRecords(t, ag, dir, mainBaseSHA, headSHA, config.Commands{})
	baseBranch := "stacked-base"
	sctx.Run.PRBaseBranch = &baseBranch

	if _, err := (&ReviewStep{}).Execute(sctx); err != nil {
		t.Fatal(err)
	}
	if len(ag.calls) != 1 {
		t.Fatalf("review agent calls = %d, want 1", len(ag.calls))
	}
	prompt := ag.calls[0].Prompt
	if !strings.Contains(prompt, "base commit: "+stackedBaseSHA) {
		t.Errorf("review prompt base = %q, want stacked base %s", prompt, stackedBaseSHA)
	}
	if strings.Contains(prompt, "base commit: "+mainBaseSHA) {
		t.Errorf("review prompt used repository default base %s instead of stacked base %s", mainBaseSHA, stackedBaseSHA)
	}
}
