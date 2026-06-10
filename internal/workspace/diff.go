// Package workspace treats the git working tree as the auditable channel
// between steps: what did this step change, regardless of whether the
// agent edited, staged, or committed.
package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PreState is the recorded tree state before a step runs.
type PreState struct {
	Head string
}

// DiffSnapshot is what one step changed.
type DiffSnapshot struct {
	Patch string
	Stat  string
}

// RecordState captures HEAD before a step runs.
func RecordState(repo string) (PreState, error) {
	head, err := git(repo, "rev-parse", "HEAD")
	if err != nil {
		return PreState{}, fmt.Errorf("recording pre-step state: %w", err)
	}
	return PreState{Head: strings.TrimSpace(head)}, nil
}

// Capture diffs the current tree (including uncommitted work, untracked
// files, and any commits the step made) against the pre-step state.
func Capture(repo string, pre PreState) (DiffSnapshot, error) {
	patch, err := git(repo, "diff", pre.Head)
	if err != nil {
		return DiffSnapshot{}, fmt.Errorf("diffing against %s: %w", pre.Head, err)
	}
	stat, err := git(repo, "diff", "--stat", pre.Head)
	if err != nil {
		return DiffSnapshot{}, err
	}

	untracked, err := git(repo, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return DiffSnapshot{}, err
	}
	var extra, extraStat strings.Builder
	for f := range strings.FieldsSeq(strings.TrimSpace(untracked)) {
		// --no-index exits 1 when files differ; that is success here.
		out, diffErr := gitAllowExit1(repo, "diff", "--no-index", "--", os.DevNull, filepath.Join(repo, f))
		if diffErr != nil {
			return DiffSnapshot{}, fmt.Errorf("diffing untracked %s: %w", f, diffErr)
		}
		extra.WriteString(out)
		fmt.Fprintf(&extraStat, " %s (new)\n", f)
	}

	return DiffSnapshot{
		Patch: patch + extra.String(),
		Stat:  strings.TrimSpace(stat + extraStat.String()),
	}, nil
}

func git(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), errb.String(), err)
	}
	return out.String(), nil
}

func gitAllowExit1(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if err == nil {
		return out.String(), nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return out.String(), nil
	}
	return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), errb.String(), err)
}
