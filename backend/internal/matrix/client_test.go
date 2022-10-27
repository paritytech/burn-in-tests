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
package matrix

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	burnin "gitlab.example.com/burn-in-tests/backend"
)

func Test_requestTmpl(t *testing.T) {
	vars := tmplVars{
		Request: burnin.Request{
			PullRequest: "https://github.com/paritytech/polkadot/pull/2398",
			RequestedBy: "haiko@example.com",
		},
		PullRequest: formatPullRequest("https://github.com/paritytech/polkadot/pull/2398"),
		JobURL:      template.URL("https://gitlab.example.com/deployments/burn-in-tests/-/jobs/752482/"),
	}

	buf := new(bytes.Buffer)
	err := requestTmpl.Execute(buf, vars)

	require.Nil(t, err)
	rendered := buf.String()
	require.Contains(t, rendered, "polkadot#2398")
}

func Test_deployTmpl(t *testing.T) {
	vars := tmplVars{
		Deployment: burnin.Deployment{
			PullRequest:  "https://github.com/paritytech/polkadot/pull/2398",
			CommitSHA:    "0fb42a943e216914ee7181b978c86786edbd07ba",
			CustomBinary: "https://gitlab.example.com/parity/polkadot/-/jobs/805835/artifacts/raw/artifacts/polkadot",
			RequestedBy:  "haiko@example.com",
			DeployedOn:   "kusama-unit-test-hostname",
			DeployedAt:   time.Now().UTC(),
			Network:      "kusama",
			NodeType:     "fullnode",
			LogViewer:    "http://grafana.example.com/explore?orgId=1&left=%5B%22now-1h%22%2C%22now%22%2C%22loki%22%2C%7B%22expr%22%3A%22%7Bhost%3D%5C%22kusama-unit-test-hostname%5C%22%7D%22%7D%5D",
			Dashboards: map[string]string{
				"substrate_networking": "http://grafana.example.com/d/vKVuiD9Zk/substrate-networking?orgId=1&refresh=1m&var-nodename=kusama-unit-test-hostname-int.example.com:9615",
			},
		},
		PullRequest: formatPullRequest("https://github.com/paritytech/polkadot/pull/2398"),
		JobURL:      template.URL("https://gitlab.example.com/deployments/burn-in-tests/-/jobs/752482/"),
	}

	vars.CommitURL = buildCommitURL(vars.Deployment.CommitSHA, vars.Deployment.PullRequest)
	vars.DashboardURL = template.URL(vars.Deployment.Dashboards["substrate_networking"])

	buf := new(bytes.Buffer)
	err := deployTmpl.Execute(buf, vars)

	require.Nil(t, err)
	rendered := buf.String()
	require.True(t, strings.Contains(rendered, "polkadot#2398"))

	require.Contains(
		t,
		rendered,
		"https://github.com/paritytech/polkadot/tree/0fb42a943e216914ee7181b978c86786edbd07ba",
	)

	require.Contains(
		t,
		rendered,
		"http://grafana.example.com/d/vKVuiD9Zk/substrate-networking?orgId=1&amp;refresh=1m&amp;var-nodename=kusama-unit-test-hostname-int.example.com:9615",
	)
}

func Test_updateTmpl(t *testing.T) {
	vars := tmplVars{
		Deployment: burnin.Deployment{
			PullRequest:  "https://github.com/paritytech/polkadot/pull/2398",
			CommitSHA:    "0fb42a943e216914ee7181b978c86786edbd07ba",
			CustomBinary: "https://gitlab.example.com/parity/polkadot/-/jobs/805835/artifacts/raw/artifacts/polkadot",
			RequestedBy:  "haiko@example.com",
			DeployedOn:   "kusama-unit-test-hostname",
			DeployedAt:   time.Now().UTC(),
			Network:      "kusama",
			NodeType:     "fullnode",
			LogViewer:    "http://grafana.example.com/explore?orgId=1&left=%5B%22now-1h%22%2C%22now%22%2C%22loki%22%2C%7B%22expr%22%3A%22%7Bhost%3D%5C%22kusama-unit-test-hostname%5C%22%7D%22%7D%5D",
			Dashboards: map[string]string{
				"substrate_networking": "http://grafana.example.com/d/vKVuiD9Zk/substrate-networking?orgId=1&refresh=1m&var-nodename=kusama-unit-test-hostname-int.example.com:9615",
			},
		},
		PullRequest: formatPullRequest("https://github.com/paritytech/polkadot/pull/2398"),
		JobURL:      template.URL("https://gitlab.example.com/deployments/burn-in-tests/-/jobs/752482/"),
	}

	vars.CommitURL = buildCommitURL(vars.Deployment.CommitSHA, vars.Deployment.PullRequest)
	vars.DashboardURL = template.URL(vars.Deployment.Dashboards["substrate_networking"])

	buf := new(bytes.Buffer)
	err := updateTmpl.Execute(buf, vars)

	require.Nil(t, err)
	rendered := buf.String()
	require.True(t, strings.Contains(rendered, "polkadot#2398"))

	require.Contains(
		t,
		rendered,
		"https://github.com/paritytech/polkadot/tree/0fb42a943e216914ee7181b978c86786edbd07ba",
	)

	require.Contains(
		t,
		rendered,
		"http://grafana.example.com/d/vKVuiD9Zk/substrate-networking?orgId=1&amp;refresh=1m&amp;var-nodename=kusama-unit-test-hostname-int.example.com:9615",
	)
}

func Test_cleanupTmpl(t *testing.T) {
	vars := tmplVars{
		Deployment: burnin.Deployment{
			PullRequest:  "https://github.com/paritytech/polkadot/pull/2398",
			CommitSHA:    "0fb42a943e216914ee7181b978c86786edbd07ba",
			CustomBinary: "https://gitlab.example.com/parity/polkadot/-/jobs/805835/artifacts/raw/artifacts/polkadot",
			RequestedBy:  "haiko@example.com",
			DeployedOn:   "kusama-unit-test-hostname",
			DeployedAt:   time.Now().UTC(),
			Network:      "kusama",
			NodeType:     "fullnode",
			LogViewer:    "http://grafana.example.com/explore?orgId=1&left=%5B%22now-1h%22%2C%22now%22%2C%22loki%22%2C%7B%22expr%22%3A%22%7Bhost%3D%5C%22kusama-unit-test-hostname%5C%22%7D%22%7D%5D",
			Dashboards: map[string]string{
				"substrate_networking": "http://grafana.example.com/d/vKVuiD9Zk/substrate-networking?orgId=1&refresh=1m&var-nodename=kusama-unit-test-hostname-int.example.com:9615",
			},
		},
		PullRequest: formatPullRequest("https://github.com/paritytech/polkadot/pull/2398"),
		JobURL:      template.URL("https://gitlab.example.com/deployments/burn-in-tests/-/jobs/752482/"),
	}

	vars.CommitURL = buildCommitURL(vars.Deployment.CommitSHA, vars.Deployment.PullRequest)
	vars.DashboardURL = template.URL(vars.Deployment.Dashboards["substrate_networking"])

	buf := new(bytes.Buffer)
	err := cleanupTmpl.Execute(buf, vars)

	require.Nil(t, err)
	rendered := buf.String()
	require.True(t, strings.Contains(rendered, "polkadot#2398"))
	require.Contains(t, rendered, "haiko@example.com")

	require.Contains(
		t,
		rendered,
		"https://gitlab.example.com/deployments/burn-in-tests/-/jobs/752482/",
	)
}
