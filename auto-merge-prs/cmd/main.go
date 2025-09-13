package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

var jsonHandler = slog.NewJSONHandler(os.Stdout, nil)
var l = slog.New(jsonHandler)

var (
	apiBase   = os.Getenv("TEDIUM_PLATFORM_API_BASE_URL")
	apiToken  = os.Getenv("TEDIUM_PLATFORM_TOKEN")
	apiType   = os.Getenv("TEDIUM_PLATFORM_TYPE")
	repoOwner = os.Getenv("TEDIUM_REPO_OWNER")
	repoName  = os.Getenv("TEDIUM_REPO_NAME")

	httpClient = &http.Client{Timeout: time.Second * 15}

	maxMergeableRetries = 5
	mergeableRetryDelay = time.Second * 5
)

func main() {
	if apiType != "gitea" && apiType != "github" {
		l.Error("unsupported API type", "type", apiType)
		os.Exit(1)
	}

	prs := getPRs()

	for _, pr := range prs {
		l.Info("considering PR", "title", pr.Title)

		if !pr.hasLabel("automerge") {
			l.Info("skipping PR, no automerge label", "title", pr.Title)
			continue
		}

		// wait for it to become mergeable
		for tries := 0; tries < maxMergeableRetries && !pr.Mergeable; tries++ {
			l.Info("waiting for PR to become mergeable")
			time.Sleep(mergeableRetryDelay)
			pr = getPR(pr.Number)
		}

		branchProtected, requiredContexts := getBranchProtection(pr)
		passingContexts := getPassingContexts(pr)

		doMerge, reason := shouldMerge(branchProtected, requiredContexts, passingContexts)
		if !doMerge {
			l.Info("PR cannot be merged", "reason", reason)
			continue
		}

		l.Info("attempting to merge PR...")
		mergePR(pr)
		l.Info("merged")

		// slight pause to make sure mergability of other PRs is re-evaluated by the platform
		time.Sleep(time.Second * 2)
	}
}

func getPRs() []PullRequest {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open", apiBase, repoOwner, repoName)
	data, _ := doRequest("GET", url, nil)
	var prs []PullRequest
	err := json.Unmarshal(data, &prs)
	if err != nil {
		l.Error("error parsing PR list", "error", err)
	}
	return prs
}

func getPR(number int) PullRequest {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", apiBase, repoOwner, repoName, number)
	data, _ := doRequest("GET", url, nil)
	var pr PullRequest
	err := json.Unmarshal(data, &pr)
	if err != nil {
		l.Error("error parsing PR", "error", err)
	}
	return pr
}

func getBranchProtection(pr PullRequest) (bool, []string) {
	switch apiType {
	case "gitea":
		url := fmt.Sprintf("%s/repos/%s/%s/branch_protections/%s", apiBase, repoOwner, repoName, pr.Base.Ref)
		data, status := doRequest("GET", url, nil)
		if status == http.StatusNotFound {
			return false, nil
		}

		var bp GiteaBranchProtection
		err := json.Unmarshal(data, &bp)
		if err != nil {
			l.Error("error parsing branch protection", "error", err)
		}

		if len(bp.StatusCheckContexts) > 0 {
			return true, bp.StatusCheckContexts
		}

	case "github":
		url := fmt.Sprintf("%s/repos/%s/%s/branches/%s/protection", apiBase, repoOwner, repoName, pr.Base.Ref)
		data, status := doRequest("GET", url, nil)
		if status == http.StatusNotFound {
			return false, nil
		}

		var bp GitHubBranchProtection
		err := json.Unmarshal(data, &bp)
		if err != nil {
			l.Error("error parsing branch protection", "error", err)
		}

		if bp.RequiredStatusChecks != nil {
			return true, bp.RequiredStatusChecks.Contexts
		}
	}

	return false, nil
}

func getPassingContexts(pr PullRequest) []string {
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s/status", apiBase, repoOwner, repoName, pr.Head.SHA)
	data, _ := doRequest("GET", url, nil)

	var combinedStatus CommitStatus
	err := json.Unmarshal(data, &combinedStatus)
	if err != nil {
		l.Error("error parsing commit status", "error", err)
	}

	passing := []string{}
	for _, s := range combinedStatus.Statuses {
		// github uses "state", gitea uses "status", so check both
		if s.Status == "success" || s.State == "success" {
			passing = append(passing, s.Context)
		}
	}

	return passing
}

func mergePR(pr PullRequest) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/merge", apiBase, repoOwner, repoName, pr.Number)
	body := strings.NewReader(fmt.Sprintf(`{"Do":"squash","merge_message":"Squash merge PR #%d: %s"}`, pr.Number, pr.Title))

	var data []byte
	var status int

	switch apiType {
	case "gitea":
		data, status = doRequest("POST", url, body)

	case "github":
		data, status = doRequest("PUT", url, body)
	}

	if status != http.StatusOK {
		l.Error("merge failed", "status", status, "body", string(data))
		os.Exit(1)
	}
}

func shouldMerge(protected bool, requiredContexts []string, passingContexts []string) (bool, string) {
	if !protected {
		return false, "target branch is unprotected"
	}

	if len(requiredContexts) == 0 {
		return false, "target branch is protected but does not specify required checks"
	}

	for _, c := range requiredContexts {
		if !slices.Contains(passingContexts, c) {
			return false, fmt.Sprintf("required context '%s' is not satisfied", c)
		}
	}

	return true, "all requirements are met"
}

func doRequest(method string, url string, body io.Reader) ([]byte, int) {
	req, _ := http.NewRequest(method, url, body)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		l.Error("HTTP error", "error", err)
		os.Exit(1)
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			l.Error("error closing request body", "error", err)
		}
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		l.Error("error reading request body", "error", err)
		os.Exit(1)
	}

	return data, resp.StatusCode
}
