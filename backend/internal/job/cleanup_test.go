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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	burnin "gitlab.example.com/burn-in-tests/backend"
)

func Test_ProcessCleanup(t *testing.T) {
	diff := mkCommitDiff("runs/run-kusama-fullnode-0-1602856341.toml", false, false, true, "@@ -1,8 +0,0 @@\n-pull_request=\"https://github.com/paritytech/polkadot/pull/2013\"\n-requested_by=\"mxinden\"\n-deployed_at=2020-11-10T20:27:11.605929Z\n-network=\"kusama\"\n-deployed_on=\"kusama-unit-test-hostname\"\n-public_fqdn=\"kusama-unit-test-hostname.example.com\"\n-internal_fqdn=\"kusama-unit-test-hostname-int.example.com\"\n-custom_binary=\"https://gitlab.example.com/parity/polkadot/-/jobs/752482/artifacts/raw/artifacts/polkadot\"")
	gitlab := newMockGitlabClient(diff)
	alertmanager := new(mockAlertManager)
	ansible := new(mockAnsibleDriver)
	matrix := new(mockMatrix)

	err := ProcessCleanup("master", gitlab, alertmanager, ansible, matrix)

	require.NoError(t, err)

	require.Len(t, alertmanager.createSilenceCalls, 1)
	silenceCall := alertmanager.createSilenceCalls[0]
	require.Len(t, silenceCall.matchers, 1)
	matcher := silenceCall.matchers[0]
	require.Equal(t, "instance", matcher.Name)
	require.Equal(t, ".*kusama-unit-test-hostname.*", matcher.Value)
	require.True(t, matcher.IsRegex)
	require.Equal(t, "Burn-in Automator", silenceCall.createdBy)
	require.Equal(
		t,
		"Cleaning up burn-in test for https://github.com/paritytech/polkadot/pull/2013 on kusama-unit-test-hostname",
		silenceCall.comment,
	)

	require.Len(t, ansible.runPlaybookCalls, 1)
	playbookCall := ansible.runPlaybookCalls[0]
	require.Equal(t, "kusama-nodes.yml", playbookCall.name)
	require.Equal(t, "kusama-unit-test-hostname.example.com", playbookCall.runOn)
	require.Equal(t, "kusama-unit-test-hostname", playbookCall.nodePublicName)
	require.NotNil(t, playbookCall.nodeBinary)
	require.Equal(t, polkadotNightlyBuildURL, playbookCall.nodeBinary.String())

	require.Len(t, gitlab.deleteFileCalls, 1)
	dfc := gitlab.deleteFileCalls[0]
	require.Equal(t, "requests/request-1602856341.toml", dfc.path)
	require.True(t, strings.HasPrefix(dfc.commitMsg, gitlab.PrefixSkipCI("")))
	require.Equal(t, "master", dfc.branch)

	require.Len(t, matrix.requestNotificationCalls, 0)
	require.Len(t, matrix.updateNotificationCalls, 0)
	require.Len(t, matrix.errorNotificationCalls, 0)
	require.Len(t, matrix.deploymentNotificationCalls, 0)
	require.Len(t, matrix.cleanupNotificationCalls, 1)
	matrixCall := matrix.cleanupNotificationCalls[0]
	require.NotNil(t, matrixCall.DeployedOn)
	require.Equal(t, "kusama-unit-test-hostname", matrixCall.DeployedOn)
}

func Test_ProcessPendingCleanup(t *testing.T) {
	diff := mkCommitDiff("runs/run-kusama-fullnode-0-1631538021.toml", false, false, true, "@@ -1,8 +0,0 @@\n-pull_request=\"https://github.com/paritytech/polkadot/pull/2013\"\n-requested_by=\"mxinden\"\n-network=\"kusama\"\n-\n-custom_binary=\"https://gitlab.example.com/parity/polkadot/-/jobs/752482/artifacts/raw/artifacts/polkadot\"")
	gitlab := newMockGitlabClient(diff)
	alertmanager := new(mockAlertManager)
	ansible := new(mockAnsibleDriver)
	matrix := new(mockMatrix)

	err := ProcessCleanup("master", gitlab, alertmanager, ansible, matrix)

	require.NoError(t, err)

	require.Len(t, alertmanager.createSilenceCalls, 0)
	require.Len(t, ansible.runPlaybookCalls, 0)

	require.Len(t, gitlab.deleteFileCalls, 1)
	dfc := gitlab.deleteFileCalls[0]
	require.Equal(t, "requests/request-1631538021.toml", dfc.path)
	require.True(t, strings.HasPrefix(dfc.commitMsg, gitlab.PrefixSkipCI("")))
	require.Equal(t, "master", dfc.branch)

	require.Len(t, matrix.requestNotificationCalls, 0)
	require.Len(t, matrix.updateNotificationCalls, 0)
	require.Len(t, matrix.errorNotificationCalls, 0)
	require.Len(t, matrix.deploymentNotificationCalls, 0)
	require.Len(t, matrix.cleanupNotificationCalls, 0)
}

func Test_parseDeletedDeploymentDiff(t *testing.T) {
	deployedAt, err := time.Parse(time.RFC3339, "2020-11-10T20:27:11.605929Z")
	require.NoError(t, err)

	deployedOn := "kusama-unit-test-hostname"

	cases := []struct {
		description    string
		input          string
		errorExpected  bool
		expectedOutput burnin.Deployment
	}{
		{
			"valid diff, but not a file deletion",
			"@@ -3,3 +3,6 @@\n+pull_request=\"https://github.com/paritytech/polkadot/pull/2013\"\n+requested_by=\"mxinden\"\n+deployed_at=2020-11-10T20:27:11.605929Z\n+deployed_on=\"kusama-unit-test-hostname\"\n+custom_binary=\"https://gitlab.example.com/parity/polkadot/-/jobs/752482/artifacts/raw/artifacts/polkadot\"",
			true,
			burnin.Deployment{},
		},
		{
			"valid",
			"@@ -1,8 +0,0 @@\n-pull_request=\"https://github.com/paritytech/polkadot/pull/2013\"\n-requested_by=\"mxinden\"\n-deployed_at=2020-11-10T20:27:11.605929Z\n-deployed_on=\"kusama-unit-test-hostname\"\n-custom_binary=\"https://gitlab.example.com/parity/polkadot/-/jobs/752482/artifacts/raw/artifacts/polkadot\"",
			false,
			burnin.Deployment{
				PullRequest:  "https://github.com/paritytech/polkadot/pull/2013",
				RequestedBy:  "mxinden",
				DeployedAt:   deployedAt,
				DeployedOn:   deployedOn,
				CustomBinary: "https://gitlab.example.com/parity/polkadot/-/jobs/752482/artifacts/raw/artifacts/polkadot",
			},
		},
	}

	for _, c := range cases {
		actualOutput, err := parseDeletedDeploymentDiff(c.input)
		if c.errorExpected {
			require.Error(t, err, c.description)
		} else {
			require.Nil(t, err)
			require.Equal(t, c.expectedOutput.PullRequest, actualOutput.PullRequest, c.description)
			require.Equal(t, c.expectedOutput.RequestedBy, actualOutput.RequestedBy, c.description)
			require.NotNil(t, actualOutput.DeployedAt, c.description)
			require.NotNil(t, actualOutput.DeployedOn, c.description)
			require.Equal(t, c.expectedOutput.DeployedAt, actualOutput.DeployedAt, c.description)
			require.Equal(t, c.expectedOutput.DeployedOn, actualOutput.DeployedOn, c.description)
			require.Equal(t, c.expectedOutput.CustomBinary, actualOutput.CustomBinary, c.description)
		}
	}
}

func Test_validCleanupCommit(t *testing.T) {
	cases := []struct {
		description    string
		input          []burnin.CommitDiff
		expectedOutput bool
	}{
		{
			"no commit diffs",
			[]burnin.CommitDiff{},
			false,
		},
		{
			"more than one commit diff",
			[]burnin.CommitDiff{
				mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", false, false, true, ""),
				mkCommitDiff("runs/run-fullnode-1-1602856340.toml", false, false, true, ""),
			},
			false,
		},
		{
			"invalid path",
			[]burnin.CommitDiff{
				mkCommitDiff("utils/cmd/run-job/main.go", false, false, true, ""),
			},
			false,
		},
		{
			"added file",
			[]burnin.CommitDiff{
				mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", true, false, false, ""),
			},
			false,
		},
		{
			"renamed file",
			[]burnin.CommitDiff{
				mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", false, true, false, ""),
			},
			false,
		},
		{
			"valid delete commit",
			[]burnin.CommitDiff{
				mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", false, false, true, "content does not get validated"),
			},
			true,
		},
	}

	for _, c := range cases {
		actualOutput := validCleanupCommit(c.input)
		require.Equal(t, c.expectedOutput, actualOutput, c.description)
	}
}
