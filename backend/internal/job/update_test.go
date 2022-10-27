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

	"github.com/stretchr/testify/require"
	burnin "gitlab.example.com/burn-in-tests/backend"
)

const validDiffContent = `@@ -1,6 +1,6 @@pull_request="https://github.com/paritytech/polkadot/pull/2013"\n-commit_sha="f52b0b01d8f27fdb387667de5a56da2754ce77a1"\n+commit_sha="a7810560c0f62dd6d347e710a5e2a64da465c109"\nrequested_by="mxinden"\nsync_from_scratch=false\n[node_types]\n`

func Test_ProcessUpdate(t *testing.T) {
	diff := mkCommitDiff("runs/run-kusama-fullnode-0-1610469388.toml", false, false, false, validDiffContent)
	gitlab := newMockGitlabClient(diff)
	alertmanager := new(mockAlertManager)
	ansible := new(mockAnsibleDriver)
	matrix := new(mockMatrix)

	err := ProcessUpdate("testdata", "master", gitlab, alertmanager, ansible, matrix)

	require.NoError(t, err)

	require.Len(t, alertmanager.createSilenceCalls, 1)
	silenceCall := alertmanager.createSilenceCalls[0]
	require.Len(t, silenceCall.matchers, 1)
	matcher := silenceCall.matchers[0]
	require.Equal(t, "instance", matcher.Name)
	require.Equal(t, ".*kusama-fullnode-uw1-0.*", matcher.Value)
	require.True(t, matcher.IsRegex)
	require.Equal(t, "Burn-in Automator", silenceCall.createdBy)
	require.Equal(
		t,
		"Updating burn-in test for https://github.com/paritytech/polkadot/pull/2013 on kusama-fullnode-uw1-0",
		silenceCall.comment,
	)

	require.Len(t, ansible.runPlaybookCalls, 1)
	playbookCall := ansible.runPlaybookCalls[0]
	require.Equal(t, "kusama-nodes.yml", playbookCall.name)
	require.Equal(t, "kusama-fullnode-uw1-0.example.com", playbookCall.runOn)
	require.Equal(t, "kusama-fullnode-uw1-0", playbookCall.nodePublicName)
	require.NotNil(t, playbookCall.nodeBinary)

	require.Equal(
		t,
		"https://gitlab.example.com/mocks/mockproject/-/jobs/752482/artifacts/raw/artifacts/polkadot",
		playbookCall.nodeBinary.String(),
	)

	require.Len(t, gitlab.createFileCalls, 0)
	require.Len(t, gitlab.deleteFileCalls, 0)

	require.Len(t, gitlab.updateFileCalls, 1)
	updateCall := gitlab.updateFileCalls[0]
	require.Equal(t, "master", updateCall.branch)
	require.Equal(t, "runs/run-kusama-fullnode-0-1610469388.toml", updateCall.path)
	require.True(t, strings.HasPrefix(updateCall.commitMsg, "[skip ci]"))

	require.Len(t, matrix.requestNotificationCalls, 0)
	require.Len(t, matrix.deploymentNotificationCalls, 0)
	require.Len(t, matrix.cleanupNotificationCalls, 0)
	require.Len(t, matrix.errorNotificationCalls, 0)
	require.Len(t, matrix.updateNotificationCalls, 1)
	matrixCall := matrix.updateNotificationCalls[0]
	require.NotEmpty(t, matrixCall.UpdatedAt)
	require.NotNil(t, matrixCall.DeployedOn)
	require.Equal(t, "kusama-fullnode-uw1-0", matrixCall.DeployedOn)
}

func Test_validUpdateCommit(t *testing.T) {
	cases := []struct {
		description    string
		input          []burnin.CommitDiff
		expectedResult bool
	}{
		{
			"no commit diffs",
			[]burnin.CommitDiff{},
			false,
		},
		{
			"more than one commit diff",
			[]burnin.CommitDiff{
				mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", false, false, true, validDiffContent),
				mkCommitDiff("runs/run-fullnode-1-1602856340.toml", false, false, true, validDiffContent),
			},
			false,
		},
		{
			"invalid path",
			[]burnin.CommitDiff{
				mkCommitDiff("utils/cmd/run-job/main.go", false, false, true, validDiffContent),
			},
			false,
		},
		{
			"added file",
			[]burnin.CommitDiff{
				mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", true, false, false, validDiffContent),
			},
			false,
		},
		{
			"renamed file",
			[]burnin.CommitDiff{
				mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", false, true, false, validDiffContent),
			},
			false,
		},
		{
			"deleted file",
			[]burnin.CommitDiff{
				mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", false, false, true, validDiffContent),
			},
			false,
		},
		{
			"invalid diff content; neither commit_sha nor custom_binary change",
			[]burnin.CommitDiff{
				mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", false, false, false, "@@ -1,6 +1,6 @@-sync_from_scratch=false\n+sync_from_scratch=true\n[node_types]\n"),
			},
			false,
		},
		{
			"valid update commit",
			[]burnin.CommitDiff{
				mkCommitDiff("runs/run-kusama-fullnode-0-1602856340.toml", false, false, false, validDiffContent),
			},
			true,
		},
	}

	for _, c := range cases {
		actualResult := validUpdateCommit(c.input)
		require.Equal(t, c.expectedResult, actualResult, c.description)
	}
}
