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
package job

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	burnin "gitlab.example.com/burn-in-tests/backend"
)

type createBranchArgs struct {
	name       string
	fromBranch string
}

type commitFileArgs struct {
	path      string
	branch    string
	commitMsg string
	content   []byte
}

type createMergeRequestArgs struct {
	title        string
	sourceBranch string
	targetBranch string
}

type getLastCommitDiffsFn func(string) ([]burnin.CommitDiff, error)

type mockGitlabClient struct {
	createBranchCalls       []createBranchArgs
	createFileCalls         []commitFileArgs
	updateFileCalls         []commitFileArgs
	deleteFileCalls         []commitFileArgs
	createMergeRequestCalls []createMergeRequestArgs
	runners                 []burnin.Runner
	runnerTags              map[int][]string

	getLastCommitDiffs    getLastCommitDiffsFn
	getPipelinesForBranch func(string) ([]burnin.Pipeline, error)
	getPipelineForCommit  func(string) (burnin.Pipeline, error)
	getPipeline           func(int) (burnin.Pipeline, error)
	getPipelineJobs       func(int) ([]burnin.Job, error)
	getJob                func(int) (burnin.Job, error)
	startJob              func(int) error
}

func newMockGitlabClient(diff burnin.CommitDiff) *mockGitlabClient {
	return &mockGitlabClient{
		getLastCommitDiffs: func(string) ([]burnin.CommitDiff, error) {
			return []burnin.CommitDiff{diff}, nil
		},
	}
}

func (c *mockGitlabClient) GetLastCommitDiffs(branch string) ([]burnin.CommitDiff, error) {
	if c.getLastCommitDiffs != nil {
		return c.getLastCommitDiffs(branch)
	}
	return []burnin.CommitDiff{}, nil
}

func (c *mockGitlabClient) GetPipelinesForBranch(branch string) ([]burnin.Pipeline, error) {
	if c.getPipelinesForBranch != nil {
		return c.getPipelinesForBranch(branch)
	}

	return []burnin.Pipeline{}, nil
}

func (c *mockGitlabClient) GetPipeline(id int) (burnin.Pipeline, error) {
	if c.getPipeline != nil {
		return c.getPipeline(id)
	}

	return burnin.Pipeline{}, nil
}

func (c *mockGitlabClient) GetPipelineForCommit(sha string) (burnin.Pipeline, error) {
	if c.getPipelineForCommit != nil {
		return c.getPipelineForCommit(sha)
	}

	return burnin.Pipeline{}, nil
}

func (c *mockGitlabClient) GetPipelineJobs(id int) ([]burnin.Job, error) {
	if c.getPipelineJobs != nil {
		return c.getPipelineJobs(id)
	}

	return []burnin.Job{}, nil
}

func (c *mockGitlabClient) GetJob(id int) (burnin.Job, error) {
	if c.getJob != nil {
		return c.getJob(id)
	}

	return burnin.Job{}, nil
}

func (c *mockGitlabClient) StartJob(id int) error {
	if c.startJob != nil {
		return c.startJob(id)
	}

	return nil
}

func (c *mockGitlabClient) CreateBranch(name, fromBranch string) error {
	c.createBranchCalls = append(c.createBranchCalls, createBranchArgs{name, fromBranch})
	return nil
}

func (c *mockGitlabClient) ListDirectory(string, string) ([]burnin.FileInfo, error) {
	return []burnin.FileInfo{}, nil
}

func (c *mockGitlabClient) CreateFile(path, branch, commitMsg string, content []byte) error {
	c.createFileCalls = append(c.createFileCalls, commitFileArgs{
		path, branch, commitMsg, content,
	})
	return nil
}

func (c *mockGitlabClient) UpdateFile(path, branch, commitMsg string, content []byte) error {
	c.updateFileCalls = append(c.updateFileCalls, commitFileArgs{
		path, branch, commitMsg, content,
	})
	return nil
}

func (c *mockGitlabClient) DeleteFile(path, branch, commitMsg string) error {
	c.deleteFileCalls = append(c.deleteFileCalls, commitFileArgs{
		path: path, branch: branch, commitMsg: commitMsg,
	})
	return nil
}

func (c *mockGitlabClient) CreateMergeRequest(title, sourceBranch, targetBranch string) (burnin.MergeRequest, error) {
	c.createMergeRequestCalls = append(c.createMergeRequestCalls, createMergeRequestArgs{
		title, sourceBranch, targetBranch,
	})
	return burnin.MergeRequest{
		ID:        42,
		ProjectID: 23,
		WebURL:    "https://gitlab.example.com/mocks/mockproject/-/merge_requests/42",
	}, nil
}

func (c *mockGitlabClient) GetRunners() ([]burnin.Runner, error) {
	return c.runners, nil
}

func (c *mockGitlabClient) GetRunnerTags(id int) ([]string, error) {
	tags, exists := c.runnerTags[id]
	if !exists {
		return nil, errors.New("runner not found")
	}

	return tags, nil
}

func (c *mockGitlabClient) PauseRunner(string) error {
	return nil
}

func (c *mockGitlabClient) UnPauseRunner(string) error {
	return nil
}

func (c *mockGitlabClient) WebURLForBranch(branch string) (*url.URL, error) {
	return url.Parse("https://gitlab.example.com/mocks/mockproject/-/tree/" + branch)
}

func (c *mockGitlabClient) WebURLForJob(id int) (*url.URL, error) {
	return url.Parse("https://gitlab.example.com/mocks/mockproject/-/jobs/" + strconv.Itoa(id))
}

func (c *mockGitlabClient) PrefixSkipCI(s string) string {
	return fmt.Sprintf("[skip ci] %s", s)
}

func (c *mockGitlabClient) PrefixDeploy(nw string, nt burnin.NodeType, s string) string {
	return fmt.Sprintf("[deploy-%s-%v] %s", nw, nt, s)
}

func (c *mockGitlabClient) PrefixUpdateDeployment(s string) string {
	return fmt.Sprintf("[update-deployment] %s", s)
}

func (c *mockGitlabClient) PrefixCleanup(s string) string {
	return fmt.Sprintf("[cleanup] %s", s)
}

type createSilenceArgs struct {
	matchers  []burnin.AlertMatcher
	startsAt  time.Time
	endsAt    time.Time
	createdBy string
	comment   string
}

func (a *createSilenceArgs) GetMatcher(name string) *burnin.AlertMatcher {
	for _, m := range a.matchers {
		if m.Name == name {
			return &m
		}
	}
	return nil
}

type mockAlertManager struct {
	createSilenceCalls []createSilenceArgs
}

func (a *mockAlertManager) CreateSilence(
	matchers []burnin.AlertMatcher,
	startsAt,
	endsAt time.Time,
	createdBy,
	comment string,
) (string, error) {
	a.createSilenceCalls = append(a.createSilenceCalls, createSilenceArgs{
		matchers, startsAt, endsAt, createdBy, comment,
	})
	return "alert-1234", nil
}

func (a *mockAlertManager) DeleteSilence(string) error {
	return nil
}

type runPlaybookArgs struct {
	name           string
	runOn          string
	nodeBinary     *url.URL
	wipeChainDB    bool
	nodePublicName string
	customOptions  []string
}

type mockAnsibleDriver struct {
	runPlaybookCalls []runPlaybookArgs
}

func (d *mockAnsibleDriver) RunPlaybook(
	name string,
	runOn string,
	nodeBinary *url.URL,
	wipeChainDB bool,
	nodePublicName string,
	customOptions []string,
) error {
	d.runPlaybookCalls = append(
		d.runPlaybookCalls,
		runPlaybookArgs{
			name,
			runOn,
			nodeBinary,
			wipeChainDB,
			nodePublicName,
			customOptions,
		})
	return nil
}

type mockMatrix struct {
	requestNotificationCalls    []burnin.Request
	deploymentNotificationCalls []burnin.Deployment
	updateNotificationCalls     []burnin.Deployment
	cleanupNotificationCalls    []burnin.Deployment
	errorNotificationCalls      []error
}

func (c *mockMatrix) SendRequestNotification(request burnin.Request) error {
	c.requestNotificationCalls = append(c.requestNotificationCalls, request)
	return nil
}

func (c *mockMatrix) SendDeploymentNotification(deployment burnin.Deployment) error {
	c.deploymentNotificationCalls = append(c.deploymentNotificationCalls, deployment)
	return nil
}

func (c *mockMatrix) SendUpdateNotification(deployment burnin.Deployment) error {
	c.updateNotificationCalls = append(c.updateNotificationCalls, deployment)
	return nil
}

func (c *mockMatrix) SendCleanupNotification(deployment burnin.Deployment) error {
	c.cleanupNotificationCalls = append(c.cleanupNotificationCalls, deployment)
	return nil
}

func (c *mockMatrix) SendErrorNotification(err error) error {
	c.errorNotificationCalls = append(c.errorNotificationCalls, err)
	return nil
}
