package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
	"os"
	"slices"
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
	mergeableRetryDelay = time.Second * 15
)

func main() {
	if apiType != "gitea" && apiType != "github" {
		l.Error("unsupported API type", "type", apiType)
		os.Exit(1)
	}

	prs, err := getPRs()
	if err != nil {
		l.Error("error getting PRs", "error", err)
		os.Exit(1)
	}

	anyFailed := false

PRLoop:
	for _, pr := range prs {
		l.Info("considering PR", "title", pr.Title)

		if !pr.hasLabel("automerge") {
			l.Info("skipping PR, no automerge label", "title", pr.Title)
			continue
		}

		// wait for it to become mergeable
		for range maxMergeableRetries {
			l.Info("waiting for PR to become mergeable")
			time.Sleep(mergeableRetryDelay)
			pr, err = getPR(pr.Number)
			if err != nil {
				l.Info("error loading this PR; will continue with others", "error", err)
				anyFailed = true
				continue PRLoop
			}

			if pr.Mergeable {
				break
			}
		}

		if !pr.Mergeable {
			l.Info("PR is not mergeable")
			continue PRLoop
		}

		branchProtected, requiredContexts, err := getBranchProtection(pr)
		if err != nil {
			l.Info("error checking branch protection; will continue with others", "error", err)
			anyFailed = true
			continue PRLoop
		}

		passingContexts, err := getPassingContexts(pr)
		if err != nil {
			l.Info("error getting passing contexts; will continue with others", "error", err)
			anyFailed = true
			continue PRLoop
		}

		doMerge, reason := shouldMerge(branchProtected, requiredContexts, passingContexts)
		if !doMerge {
			l.Info("PR cannot be merged", "reason", reason)
			continue
		}

		l.Info("attempting to merge PR...")
		err = mergePR(pr)
		if err != nil {
			l.Info("error merging PR; will continue with others", "error", err)
			anyFailed = true
			continue PRLoop
		}
		l.Info("merged")

		l.Info("deleting branch...")
		err = deleteBranch(pr)
		if err != nil {
			l.Info("error deleting branch; will continue with others", "error", err)
			anyFailed = true
			continue PRLoop
		}
		l.Info("deleted")

		// pause to make sure mergability of other PRs is re-evaluated by the platform
		time.Sleep(time.Second * 30)
	}

	if anyFailed {
		os.Exit(1)
	}
}

func getPRs() ([]PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open", apiBase, repoOwner, repoName)
	data, _, err := doRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting PR list: %w", err)
	}
	var prs []PullRequest
	err = json.Unmarshal(data, &prs)
	if err != nil {
		return nil, fmt.Errorf("error parsing PR list: %w", err)
	}
	return prs, nil
}

func getPR(number int) (PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", apiBase, repoOwner, repoName, number)
	data, _, err := doRequest("GET", url, nil)
	if err != nil {
		return PullRequest{}, fmt.Errorf("error getting PR: %w", err)
	}
	var pr PullRequest
	err = json.Unmarshal(data, &pr)
	if err != nil {
		return PullRequest{}, fmt.Errorf("error parsing PR: %w", err)
	}
	return pr, nil
}

func getBranchProtection(pr PullRequest) (bool, []string, error) {
	switch apiType {
	case "gitea":
		url := fmt.Sprintf("%s/repos/%s/%s/branch_protections/%s", apiBase, repoOwner, repoName, neturl.PathEscape(pr.Base.Ref))
		data, status, err := doRequest("GET", url, nil)
		if err != nil {
			return false, nil, fmt.Errorf("error getting branch protection: %w", err)
		}
		if status == http.StatusNotFound {
			return false, nil, nil
		}

		var bp GiteaBranchProtection
		err = json.Unmarshal(data, &bp)
		if err != nil {
			return false, nil, fmt.Errorf("error parsing branch protection: %w", err)
		}

		if len(bp.StatusCheckContexts) > 0 {
			return true, bp.StatusCheckContexts, nil
		}

	case "github":
		url := fmt.Sprintf("%s/repos/%s/%s/branches/%s/protection", apiBase, repoOwner, repoName, neturl.PathEscape(pr.Base.Ref))
		data, status, err := doRequest("GET", url, nil)
		if err != nil {
			return false, nil, fmt.Errorf("error getting branch protection: %w", err)
		}
		if status == http.StatusNotFound {
			return false, nil, nil
		}

		var bp GitHubBranchProtection
		err = json.Unmarshal(data, &bp)
		if err != nil {
			return false, nil, fmt.Errorf("error parsing branch protection: %w", err)
		}

		if bp.RequiredStatusChecks != nil {
			return true, bp.RequiredStatusChecks.Contexts, nil
		}
	}

	return false, nil, nil
}

func getPassingContexts(pr PullRequest) ([]string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s/status", apiBase, repoOwner, repoName, pr.Head.SHA)
	data, _, err := doRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting commit status: %w", err)
	}

	var combinedStatus CommitStatus
	err = json.Unmarshal(data, &combinedStatus)
	if err != nil {
		return nil, fmt.Errorf("error parsing commit status: %w", err)
	}

	passing := []string{}
	for _, s := range combinedStatus.Statuses {
		// gitea uses "status", github uses "state", so check both
		if s.Status == "success" || s.State == "success" {
			passing = append(passing, s.Context)
		}
	}

	return passing, nil
}

func mergePR(pr PullRequest) error {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/merge", apiBase, repoOwner, repoName, pr.Number)

	var body any
	switch apiType {
	case "gitea":
		body = GiteaMergeRequest{Method: "squash"}

	case "github":
		body = GitHubMergeRequest{Method: "squash"}
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("error marshalling merge request: %w", err)
	}

	var data []byte
	var status int

	switch apiType {
	case "gitea":
		data, status, err = doRequest("POST", url, bytes.NewReader(bodyBytes))

	case "github":
		data, status, err = doRequest("PUT", url, bytes.NewReader(bodyBytes))
	}

	if err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	if status != http.StatusOK {
		return fmt.Errorf("merge failed, status %d, message: %s", status, string(data))
	}

	return nil
}

func deleteBranch(pr PullRequest) error {
	var url string
	switch apiType {
	case "gitea":
		url = fmt.Sprintf("%s/repos/%s/%s/branches/%s", apiBase, repoOwner, repoName, neturl.PathEscape(pr.Head.Ref))

	case "github":
		url = fmt.Sprintf("%s/repos/%s/%s/git/refs/%s", apiBase, repoOwner, repoName, neturl.PathEscape(pr.Head.Ref))
	}

	data, status, err := doRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	if status < 200 || status > 299 {
		return fmt.Errorf("delete failed, status %d, message: %s", status, string(data))
	}

	return nil
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

func doRequest(method string, url string, body io.Reader) ([]byte, int, error) {
	req, _ := http.NewRequest(method, url, body)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
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
		return nil, 0, err
	}

	return data, resp.StatusCode, nil
}
