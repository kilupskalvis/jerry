package spec

import "testing"

func TestSuggest(t *testing.T) {
	valid := []string{"post_pr_comment", "add_check_status", "create_pull_request"}
	if got := Suggest("post_pr_coment", valid); got != "post_pr_comment" {
		t.Errorf("Suggest = %q", got)
	}
	if got := Suggest("zzzzz", valid); got != "" {
		t.Errorf("Suggest for nonsense = %q, want empty", got)
	}
}
