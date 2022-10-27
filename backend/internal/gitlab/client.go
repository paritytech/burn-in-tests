// Copyright (C) 2022 Parity Technologies (UK) Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later WITH Classpath-exception-2.0

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.
package gitlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	burnin "gitlab.example.com/burn-in-tests/backend"
	"gitlab.example.com/burn-in-tests/backend/internal/job"
)

const (
	CommitAuthorName  = "Burn-in Automator"
	CommitAuthorEmail = "bot@example.com"
)

type Client struct {
	serverURL   *url.URL
	projectURL  *url.URL // API URL to the project (e.g. https://gitlab.example.com/api/v4/project/42)
	project     Project
	accessToken string
	httpClient  *http.Client
}

type Project struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	PathWithNamespace string `json:"path_with_namespace"`
}

func NewClient(serverURL *url.URL, projectID int, accessToken string) (*Client, error) {
	projectURL, err := url.Parse(fmt.Sprintf("api/v4/projects/%d", projectID))
	if err != nil {
		return nil, err
	}

	return &Client{
		serverURL:   serverURL,
		projectURL:  serverURL.ResolveReference(projectURL),
		project:     Project{ID: projectID},
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (c *Client) Authenticate() error {
	request, err := http.NewRequest(http.MethodGet, c.projectURL.String(), nil)
	if err != nil {
		return err
	}

	request.Header.Set("PRIVATE-TOKEN", c.accessToken)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	err = errorIfNot(http.StatusOK, request, nil, response, false)
	if err != nil {
		return err
	}

	return json.NewDecoder(response.Body).Decode(&c.project)
}

func (c *Client) GetLastCommitDiffs(branch string) ([]burnin.CommitDiff, error) {
	var diffs []burnin.CommitDiff
	u, err := c.addPathsToProjectURL("repository/commits", branch, "diff")
	if err != nil {
		return diffs, err
	}

	request, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return diffs, err
	}

	request.Header.Set("PRIVATE-TOKEN", c.accessToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return diffs, err
	}
	defer response.Body.Close()

	err = errorIfNot(http.StatusOK, request, nil, response, false)
	if err != nil {
		return diffs, err
	}

	err = json.NewDecoder(response.Body).Decode(&diffs)
	if err != nil {
		return diffs, err
	}

	if len(diffs) < 1 {
		return diffs, fmt.Errorf("GitLab API returned no diffs for branch '%s'", branch)
	}

	return diffs, nil
}

// GetPipelinesForBranch does not return an error when the API returns an empty list of pipelines
func (c *Client) GetPipelinesForBranch(branch string) ([]burnin.Pipeline, error) {
	var pipelines []burnin.Pipeline
	u, err := c.addPathsToProjectURL("pipelines")
	if err != nil {
		return pipelines, err
	}

	q := u.Query()
	q.Set("ref", branch)
	q.Set("order_by", "updated_at")
	u.RawQuery = q.Encode()

	request, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return pipelines, err
	}

	request.Header.Set("PRIVATE-TOKEN", c.accessToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return pipelines, err
	}
	defer response.Body.Close()

	err = errorIfNot(http.StatusOK, request, nil, response, false)
	if err != nil {
		return pipelines, err
	}

	err = json.NewDecoder(response.Body).Decode(&pipelines)
	if err != nil {
		return pipelines, err
	}

	return pipelines, nil
}

// GetPipelineForCommit returns ErrPipelineNotFound when the API returns an empty list of pipelines or the list does not
// contain a pipeline for the given commit SHA (yet).
func (c *Client) GetPipelineForCommit(sha string) (burnin.Pipeline, error) {
	var pipelines []burnin.Pipeline
	u, err := c.addPathsToProjectURL("pipelines")
	if err != nil {
		return burnin.Pipeline{}, err
	}

	q := u.Query()
	q.Set("sha", sha)
	q.Set("order_by", "updated_at")
	u.RawQuery = q.Encode()

	request, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return burnin.Pipeline{}, err
	}

	request.Header.Set("PRIVATE-TOKEN", c.accessToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return burnin.Pipeline{}, err
	}
	defer response.Body.Close()

	err = errorIfNot(http.StatusOK, request, nil, response, false)
	if err != nil {
		return burnin.Pipeline{}, err
	}

	err = json.NewDecoder(response.Body).Decode(&pipelines)
	if err != nil {
		return burnin.Pipeline{}, err
	}

	if len(pipelines) < 1 {
		return burnin.Pipeline{}, burnin.ErrPipelineNotFound
	}

	return pipelines[0], nil
}

func (c *Client) GetPipeline(pipelineID int) (burnin.Pipeline, error) {
	var pipeline burnin.Pipeline
	u, err := c.addPathsToProjectURL("pipelines", strconv.Itoa(pipelineID))
	if err != nil {
		return pipeline, err
	}

	request, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return pipeline, err
	}

	request.Header.Set("PRIVATE-TOKEN", c.accessToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return pipeline, err
	}
	defer response.Body.Close()

	err = errorIfNot(http.StatusOK, request, nil, response, false)
	if err != nil {
		return pipeline, err
	}

	err = json.NewDecoder(response.Body).Decode(&pipeline)
	if err != nil {
		return pipeline, err
	}

	return pipeline, nil
}

func (c *Client) GetPipelineJobs(pipelineID int) ([]burnin.Job, error) {
	var jobs []burnin.Job
	u, err := c.addPathsToProjectURL("pipelines", strconv.Itoa(pipelineID), "jobs")
	if err != nil {
		return jobs, err
	}

	q := u.Query()
	q.Set("per_page", "100")
	u.RawQuery = q.Encode()

	request, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return jobs, err
	}

	request.Header.Set("PRIVATE-TOKEN", c.accessToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return jobs, err
	}
	defer response.Body.Close()

	err = errorIfNot(http.StatusOK, request, nil, response, false)
	if err != nil {
		return jobs, err
	}

	err = json.NewDecoder(response.Body).Decode(&jobs)
	if err != nil {
		return jobs, err
	}

	if len(jobs) < 1 {
		return jobs, fmt.Errorf("GitLab API returned no jobs for pipeline '%d'", pipelineID)
	}

	return jobs, nil
}

func (c *Client) GetJob(jobID int) (burnin.Job, error) {
	var glJob burnin.Job
	u, err := c.addPathsToProjectURL("jobs", strconv.Itoa(jobID))
	if err != nil {
		return glJob, err
	}

	request, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return glJob, err
	}

	request.Header.Set("PRIVATE-TOKEN", c.accessToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return glJob, err
	}
	defer response.Body.Close()

	err = errorIfNot(http.StatusOK, request, nil, response, false)
	if err != nil {
		return glJob, err
	}

	err = json.NewDecoder(response.Body).Decode(&glJob)
	if err != nil {
		return glJob, err
	}

	return glJob, nil
}

func (c *Client) StartJob(jobID int) error {
	u, err := c.addPathsToProjectURL("jobs", strconv.Itoa(jobID), "play")
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, u.String(), nil)
	if err != nil {
		return err
	}

	request.Header.Set("PRIVATE-TOKEN", c.accessToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}

	return errorIfNot(http.StatusOK, request, nil, response, true)
}

func (c *Client) CreateBranch(name, fromBranch string) error {
	u, err := c.addPathsToProjectURL("repository/branches")
	if err != nil {
		return err
	}

	q := u.Query()
	q.Set("branch", name)
	q.Set("ref", fromBranch)
	u.RawQuery = q.Encode()

	request, err := http.NewRequest(http.MethodPost, u.String(), nil)
	if err != nil {
		return err
	}

	request.Header.Set("PRIVATE-TOKEN", c.accessToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}

	return errorIfNot(http.StatusCreated, request, nil, response, true)
}

func (c *Client) ListDirectory(path, branch string) ([]burnin.FileInfo, error) {
	var items []burnin.FileInfo
	u, err := c.addPathsToProjectURL("repository/tree")
	if err != nil {
		return items, err
	}

	q := u.Query()
	q.Set("path", path)
	q.Set("ref", branch)
	q.Set("recursive", "false")
	q.Set("per_page", "1000")
	u.RawQuery = q.Encode()

	request, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return items, err
	}

	request.Header.Set("PRIVATE-TOKEN", c.accessToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return items, err
	}
	defer response.Body.Close()

	err = errorIfNot(http.StatusOK, request, nil, response, false)
	if err != nil {
		return items, err
	}

	err = json.NewDecoder(response.Body).Decode(&items)
	if err != nil {
		return items, err
	}

	return items, nil
}

// CreateFile commits a new file to the given branch. The commit message is prepended with the prefix defined in the
// constant SkipCI, if the parameter skipCI is set to true. This is useful to avoid triggering CI jobs from commits
// added within CI jobs.
func (c *Client) CreateFile(path, branch, commitMsg string, content []byte) error {
	return c.commitFile(CREATE, path, branch, commitMsg, string(content))
}

// UpdateFile changes an existing file on the given branch. The commit message is prepended with the prefix defined in
// the constant SkipCI, if the parameter skipCI is set to true. This is useful to avoid triggering CI jobs from commits
// added within CI jobs.
func (c *Client) UpdateFile(path, branch, commitMsg string, content []byte) error {
	return c.commitFile(UPDATE, path, branch, commitMsg, string(content))
}

// DeleteFile removes a file from the given branch. The commit message is prepended with the prefix defined in the
// constant SkipCI, if the parameter skipCI is set to true. to avoid triggering CI jobs from commits added within CI
// jobs.
func (c *Client) DeleteFile(path, branch, commitMsg string) error {
	return c.commitFile(DELETE, path, branch, commitMsg, "")
}

func (c *Client) CreateMergeRequest(title, sourceBranch, targetBranch string) (burnin.MergeRequest, error) {
	var mr burnin.MergeRequest
	u, err := c.addPathsToProjectURL("merge_requests")
	if err != nil {
		return mr, err
	}

	payload := struct {
		ID                 int    `json:"id"`
		SourceBranch       string `json:"source_branch"`
		TargetBranch       string `json:"target_branch"`
		Title              string `json:"title"`
		RemoveSourceBranch bool   `json:"remove_source_branch"`
		AllowCollaboration bool   `json:"allow_collaboration"`
		Squash             bool   `json:"squash"`
	}{
		ID:                 c.project.ID,
		SourceBranch:       sourceBranch,
		TargetBranch:       targetBranch,
		Title:              title,
		RemoveSourceBranch: true,
		AllowCollaboration: true,
		Squash:             true,
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		return mr, err
	}

	request, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(buf))
	if err != nil {
		return mr, err
	}

	request.Header.Set("PRIVATE-TOKEN", c.accessToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return mr, err
	}

	defer response.Body.Close()
	err = errorIfNot(http.StatusCreated, request, buf, response, false)
	if err != nil {
		return mr, err
	}

	err = json.NewDecoder(response.Body).Decode(&mr)
	if err != nil {
		return mr, err
	}

	return mr, nil
}

func (c *Client) PrefixSkipCI(s string) string {
	return fmt.Sprintf("[skip ci] %s", s)
}

func (c *Client) PrefixDeploy(nw string, nt burnin.NodeType, s string) string {
	return fmt.Sprintf("[deploy-%s-%v] %s", nw, nt, s)
}

func (c *Client) PrefixUpdateDeployment(s string) string {
	return fmt.Sprintf("[update-deployment] %s", s)
}

func (c *Client) PrefixCleanup(s string) string {
	return fmt.Sprintf("[cleanup] %s", s)
}

func (c *Client) WebURLForBranch(branch string) (*url.URL, error) {
	return job.AddPathsToURL(c.serverURL, c.project.PathWithNamespace, "-/tree", branch)
}

func (c *Client) WebURLForJob(id int) (*url.URL, error) {
	return job.AddPathsToURL(c.serverURL, c.project.PathWithNamespace, "-/jobs", strconv.Itoa(id))
}

func (c *Client) GetRunners() ([]burnin.Runner, error) {
	runners := make([]burnin.Runner, 0)
	var totalPages int64 = 0
	var currentPage int64 = 1
	keepGoing := true

	for keepGoing {
		u, err := c.addPathsToAPIURL("runners/all")
		if err != nil {
			return runners, err
		}

		q := url.Values{}
		q.Set("page", strconv.FormatInt(currentPage, 10))
		request, err := http.NewRequest(http.MethodGet, u.String()+"?"+q.Encode(), nil)
		if err != nil {
			return runners, err
		}

		request.Header.Set("PRIVATE-TOKEN", c.accessToken)
		response, err := c.httpClient.Do(request)
		if err != nil {
			return runners, err
		}

		err = errorIfNot(http.StatusOK, request, nil, response, false)
		if err != nil {
			_ = response.Body.Close()
			return runners, err
		}

		var r []burnin.Runner
		err = json.NewDecoder(response.Body).Decode(&r)
		if err != nil {
			_ = response.Body.Close()
			return runners, err
		}

		_ = response.Body.Close()
		runners = append(runners, r...)

		if totalPages == 0 {
			if response.Header.Get("x-total-pages") != "" {
				totalPages, err = strconv.ParseInt(response.Header.Get("x-total-pages"), 0, 0)
				if err != nil {
					return runners, err
				}
			} else {
				totalPages = 1
			}
		} else {
			if currentPage < totalPages {
				currentPage += 1
			} else {
				keepGoing = false
			}
		}
	}

	return runners, nil
}

func (c *Client) GetRunnerTags(id int) ([]string, error) {
	u, err := c.addPathsToAPIURL("runners", strconv.Itoa(id))
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("PRIVATE-TOKEN", c.accessToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	err = errorIfNot(http.StatusOK, request, nil, response, false)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Tags []string `json:"tag_list"`
	}

	err = json.NewDecoder(response.Body).Decode(&payload)
	return payload.Tags, err
}

// PauseRunner relies on the convention that the "description" fields as returned by GET /api/v4/runners/all contains
// the passed in hostname (and nothing else) in order to determine the Runner ID, which is required for pausing the
// runner.
func (c *Client) PauseRunner(hostname string) error {
	return c.setRunnerActiveFlag(hostname, false)
}

// UnPauseRunner relies on the convention that the "description" fields as returned by GET /api/v4/runners/all contains
// the passed in hostname (and nothing else) in order to determine the Runner ID, which is required for unpausing the
// runner.
func (c *Client) UnPauseRunner(hostname string) error {
	return c.setRunnerActiveFlag(hostname, true)
}

func (c *Client) setRunnerActiveFlag(hostname string, active bool) error {
	runners, err := c.GetRunners()
	if err != nil {
		return err
	}

	runnerID := -1
	for _, runner := range runners {
		if runner.Description == hostname {
			runnerID = runner.ID
			break
		}
	}

	if runnerID == -1 {
		return fmt.Errorf("could not find runner ID for hostname %s", hostname)
	}

	u, err := c.addPathsToAPIURL("runners", strconv.Itoa(runnerID))
	if err != nil {
		return err
	}

	requestBody := fmt.Sprintf(`{"active": %v}`, active)
	request, err := http.NewRequest(http.MethodPut, u.String(), strings.NewReader(requestBody))
	if err != nil {
		return err
	}
	request.Header.Set("PRIVATE-TOKEN", c.accessToken)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	return errorIfNot(http.StatusOK, request, []byte(requestBody), response, true)
}

type commitAction string

const (
	CREATE commitAction = "create"
	UPDATE              = "update"
	DELETE              = "delete"
)

func (c *Client) commitFile(action commitAction, path, branch, commitMsg, content string) error {
	u, err := c.addPathsToProjectURL("repository/commits")
	if err != nil {
		return err
	}

	payload := struct {
		Branch        string      `json:"branch"`
		CommitMessage string      `json:"commit_message"`
		AuthorName    string      `json:"author_name"`
		AuthorEmail   string      `json:"author_email"`
		Actions       interface{} `json:"actions"`
	}{
		Branch:        branch,
		CommitMessage: commitMsg,
		AuthorName:    CommitAuthorName,
		AuthorEmail:   CommitAuthorEmail,
		Actions: []struct {
			Action   commitAction `json:"action"`
			FilePath string       `json:"file_path"`
			Content  string       `json:"content"`
		}{
			{
				Action:   action,
				FilePath: path,
				Content:  content,
			},
		},
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(buf))
	if err != nil {
		return err
	}

	request.Header.Set("PRIVATE-TOKEN", c.accessToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}

	return errorIfNot(http.StatusCreated, request, buf, response, true)
}

func (c *Client) addPathsToAPIURL(paths ...string) (*url.URL, error) {
	p := []string{"api", "v4"}
	p = append(p, paths...)
	return job.AddPathsToURL(c.serverURL, p...)
}

func (c *Client) addPathsToProjectURL(paths ...string) (*url.URL, error) {
	return job.AddPathsToURL(c.projectURL, paths...)
}

func errorIfNot(status int, request *http.Request, payload []byte, response *http.Response, closeResponse bool) error {
	if closeResponse {
		defer response.Body.Close()
	}

	if response.StatusCode == status {
		return nil
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	return fmt.Errorf(`HTTP request to GitLab API failed.
Request: %s %s
Body: %s

Response: %s
Body: %s`, request.Method, request.URL.String(), payload, response.Status, string(responseBody))
}
