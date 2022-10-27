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
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	burnin "gitlab.example.com/burn-in-tests/backend"
)

const silenceComment = "Deploying nightly Polkadot build to idle burn-in nodes"

// ProcessRefresh relies on the convention that the "description" fields as returned by GET /api/v4/runners/all contains
// the hostname (and nothing else). It also relies on the runners having tags such as "polkadot-fullnode" to determine
// the blockchain network they are connected to.
func ProcessRefresh(gitlab burnin.Gitlab, alertmanager burnin.Alertmanager, ansible burnin.AnsibleDriver) error {
	hostnamesByNetwork, err := getRunnerHostnamesByNetwork(gitlab, true)
	if err != nil {
		return err
	}

	for network, hostnames := range hostnamesByNetwork {
		playbook := fmt.Sprintf("%s-nodes.yml", network)
		customBinaryURL, _ := url.Parse(polkadotNightlyBuildURL) // safe to ignore errors as the input is a constant

		// Run the playbook separately for each hostname to avoid edge cases where it fails on some of them.
		for _, hostname := range hostnames {
			log.Printf("creating silence for %s host %v\n", network, hostname)
			sid, err := createRefreshSilence(alertmanager, hostname, network, silenceComment)
			if err != nil {
				return err
			}
			log.Printf("silence id: %s\n", sid)

			fqdn, _ := hostnameToFQDNs(hostname)

			log.Printf(
				"running ansible playbook %s on host %s with 'node_binary' %v\n",
				playbook,
				fqdn,
				customBinaryURL,
			)

			if err := ansible.RunPlaybook(playbook, fqdn, customBinaryURL, false, hostname, nil); err != nil {
				return err
			}
		}
	}

	return nil
}

func getRunnerHostnamesByNetwork(gitlab burnin.Gitlab, skipValidators bool) (map[string][]string, error) {
	hostnamesByNetwork := make(map[string][]string)

	runners, err := gitlab.GetRunners()
	if err != nil {
		return nil, err
	}

	// It seems that sometimes the Gitlab API response to /runners/all contains duplicates. Therefore we keep track of
	// which ones we've already added to hostnamesByNetwork.
	seen := make(map[string]bool)

	for _, runner := range runners {
		if !runner.Active {
			continue
		}

		tags, err := gitlab.GetRunnerTags(runner.ID)
		if err != nil {
			return nil, err
		}

		for _, tag := range tags {
			parts := strings.Split(tag, "-")
			if len(parts) != 2 {
				continue
			}

			if skipValidators && parts[1] == burnin.Validator {
				continue
			} else if burnin.NodeType(parts[1]) == burnin.FullNode || parts[1] == burnin.Sentry {
				if !seen[runner.Description] {
					hostnamesByNetwork[parts[0]] = append(hostnamesByNetwork[parts[0]], runner.Description)
					seen[runner.Description] = true
				}
			}
		}
	}

	return hostnamesByNetwork, nil
}

// createRefreshSilence adds a "chain" matcher and sets the silence duration to 20min instead of 5
func createRefreshSilence(alertmanager burnin.Alertmanager, hostname, network, comment string) (string, error) {
	matchers := []burnin.AlertMatcher{
		{
			Name:    "instance",
			Value:   fmt.Sprintf(".*%s.*", hostname),
			IsRegex: true,
		},
		{
			Name:    "chain",
			Value:   network,
			IsRegex: false,
		},
	}

	startsAt := time.Now()
	endsAt := startsAt.Add(time.Minute * 20)
	return alertmanager.CreateSilence(matchers, startsAt, endsAt, "Burn-in Automator", comment)
}
