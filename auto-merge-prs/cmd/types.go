package main

type PullRequest struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	Head      PRBranch `json:"head"`
	Base      PRBranch `json:"base"`
	Labels    []Label  `json:"labels"`
	Mergeable bool     `json:"mergeable"`
}

type PRBranch struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

type Label struct {
	Name string `json:"name"`
}

type GiteaBranchProtection struct {
	StatusCheckContexts []string `json:"status_check_contexts"`
}

type GitHubBranchProtection struct {
	RequiredStatusChecks *struct {
		Contexts []string `json:"contexts"`
	} `json:"required_status_checks"`
}

type CommitStatuses struct {
	Statuses []struct {
		Context string `json:"context"`
		Status  string `json:"status"`
		State   string `json:"state"`
	} `json:"statuses"`
}

type CommitChecks struct {
	CheckRuns []struct {
		Name   string `json:"name"`
		Status string `json:"conclusion"`
	} `json:"check_runs"`
}

type ParsedCommitStatuses struct {
	Passing []string
	Failing []string
	Pending []string
	Other   []string
}

type GiteaMergeRequest struct {
	Method string `json:"Do"`
}

type GitHubMergeRequest struct {
	Method string `json:"merge_method"`
}

func (pr PullRequest) hasLabel(labelName string) bool {
	for _, l := range pr.Labels {
		if l.Name == labelName {
			return true
		}
	}
	return false
}
