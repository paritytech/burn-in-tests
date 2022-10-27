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

func Test_ProcessRefresh(t *testing.T) {
	gitlab := newMockGitlabClient(burnin.CommitDiff{})
	gitlab.runners = []burnin.Runner{
		{
			ID:          1,
			Description: "kusama-fullnode-ew1-0",
			Active:      true,
			IPAddress:   "10.1.1.42",
			IsShared:    true,
			Online:      true,
			Status:      "online",
		},
		{
			ID:          2,
			Description: "polkadot-fullnode-ew1-0",
			Active:      true,
			IPAddress:   "10.1.1.23",
			IsShared:    false,
			Online:      true,
			Status:      "online",
		},
		{
			ID:          3,
			Description: "polkadot-validator-ew1-1",
			Active:      true,
			IPAddress:   "10.1.1.24",
			IsShared:    false,
			Online:      true,
			Status:      "online",
		},
		{
			ID:          4,
			Description: "polkadot-fullnode-ew1-2",
			Active:      false,
			IPAddress:   "10.1.1.5",
			IsShared:    false,
			Online:      true,
			Status:      "paused",
		},
		{
			ID:          5,
			Description: "gl-runner-4711",
			Active:      true,
			IPAddress:   "10.1.1.80",
			IsShared:    true,
			Online:      true,
			Status:      "online",
		},
	}
	gitlab.runnerTags = map[int][]string{
		1: {"foo", "kusama-fullnode"},
		2: {"polkadot-fullnode", "bar"},
		3: {"polkadot-validator"},
		4: {"polkadot-fullnode"},
		5: {"something", "unrelated-foo"},
	}

	alertmanager := new(mockAlertManager)
	ansible := new(mockAnsibleDriver)

	err := ProcessRefresh(gitlab, alertmanager, ansible)

	require.NoError(t, err)
	require.Len(t, alertmanager.createSilenceCalls, 2)
	var kusamaSilenceCall createSilenceArgs
	var polkadotSilenceCall createSilenceArgs

	for _, silenceCall := range alertmanager.createSilenceCalls {
		if silenceCall.GetMatcher("chain").Value == "kusama" {
			kusamaSilenceCall = silenceCall
		} else {
			polkadotSilenceCall = silenceCall
		}
	}

	require.Len(t, kusamaSilenceCall.matchers, 2)
	require.Equal(t, "Burn-in Automator", kusamaSilenceCall.createdBy)
	require.Equal(t, "Deploying nightly Polkadot build to idle burn-in nodes", kusamaSilenceCall.comment)

	instanceMatcher := kusamaSilenceCall.GetMatcher("instance")
	require.NotNil(t, instanceMatcher)
	require.True(t, instanceMatcher.IsRegex)
	require.Equal(t, instanceMatcher.Value, ".*kusama-fullnode-ew1-0.*")

	require.Len(t, polkadotSilenceCall.matchers, 2)
	require.Equal(t, "Burn-in Automator", polkadotSilenceCall.createdBy)
	require.Equal(t, "Deploying nightly Polkadot build to idle burn-in nodes", polkadotSilenceCall.comment)

	instanceMatcher = polkadotSilenceCall.GetMatcher("instance")
	require.NotNil(t, instanceMatcher)
	require.True(t, instanceMatcher.IsRegex)
	require.Equal(t, instanceMatcher.Value, ".*polkadot-fullnode-ew1-0.*")

	require.Len(t, ansible.runPlaybookCalls, 2)
	for _, pbCall := range ansible.runPlaybookCalls {
		require.NotNil(t, pbCall.nodeBinary)
		require.Equal(t, polkadotNightlyBuildURL, pbCall.nodeBinary.String())
		require.True(t, pbCall.name == "kusama-nodes.yml" || pbCall.name == "polkadot-nodes.yml")
	}
}
