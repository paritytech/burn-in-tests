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
	"log"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"
	burnin "gitlab.example.com/burn-in-tests/backend"
)

var (
	runPattern             = regexp.MustCompile(`run-(kusama|polkadot)-(fullnode|sentry|validator)-\d-\d+`)
	commitMsgPrefixPattern = regexp.MustCompile(`\[deploy-(kusama|polkadot)-(fullnode|sentry|validator)].*`)
	mockPoller             = Poller{sleep: func(time.Duration) {}}
)

type processRequestTestCase struct {
	description             string
	expectedSyncFromScratch bool
	commitDiff              burnin.CommitDiff

	getPipelinesForBranch func(string) ([]burnin.Pipeline, error)
	getPipelineForCommit  func(string) (burnin.Pipeline, error)
	getPipeline           func(int) (burnin.Pipeline, error)
	getPipelineJobs       func(int) ([]burnin.Job, error)
	getJob                func(int) (burnin.Job, error)
	startJob              func(int) error
}

func Test_ProcessRequest_create_requests(t *testing.T) {
	// This is a bit ugly. Used to simulate a job status changing over multiple calls to burnin.Gitlab.GetJob().
	getJobCallCount := 0
	// same as getJobCallCount, but for burnin.Gitlab.StartJob()
	startJobCallCount := 0
	// same as getJobCallCount, but for burnin.Gitlab.GetPipelinesForBranch()
	getPipelinesForBranchCallCount := 0

	cases := []processRequestTestCase{
		{
			description:             "Request includes 'custom_binary' attribute",
			expectedSyncFromScratch: false,
			commitDiff:              mkCommitDiff("requests/request-1607684670.toml", true, false, false, ""),
		},
		{
			description:             "Request lacks 'custom_binary' attribute, 'build-linux-stable' job already ran successfully",
			expectedSyncFromScratch: true,
			commitDiff:              mkCommitDiff("requests/request-1602856340.toml", true, false, false, ""),

			getPipelinesForBranch: func(string) ([]burnin.Pipeline, error) {
				return []burnin.Pipeline{
					{
						ID:     1,
						Status: "running",
						Ref:    "2013",
						SHA:    "a7810560c0f62dd6d347e710a5e2a64da465c109",
						WebURL: "https://gitlab.example.com/mocks/mockproject/-/pipelines/1",
					},
				}, nil
			},

			getPipelineJobs: func(int) ([]burnin.Job, error) {
				return []burnin.Job{
					{
						ID:     752482,
						Name:   "build-linux-stable",
						Status: "success",
						WebURL: "https://gitlab.example.com/mocks/mockproject/-/jobs/752482",
					},
				}, nil
			},
		},
		{
			description:             "Request lacks 'custom_binary' attribute, pipeline for PR branch already exists but 'build-linux-stable' job did not run yet",
			expectedSyncFromScratch: true,
			commitDiff:              mkCommitDiff("requests/request-1602856340.toml", true, false, false, ""),

			getPipelinesForBranch: func(string) ([]burnin.Pipeline, error) {
				return []burnin.Pipeline{
					{
						ID:     1,
						Status: "running",
						Ref:    "2013",
						SHA:    "a7810560c0f62dd6d347e710a5e2a64da465c109",
						WebURL: "https://gitlab.example.com/mocks/mockproject/-/pipelines/1",
					},
				}, nil
			},

			getPipelineJobs: func(int) ([]burnin.Job, error) {
				return []burnin.Job{
					{
						ID:     752481,
						Name:   "test-linux-stable",
						Status: "success",
						WebURL: "https://gitlab.example.com/mocks/mockproject/-/jobs/752481",
					},
					{
						ID:     752482,
						Name:   "build-linux-stable",
						Status: "manual",
						WebURL: "https://gitlab.example.com/mocks/mockproject/-/jobs/752482",
					},
				}, nil
			},

			getJob: func(int) (burnin.Job, error) {
				status := "running"
				getJobCallCount += 1

				if getJobCallCount > 3 {
					status = "success"
				}

				if getJobCallCount > 4 {
					require.Fail(t, "getJob() was called after it returned a job that finished successfully")
				}

				return burnin.Job{
					ID:     752482,
					Name:   "build-linux-stable",
					Status: status,
					WebURL: "https://gitlab.example.com/mocks/mockproject/-/jobs/752482",
				}, nil
			},
		},
		{
			description:             "Request lacks 'custom_binary' attribute and pipeline for PR branch does not exist yet",
			expectedSyncFromScratch: true,
			commitDiff:              mkCommitDiff("requests/request-1602856340.toml", true, false, false, ""),

			getPipelinesForBranch: func(string) ([]burnin.Pipeline, error) {
				getPipelinesForBranchCallCount += 1

				if getPipelinesForBranchCallCount < 2 {
					return []burnin.Pipeline{}, nil
				}

				status := "pending"

				if getPipelinesForBranchCallCount > 2 {
					status = "running"
				}

				if getPipelinesForBranchCallCount > 3 {
					require.Fail(t, "getPipelinesForBranch() was called again after it returned a running pipeline")
				}

				return []burnin.Pipeline{
					{
						ID:     1,
						Status: status,
						Ref:    "2013",
						SHA:    "a7810560c0f62dd6d347e710a5e2a64da465c109",
						WebURL: "https://gitlab.example.com/mocks/mockproject/-/pipelines/1",
					},
				}, nil
			},

			getPipelineJobs: func(int) ([]burnin.Job, error) {
				if getPipelinesForBranchCallCount < 3 {
					require.Fail(t, "getPipelineJobs() should not have gotten called before getPipelinesForBranch() returns a running pipeline")
				}

				return []burnin.Job{
					{
						ID:     752481,
						Name:   "test-linux-stable",
						Status: "success",
						WebURL: "https://gitlab.example.com/mocks/mockproject/-/jobs/752481",
					},
					{
						ID:     752482,
						Name:   "build-linux-stable",
						Status: "created",
						WebURL: "https://gitlab.example.com/mocks/mockproject/-/jobs/752482",
					},
				}, nil
			},

			getJob: func(int) (burnin.Job, error) {
				getJobCallCount += 1

				status := "created"

				if getJobCallCount > 4 {
					status = "manual"
				}

				if getJobCallCount > 5 {
					require.Equal(t, 1, startJobCallCount)
					status = "running"
				}

				if getJobCallCount > 8 {
					status = "success"
				}

				if getJobCallCount > 9 {
					require.Fail(t, "getJob() was called after it returned a job that finished successfully")
				}

				return burnin.Job{
					ID:     752482,
					Name:   "build-linux-stable",
					Status: status,
					WebURL: "https://gitlab.example.com/mocks/mockproject/-/jobs/752482",
				}, nil
			},

			startJob: func(id int) error {
				require.Equal(t, 752482, id)
				startJobCallCount += 1
				if startJobCallCount > 1 {
					require.Fail(t, "startJob() was called twice")
				}
				return nil
			},
		},
		{
			description:             "Request includes 'commit_sha', referencing the second to last commit",
			expectedSyncFromScratch: false,
			commitDiff:              mkCommitDiff("requests/request-1609342845.toml", true, false, false, ""),

			getPipelineForCommit: func(sha string) (burnin.Pipeline, error) {
				require.Equal(t, "ec52cc79cc774f1b9b8960ea0fbdbc3ad51dc461", sha)

				return burnin.Pipeline{
					ID:     837459,
					Status: "success",
					Ref:    "2013",
					SHA:    "ec52cc79cc774f1b9b8960ea0fbdbc3ad51dc461",
					WebURL: "https://gitlab.example.com/mocks/mockproject/-/pipelines/837459",
				}, nil
			},

			getPipelineJobs: func(id int) ([]burnin.Job, error) {
				require.Equal(t, 837459, id)

				return []burnin.Job{
					{
						ID:     752482,
						Name:   "build-linux-stable",
						Status: "success",
						WebURL: "https://gitlab.example.com/mocks/mockproject/-/jobs/752482",
					},
				}, nil
			},
		},
	}

	for _, c := range cases {
		log.Println(c.description)
		burninGitlab, buildGitlab := newMockGitlabClientsForRequestCase(c)
		matrix := new(mockMatrix)
		getJobCallCount = 0
		startJobCallCount = 0
		getPipelinesForBranchCallCount = 0

		err := ProcessRequest("testdata", "master", burninGitlab, buildGitlab, mockPoller, matrix)

		require.NoError(t, err)

		require.True(t, len(burninGitlab.createFileCalls) > 0)
		for _, call := range burninGitlab.createFileCalls {
			require.True(t, strings.HasPrefix(call.path, "runs/"))
			require.True(t, runPattern.MatchString(call.path))
			require.True(t, call.branch == "master" || runPattern.MatchString(call.branch))
			require.True(t, commitMsgPrefixPattern.MatchString(call.commitMsg))

			var deployment burnin.Deployment
			err := toml.Unmarshal(call.content, &deployment)
			require.NoError(t, err)
			require.Equal(t, "https://github.com/paritytech/polkadot/pull/2013", deployment.PullRequest)
			require.NotNil(t, deployment.CustomBinary)
			require.Equal(t, "https://gitlab.example.com/mocks/mockproject/-/jobs/752482/artifacts/raw/artifacts/polkadot", deployment.CustomBinary)
			require.Equal(t, "mxinden", deployment.RequestedBy)
			require.Equal(t, c.expectedSyncFromScratch, deployment.SyncFromScratch)
			require.Empty(t, deployment.DeployedAt)
			require.Empty(t, deployment.DeployedOn)

			if c.description == "Request includes 'custom_binary' attribute" {
				require.Len(t, deployment.CustomOptions, 2)
				require.Equal(t, "--wasm-execution Compiled", deployment.CustomOptions[0])
				require.Equal(t, "--rpc-methods Unsafe", deployment.CustomOptions[1])
			}

			if c.description == "Request includes 'commit_sha', referencing the second to last commit" {
				require.Equal(t, "ec52cc79cc774f1b9b8960ea0fbdbc3ad51dc461", deployment.CommitSHA)
			} else if c.description != "Request includes 'custom_binary' attribute" {
				require.Equal(t, "a7810560c0f62dd6d347e710a5e2a64da465c109", deployment.CommitSHA)
			}

			if c.description == "Request lacks 'custom_binary' attribute and pipeline for PR branch does not exist yet" {
				require.Equal(t, 1, startJobCallCount)
			}
		}

		require.Len(t, matrix.requestNotificationCalls, 1)
		require.True(t, len(burninGitlab.createBranchCalls) == 0)
		require.True(t, len(burninGitlab.createMergeRequestCalls) == 0)
		require.Len(t, burninGitlab.deleteFileCalls, 0)
		require.Len(t, matrix.deploymentNotificationCalls, 0)
		require.Len(t, matrix.updateNotificationCalls, 0)
		require.Len(t, matrix.cleanupNotificationCalls, 0)
		require.Len(t, matrix.errorNotificationCalls, 0)
	}
}

func Test_ProcessRequest_update_requests(t *testing.T) {
	getPipelineForCommitCallCount := 0
	getJobCallCount := 0
	startJobCallCount := 0

	cases := []processRequestTestCase{
		{
			description: "Initial request did not include 'custom_binary', 'commit_sha' was updated",
			commitDiff:  mkCommitDiff("requests/request-1609842266.toml", false, false, false, "@@ -1,6 +1,6 @@\npull_request=\"https://github.com/paritytech/polkadot/pull/2013\"\n-commit_sha=\"a7810560c0f62dd6d347e710a5e2a64da465c109\"\n+commit_sha=\"6c7d5ffe7c9b88e1c8d3ffbea5f93f2387cca110\"\nrequested_by=\"mxinden\"\nsync_from_scratch=false\n[node_types]\n"),

			getPipelineForCommit: func(sha string) (burnin.Pipeline, error) {
				require.Equal(t, "6c7d5ffe7c9b88e1c8d3ffbea5f93f2387cca110", sha)

				return burnin.Pipeline{
					ID:     837459,
					Status: "success",
					Ref:    "2013",
					SHA:    "6c7d5ffe7c9b88e1c8d3ffbea5f93f2387cca110",
					WebURL: "https://gitlab.example.com/mocks/mockproject/-/pipelines/837459",
				}, nil
			},

			getPipelineJobs: func(id int) ([]burnin.Job, error) {
				require.Equal(t, 837459, id)

				return []burnin.Job{
					{
						ID:     760569,
						Name:   "build-linux-stable",
						Status: "success",
						WebURL: "https://gitlab.example.com/mocks/mockproject/-/jobs/760569",
					},
				}, nil
			},
		},
		{
			description: "'commit_sha' was updated, 'build-linux-stable' jobs needs to be run",
			commitDiff:  mkCommitDiff("requests/request-1609842266.toml", false, false, false, "@@ -1,6 +1,6 @@\npull_request=\"https://github.com/paritytech/polkadot/pull/2013\"\n-commit_sha=\"a7810560c0f62dd6d347e710a5e2a64da465c109\"\n+commit_sha=\"6c7d5ffe7c9b88e1c8d3ffbea5f93f2387cca110\"\nrequested_by=\"mxinden\"\nsync_from_scratch=false\n[node_types]\n"),

			getPipelineForCommit: func(sha string) (burnin.Pipeline, error) {
				require.Equal(t, "6c7d5ffe7c9b88e1c8d3ffbea5f93f2387cca110", sha)

				return burnin.Pipeline{
					ID:     837459,
					Status: "success",
					Ref:    "2013",
					SHA:    "6c7d5ffe7c9b88e1c8d3ffbea5f93f2387cca110",
					WebURL: "https://gitlab.example.com/mocks/mockproject/-/pipelines/837459",
				}, nil
			},

			getPipelineJobs: func(id int) ([]burnin.Job, error) {
				require.Equal(t, 837459, id)

				return []burnin.Job{
					{
						ID:     760569,
						Name:   "build-linux-stable",
						Status: "manual",
						WebURL: "https://gitlab.example.com/mocks/mockproject/-/jobs/760569",
					},
				}, nil
			},

			getJob: func(int) (burnin.Job, error) {
				status := "running"
				getJobCallCount += 1

				if getJobCallCount > 3 {
					status = "success"
				}

				if getJobCallCount > 4 {
					require.Fail(t, "getJob() was called after it returned a job that finished successfully")
				}

				return burnin.Job{
					ID:     760569,
					Name:   "build-linux-stable",
					Status: status,
					WebURL: "https://gitlab.example.com/mocks/mockproject/-/jobs/760569",
				}, nil
			},
		},
		{
			description: "'commit_sha' was updated, pipeline does not exist yet",
			commitDiff:  mkCommitDiff("requests/request-1609842266.toml", false, false, false, "@@ -1,6 +1,6 @@\npull_request=\"https://github.com/paritytech/polkadot/pull/2013\"\n-commit_sha=\"a7810560c0f62dd6d347e710a5e2a64da465c109\"\n+commit_sha=\"6c7d5ffe7c9b88e1c8d3ffbea5f93f2387cca110\"\nrequested_by=\"mxinden\"\nsync_from_scratch: false\n[node_types]\n"),

			getPipelinesForBranch: func(_ string) ([]burnin.Pipeline, error) {
				require.Fail(t, "getPipelinesForBranch() is not supposed to be called in this test case. expected getPipelineForCommit() to be called instead")
				return []burnin.Pipeline{}, nil
			},

			getPipelineForCommit: func(sha string) (burnin.Pipeline, error) {
				require.Equal(t, "6c7d5ffe7c9b88e1c8d3ffbea5f93f2387cca110", sha)

				getPipelineForCommitCallCount += 1

				if getPipelineForCommitCallCount < 3 {
					return burnin.Pipeline{}, burnin.ErrPipelineNotFound
				}

				if getPipelineForCommitCallCount > 4 {
					require.Fail(t, "getPipelineForCommit() was called after it already returned a running pipeline")
					return burnin.Pipeline{}, nil
				}

				// HACK Mutating getJobCallCount here is a terrible footgun!
				// This is done to have the "build-linux-stable" job be started and polled as well in this test
				// case, after getPipelineForCommit() was called a few times and returns a running pipeline.
				getJobCallCount = 0

				return burnin.Pipeline{
					ID:     837459,
					Status: "running",
					Ref:    "2013",
					SHA:    "6c7d5ffe7c9b88e1c8d3ffbea5f93f2387cca110",
					WebURL: "https://gitlab.example.com/mocks/mockproject/-/pipelines/837459",
				}, nil
			},

			getPipelineJobs: func(id int) ([]burnin.Job, error) {
				if getPipelineForCommitCallCount < 3 {
					require.Fail(t, "getPipelineJobs() was called before getPipelineForCommit() returned a running pipeline")
					return []burnin.Job{}, nil
				}

				require.Equal(t, 837459, id)

				return []burnin.Job{
					{
						ID:     760569,
						Name:   "build-linux-stable",
						Status: "created",
						WebURL: "https://gitlab.example.com/mocks/mockproject/-/jobs/760569",
					},
				}, nil
			},

			startJob: func(id int) error {
				require.Equal(t, 760569, id)
				startJobCallCount += 1
				if startJobCallCount > 1 {
					require.Fail(t, "startJob() was called twice")
				}
				return nil
			},

			getJob: func(int) (burnin.Job, error) {
				status := "created"
				getJobCallCount += 1

				if getJobCallCount > 3 {
					status = "manual"
				}

				if getJobCallCount > 5 {
					require.Equal(t, 1, startJobCallCount)
					status = "running"
				}

				if getJobCallCount > 9 {
					status = "success"
				}

				if getJobCallCount > 10 {
					require.Fail(t, "getJob() was called after it returned a job that finished successfully")
				}

				return burnin.Job{
					ID:     760569,
					Name:   "build-linux-stable",
					Status: status,
					WebURL: "https://gitlab.example.com/mocks/mockproject/-/jobs/760569",
				}, nil
			},
		},
		{
			description: "no 'commit_sha', 'custom_binary' was updated",
			commitDiff:  mkCommitDiff("requests/request-1610465640.toml", false, false, false, `@@ -1,6 +1,6 @@\npull_request="https://github.com/paritytech/polkadot/pull/2013"\n-custom_binary="https://gitlab.example.com/mocks/mockproject/-/jobs/752482/artifacts/raw/artifacts/polkadot"\n+custom_binary="https://gitlab.example.com/mocks/mockproject/-/jobs/760569/artifacts/raw/artifacts/polkadot"\nrequested_by="mxinden"\nsync_from_scratch=false\n[node_types]\n`),

			getPipelinesForBranch: func(_ string) ([]burnin.Pipeline, error) {
				require.Fail(t, "getPipelinesForBranch() is not supposed to be called in this test case")
				return []burnin.Pipeline{}, nil
			},

			getPipelineForCommit: func(_ string) (burnin.Pipeline, error) {
				require.Fail(t, "getPipelineForCommit() is not supposed to be called in this test case")
				return burnin.Pipeline{}, nil
			},

			getPipelineJobs: func(_ int) ([]burnin.Job, error) {
				require.Fail(t, "getPipelineJobs() is not supposed to be called in this test case")
				return []burnin.Job{}, nil
			},

			startJob: func(_ int) error {
				require.Fail(t, "startJob() is not supposed to be called in this test case")
				return nil
			},

			getJob: func(int) (burnin.Job, error) {
				require.Fail(t, "getJob() is not supposed to be called in this test case")
				return burnin.Job{}, nil
			},
		},
		{
			description: "'custom_binary' was updated, 'commit_sha' is removed",
			commitDiff:  mkCommitDiff("requests/request-1610469139.toml", false, false, false, `@@ -1,6 +1,6 @@\npull_request="https://github.com/paritytech/polkadot/pull/2013"\n-custom_binary="https://gitlab.example.com/mocks/mockproject/-/jobs/752482/artifacts/raw/artifacts/polkadot"\n+custom_binary="https://gitlab.example.com/mocks/mockproject/-/jobs/760569/artifacts/raw/artifacts/polkadot"\nrequested_by="mxinden"\nsync_from_scratch=false\n[node_types]\n`),

			getPipelinesForBranch: func(_ string) ([]burnin.Pipeline, error) {
				require.Fail(t, "getPipelinesForBranch() is not supposed to be called in this test case")
				return []burnin.Pipeline{}, nil
			},

			getPipelineForCommit: func(_ string) (burnin.Pipeline, error) {
				require.Fail(t, "getPipelineForCommit() is not supposed to be called in this test case")
				return burnin.Pipeline{}, nil
			},

			getPipelineJobs: func(_ int) ([]burnin.Job, error) {
				require.Fail(t, "getPipelineJobs() is not supposed to be called in this test case")
				return []burnin.Job{}, nil
			},

			startJob: func(_ int) error {
				require.Fail(t, "startJob() is not supposed to be called in this test case")
				return nil
			},

			getJob: func(int) (burnin.Job, error) {
				require.Fail(t, "getJob() is not supposed to be called in this test case")
				return burnin.Job{}, nil
			},
		},
		{
			description: "'custom_binary' and 'commit_sha' were updated",
			commitDiff:  mkCommitDiff("requests/request-1610469388.toml", false, false, false, `@@ -1,6 +1,6 @@\npull_request="https://github.com/paritytech/polkadot/pull/2013"\n-commit_sha="a7810560c0f62dd6d347e710a5e2a64da465c109"\n+commit_sha="6c7d5ffe7c9b88e1c8d3ffbea5f93f2387cca110"\n-custom_binary="https://gitlab.example.com/mocks/mockproject/-/jobs/752482/artifacts/raw/artifacts/polkadot"\n+custom_binary="https://gitlab.example.com/mocks/mockproject/-/jobs/760569/artifacts/raw/artifacts/polkadot"\nrequested_by="mxinden"\nsync_from_scratch=false\n[node_types]\n`),

			getPipelinesForBranch: func(_ string) ([]burnin.Pipeline, error) {
				require.Fail(t, "getPipelinesForBranch() is not supposed to be called in this test case")
				return []burnin.Pipeline{}, nil
			},

			getPipelineForCommit: func(_ string) (burnin.Pipeline, error) {
				require.Fail(t, "getPipelineForCommit() is not supposed to be called in this test case")
				return burnin.Pipeline{}, nil
			},

			getPipelineJobs: func(_ int) ([]burnin.Job, error) {
				require.Fail(t, "getPipelineJobs() is not supposed to be called in this test case")
				return []burnin.Job{}, nil
			},

			startJob: func(_ int) error {
				require.Fail(t, "startJob() is not supposed to be called in this test case")
				return nil
			},

			getJob: func(int) (burnin.Job, error) {
				require.Fail(t, "getJob() is not supposed to be called in this test case")
				return burnin.Job{}, nil
			},
		},
	}

	for _, c := range cases {
		log.Println(c.description)

		burninGitlab, buildGitlab := newMockGitlabClientsForRequestCase(c)
		matrix := new(mockMatrix)
		getJobCallCount = 0
		startJobCallCount = 0
		getPipelineForCommitCallCount = 0

		err := ProcessRequest("testdata", "master", burninGitlab, buildGitlab, mockPoller, matrix)

		require.NoError(t, err, c.description)
		require.Equal(t, 0, len(burninGitlab.createBranchCalls))
		require.Len(t, burninGitlab.createFileCalls, 0)
		require.Len(t, burninGitlab.deleteFileCalls, 0)

		require.True(t, len(burninGitlab.updateFileCalls) > 0)
		for _, call := range burninGitlab.updateFileCalls {
			require.Equal(t, "master", call.branch)
			require.True(t, strings.HasPrefix(call.path, "runs/"))
			require.True(t, runPattern.MatchString(call.path))
			require.True(t, strings.HasPrefix(call.commitMsg, "[update-deployment] "))

			var deployment burnin.Deployment
			err := toml.Unmarshal(call.content, &deployment)
			require.NoError(t, err, c.description)
			require.Equal(t, "https://github.com/paritytech/polkadot/pull/2013", deployment.PullRequest)

			if strings.Contains(c.description, "no 'commit_sha'") || strings.Contains(c.description, "'commit_sha' is removed") {
				require.Equal(t, "", deployment.CommitSHA)
			} else {
				require.Equal(t, "6c7d5ffe7c9b88e1c8d3ffbea5f93f2387cca110", deployment.CommitSHA)
			}

			require.NotNil(t, deployment.CustomBinary)
			require.Equal(t, "https://gitlab.example.com/mocks/mockproject/-/jobs/760569/artifacts/raw/artifacts/polkadot", deployment.CustomBinary)
			require.NotNil(t, deployment.DeployedAt)
			require.NotNil(t, deployment.UpdatedAt)
			require.NotNil(t, deployment.DeployedOn)
		}

		require.Len(t, burninGitlab.createMergeRequestCalls, 0)

		require.Len(t, matrix.requestNotificationCalls, 0) // request updates do not trigger notifications
		require.Len(t, matrix.deploymentNotificationCalls, 0)
		require.Len(t, matrix.updateNotificationCalls, 0)
		require.Len(t, matrix.cleanupNotificationCalls, 0)
		require.Len(t, matrix.errorNotificationCalls, 0)
	}
}

func Test_validateRequest(t *testing.T) {
	cases := []struct {
		description    string
		input          []burnin.CommitDiff
		expectedOutput requestKind
	}{
		{
			"no commit diffs",
			[]burnin.CommitDiff{},
			invalidRequest,
		},
		{
			"more than one commit diff",
			[]burnin.CommitDiff{
				mkCommitDiff("requests/request-1602856340.toml", true, false, false, ""),
				mkCommitDiff("requests/request-1602856340.toml", true, false, false, ""),
			},
			invalidRequest,
		},
		{
			"invalid path",
			[]burnin.CommitDiff{
				mkCommitDiff("utils/cmd/run-job/main.go", true, false, false, ""),
			},
			invalidRequest,
		},
		{
			"renamed file",
			[]burnin.CommitDiff{
				mkCommitDiff("requests/request-1602856340.toml", false, true, false, ""),
			},
			invalidRequest,
		},
		{
			"deleted file",
			[]burnin.CommitDiff{
				mkCommitDiff("requests/request-1602856340.toml", false, false, true, ""),
			},
			invalidRequest,
		},
		{
			"valid new request",
			[]burnin.CommitDiff{
				mkCommitDiff("requests/request-1602856340.toml", true, false, false, ""),
			},
			newRequest,
		},
		{
			"valid new request with 'custom_binary'",
			[]burnin.CommitDiff{
				mkCommitDiff("requests/request-1607684670.toml", true, false, false, ""),
			},
			newRequest,
		},
		{
			"valid new request with 'commit_sha'",
			[]burnin.CommitDiff{
				mkCommitDiff("requests/request-1609342845.toml", true, false, false, ""),
			},
			newRequest,
		},
		{
			"valid updated request with updated 'commit_sha'",
			[]burnin.CommitDiff{
				mkCommitDiff("requests/request-1609842266.toml", false, false, false, `@@ -1,6 +1,6 @@\npull_request="https://github.com/paritytech/polkadot/pull/2013"\n-commit_sha="ec52cc79cc774f1b9b8960ea0fbdbc3ad51dc461"\n+commit_sha="6c7d5ffe7c9b88e1c8d3ffbea5f93f2387cca110"\nrequested_by="mxinden"\nsync_from_scratch=false\n[node_types]\n`),
			},
			updatedCommitSHA,
		},
		{
			"valid updated request with updated 'custom_binary'",
			[]burnin.CommitDiff{
				mkCommitDiff("requests/request-1609842266.toml", false, false, false, `@@ -1,6 +1,6 @@\npull_request="https://github.com/paritytech/polkadot/pull/2013"\n-custom_binary="https://gitlab.example.com/mocks/mockproject/-/jobs/752482/artifacts/raw/artifacts/polkadot"\n+custom_binary="https://gitlab.example.com/mocks/mockproject/-/jobs/760569/artifacts/raw/artifacts/polkadot"\nrequested_by="mxinden"\nsync_from_scratch: false\n[node_types]\n`),
			},
			updatedCustomBinary,
		},
		{
			"valid updated request with updated 'commit_sha' and 'custom_binary'",
			[]burnin.CommitDiff{
				mkCommitDiff("requests/request-1609842266.toml", false, false, false, `@@ -1,6 +1,6 @@\npull_request="https://github.com/paritytech/polkadot/pull/2013"\n-commit_sha="ec52cc79cc774f1b9b8960ea0fbdbc3ad51dc461"\n+commit_sha="6c7d5ffe7c9b88e1c8d3ffbea5f93f2387cca110"\n-custom_binary="https://gitlab.example.com/mocks/mockproject/-/jobs/752482/artifacts/raw/artifacts/polkadot'\n+custom_binary: 'https://gitlab.example.com/mocks/mockproject/-/jobs/760569/artifacts/raw/artifacts/polkadot"\nrequested_by="mxinden"\nsync_from_scratch=false\n[node_types]\n`),
			},
			updatedCommitSHAAndCustomBinary,
		},
		// Valid diff that caused a failed "request" job
		// (issue https://gitlab.example.com/burn-in-tests/backend/-/issues/8)
		{
			"valid updated request with added 'commit_sha'",
			[]burnin.CommitDiff{
				mkCommitDiff("requests/request-1613138434.toml", false, false, false, "@@ -1,6 +1,7 @@\n pull_request = \"https://github.com/paritytech/polkadot/pull/2426\"\n+commit_sha = \"87f3f19ea5478df136099e892ad3c91aae59aa05\"\n requested_by = \"haiko@example.com\"\n sync_from_scratch = false\n \n [nodes.kusama]\n-  fullnode = 1\n\\ No newline at end of file\n+  fullnode = 1\n"),
			},
			updatedCommitSHA,
		},
	}

	for _, c := range cases {
		actualOutput := validateRequest(c.input)
		require.Equal(t, c.expectedOutput, actualOutput, c.description)
	}
}

func Test_parseRequestID(t *testing.T) {
	cases := []struct {
		description    string
		input          string
		errorExpected  bool
		expectedOutput string
	}{
		{
			"missing request ID",
			"requests/request-.toml",
			true,
			"",
		},
		{
			"valid",
			"requests/request-1602856340.toml",
			false,
			"1602856340",
		},
	}

	for _, c := range cases {
		actualOutput, err := parseRequestID(c.input)
		if c.errorExpected {
			require.Error(t, err, c.description)
		} else {
			require.NoError(t, err)
			require.Equal(t, c.expectedOutput, actualOutput, c.description)
		}
	}
}

func Test_toml(t *testing.T) {

	s := `pull_request="https://github.com/paritytech/polkadot/pull/2013"
requested_by="mxinden"
sync_from_scratch = true

[nodes.kusama]
  fullnode = 0
  sentry = 0
  validator = 1

[nodes.polkadot]
  fullnode = 2
  sentry = 0
  validator = 0`

	var r burnin.Request
	err := toml.Unmarshal([]byte(s), &r)
	require.NoError(t, err)
	require.Equal(t, "mxinden", r.RequestedBy)
	require.Equal(t, 0, r.Nodes["kusama"][burnin.FullNode])
	require.Equal(t, 1, r.Nodes["kusama"][burnin.Validator])
	require.Equal(t, 2, r.Nodes["polkadot"][burnin.FullNode])
	require.Equal(t, 0, r.Nodes["polkadot"][burnin.Validator])
}

func Test_parseRequestFile(t *testing.T) {
	customBinary := "https://gitlab.example.com/mocks/mockproject/-/jobs/752482/artifacts/raw/artifacts/polkadot"

	cases := []struct {
		description    string
		input          string
		errorExpected  bool
		expectedOutput burnin.Request
	}{
		{
			"too many nodes",
			"testdata/requests/request-1602856340_too_many_nodes.toml",
			true,
			burnin.Request{},
		},
		{
			"invalid node type",
			"testdata/requests/request-1602856340_invalid_node_type.toml",
			true,
			burnin.Request{},
		},
		{
			"invalid custom binary URL",
			"testdata/requests/request-1607684670_invalid_custom_binary.toml",
			true,
			burnin.Request{},
		},
		{
			"invalid pull request URL",
			"testdata/requests/request-1607684670_invalid_pr_url.toml",
			true,
			burnin.Request{},
		},
		{
			"valid",
			"testdata/requests/request-1607684670.toml",
			false,
			burnin.Request{
				PullRequest:     "https://github.com/paritytech/polkadot/pull/2013",
				CustomBinary:    &customBinary,
				RequestedBy:     "mxinden",
				SyncFromScratch: true,
				Nodes: burnin.NodesPerNetworkMap{
					"kusama": map[burnin.NodeType]int{
						burnin.FullNode:  1,
						burnin.Sentry:    0,
						burnin.Validator: 0,
					},
				},
			},
		},
	}

	for _, c := range cases {
		actualOutput, err := parseRequestFile(c.input)
		if c.errorExpected {
			require.Error(t, err, c.description)
		} else {
			require.NoError(t, err)
			require.Equal(t, c.expectedOutput.PullRequest, actualOutput.PullRequest, c.description)
			require.Equal(t, c.expectedOutput.RequestedBy, actualOutput.RequestedBy, c.description)
			require.Equal(t, c.expectedOutput.Nodes["fullnode"], actualOutput.Nodes["fullnode"], c.description)
			require.Equal(t, c.expectedOutput.Nodes["sentry"], actualOutput.Nodes["sentry"], c.description)
			require.Equal(t, c.expectedOutput.Nodes["validator"], actualOutput.Nodes["validator"], c.description)
		}
	}
}

func newMockGitlabClientsForRequestCase(testCase processRequestTestCase) (*mockGitlabClient, *mockGitlabClient) {
	burninGitlab := &mockGitlabClient{
		getLastCommitDiffs: func(string) ([]burnin.CommitDiff, error) {
			return []burnin.CommitDiff{testCase.commitDiff}, nil
		},
	}

	buildGitlab := &mockGitlabClient{
		getPipelinesForBranch: testCase.getPipelinesForBranch,
		getPipelineForCommit:  testCase.getPipelineForCommit,
		getPipeline:           testCase.getPipeline,
		getPipelineJobs:       testCase.getPipelineJobs,
		getJob:                testCase.getJob,
		startJob:              testCase.startJob,
	}

	return burninGitlab, buildGitlab
}

func mkCommitDiff(newPath string, newFile, renamedFile, deletedFile bool, patch string) burnin.CommitDiff {
	mode := "0655"

	return burnin.CommitDiff{
		Diff:        patch,
		NewPath:     &newPath,
		OldPath:     &newPath,
		AMode:       &mode,
		BMode:       &mode,
		NewFile:     newFile,
		RenamedFile: renamedFile,
		DeletedFile: deletedFile,
	}
}
