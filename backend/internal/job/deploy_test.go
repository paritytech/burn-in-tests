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
	"testing"

	"github.com/stretchr/testify/require"
	burnin "gitlab.example.com/burn-in-tests/backend"
)

func Test_ProcessDeploy(t *testing.T) {
	diff := mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", true, false, false, "")
	gitlab := newMockGitlabClient(diff)
	alertmanager := new(mockAlertManager)
	ansible := new(mockAnsibleDriver)
	matrix := new(mockMatrix)

	err := ProcessDeploy("testdata", "master", "kusama-unit-test-hostname", gitlab, alertmanager, ansible, matrix)

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
		"Deploying burn-in test for https://github.com/paritytech/polkadot/pull/2013 on kusama-unit-test-hostname",
		silenceCall.comment,
	)

	require.Len(t, ansible.runPlaybookCalls, 1)

	ansibleRefreshCall := ansible.runPlaybookCalls[0]
	require.Equal(t, "kusama-nodes.yml", ansibleRefreshCall.name)
	require.Equal(t, "localhost", ansibleRefreshCall.runOn)
	require.Equal(t, "kusama-unit-test-hostname", ansibleRefreshCall.nodePublicName)
	require.Equal(
		t,
		"https://gitlab.example.com/parity/polkadot/-/jobs/752482/artifacts/raw/artifacts/polkadot",
		ansibleRefreshCall.nodeBinary.String(),
	)
	require.Len(t, ansibleRefreshCall.customOptions, 2)
	require.Equal(t, "--wasm-execution Compiled", ansibleRefreshCall.customOptions[0])
	require.Equal(t, "--rpc-methods Unsafe", ansibleRefreshCall.customOptions[1])

	require.Len(t, gitlab.createBranchCalls, 0)
	require.Len(t, gitlab.createMergeRequestCalls, 0)
	require.Len(t, gitlab.deleteFileCalls, 0)

	require.Len(t, matrix.requestNotificationCalls, 0)
	require.Len(t, matrix.updateNotificationCalls, 0)
	require.Len(t, matrix.cleanupNotificationCalls, 0)
	require.Len(t, matrix.errorNotificationCalls, 0)
	require.Len(t, matrix.deploymentNotificationCalls, 1)
	matrixCall := matrix.deploymentNotificationCalls[0]
	require.NotNil(t, matrixCall.DeployedOn)
	require.Equal(t, "kusama-unit-test-hostname", matrixCall.DeployedOn)
	require.NotNil(t, matrixCall.PublicFQDN)
	require.Equal(t, "kusama-unit-test-hostname.example.com", matrixCall.PublicFQDN)
	require.NotNil(t, matrixCall.InternalFQDN)
	require.Equal(t, "kusama-unit-test-hostname-int.example.com", matrixCall.InternalFQDN)

	require.Equal(
		t,
		"http://grafana.example.com/explore?orgId=1&left=%5B%22now-1h%22%2C%22now%22%2C%22loki%22%2C%7B%22expr%22%3A%22%7Bhost%3D%5C%22kusama-unit-test-hostname%5C%22%7D%22%7D%5D",
		matrixCall.LogViewer,
	)

	require.Len(t, matrixCall.Dashboards, 4)
	svcTasksURL, ok := matrixCall.Dashboards["substrate_service_tasks"]
	require.True(t, ok)
	require.Equal(
		t,
		"http://grafana.example.com/d/3LA6XNqZz/substrate-service-tasks?orgId=1&refresh=1m&var-nodename=kusama-unit-test-hostname-int.example.com:9615",
		svcTasksURL,
	)
}

func Test_validDeployment(t *testing.T) {
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
				mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", true, false, false, ""),
				mkCommitDiff("runs/run-full-1-1602856340.yaml", true, false, false, ""),
			},
			false,
		},
		{
			"invalid path",
			[]burnin.CommitDiff{
				mkCommitDiff("utils/cmd/run-job/main.go", true, false, false, ""),
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
			"deleted file",
			[]burnin.CommitDiff{
				mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", false, false, true, ""),
			},
			false,
		},
		{
			"valid request",
			[]burnin.CommitDiff{
				mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", true, false, false, ""),
			},
			true,
		},
	}

	for _, c := range cases {
		actualOutput := validDeployment(c.input)
		require.Equal(t, c.expectedOutput, actualOutput, c.description)
	}
}
