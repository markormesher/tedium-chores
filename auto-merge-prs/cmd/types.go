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

type CommitStatus struct {
	Statuses []struct {
		Status  string `json:"status"`
		State   string `json:"state"`
		Context string `json:"context"`
	} `json:"statuses"`
}

func (pr PullRequest) hasLabel(labelName string) bool {
	for _, l := range pr.Labels {
		if l.Name == labelName {
			return true
		}
	}
	return false
}
