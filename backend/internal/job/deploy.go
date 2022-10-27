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
	"io/ioutil"
	"log"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/pelletier/go-toml"
	burnin "gitlab.example.com/burn-in-tests/backend"
)

func ProcessDeploy(
	baseDirectory string,
	baseBranch string,
	targetHostname string,
	gitlab burnin.Gitlab,
	alertmanager burnin.Alertmanager,
	ansible burnin.AnsibleDriver,
	matrix burnin.Matrix,
) error {
	diffs, err := diffsToCurrentCommit(baseBranch, gitlab)
	if err != nil {
		return err
	}

	if !validDeployment(diffs) {
		return fmt.Errorf(
			"this CI job requires the last commit on branch '%s' to add exactly one file in folder 'runs'",
			baseBranch,
		)
	}

	repoRunFilePath := *diffs[0].NewPath // path of the "run" file relative to the repository root
	localRunFilePath := path.Join(baseDirectory, repoRunFilePath)
	log.Printf("parsing file %s\n", localRunFilePath) // log the local path once to assist debugging
	deployment, err := parseRunFile(localRunFilePath)
	if err != nil {
		return err
	}

	customBinaryURL, err := url.Parse(deployment.CustomBinary)
	if err != nil {
		return err
	}

	log.Printf("creating silence for host %s\n", targetHostname)
	comment := fmt.Sprintf("Deploying burn-in test for %s on %s", deployment.PullRequest, targetHostname)
	silenceID, err := createSilence(alertmanager, targetHostname, comment)
	if err != nil {
		return err
	}
	log.Printf("silence id: %s\n", silenceID)

	playbook := fmt.Sprintf("%s-nodes.yml", deployment.Network)
	wipeChainDb := deployment.NodeType == burnin.FullNode && deployment.SyncFromScratch
	wipeChainDbLog := ""
	if wipeChainDb {
		wipeChainDbLog = "and flag '-e node_force_wipe=true'"
	}

	log.Printf(
		"running ansible playbook %s on host %s with 'node_binary' %v %s\n",
		playbook,
		targetHostname,
		customBinaryURL,
		wipeChainDbLog,
	)
	if err := ansible.RunPlaybook(
		playbook,
		"localhost",
		customBinaryURL,
		wipeChainDb,
		targetHostname,
		deployment.CustomOptions,
	); err != nil {
		return err
	}

	log.Printf("adding 'deployed_at' and 'deployed_on' to file %s\n", repoRunFilePath)
	deployment, err = addDeploymentInfo(repoRunFilePath, deployment, targetHostname, gitlab, baseBranch)
	if err != nil {
		return err
	}

	log.Printf("pausing gitlab runner on host %s\n", targetHostname)
	if err := gitlab.PauseRunner(targetHostname); err != nil {
		return err
	}

	return matrix.SendDeploymentNotification(deployment)
}

func addDeploymentInfo(
	path string,
	deployment burnin.Deployment,
	targetHostname string,
	gitlab burnin.Gitlab,
	branch string,
) (burnin.Deployment, error) {
	deployment.DeployedAt = time.Now().UTC()
	deployment.DeployedOn = targetHostname
	deployment.PublicFQDN, deployment.InternalFQDN = hostnameToFQDNs(targetHostname)

	deployment = addLogViewerURL(deployment)
	deployment = addDashboardURLs(deployment)

	runFileContent, err := toml.Marshal(deployment)
	if err != nil {
		return deployment, err
	}

	commitMsg := gitlab.PrefixSkipCI(fmt.Sprintf("deployed_on: %s", targetHostname))
	return deployment, gitlab.UpdateFile(path, branch, commitMsg, runFileContent)
}

func addLogViewerURL(deployment burnin.Deployment) burnin.Deployment {
	logQueryFmt := `["now-1h","now","%s",{"expr":"{host=\"%s\"}"}]`
	var loki string

	if deployment.Network == "westend" {
		loki = "loki.foo-bar"
	} else {
		loki = "loki.foo-baz"
	}
	logQuery := fmt.Sprintf(logQueryFmt, loki, deployment.DeployedOn)

	deployment.LogViewer = fmt.Sprintf(
		"http://grafana.example.com/explore?orgId=1&left=%s",
		url.QueryEscape(logQuery),
	)

	return deployment
}

func addDashboardURLs(deployment burnin.Deployment) burnin.Deployment {
	dashFmt := `http://grafana.example.com/d/%s?orgId=1&refresh=1m&var-nodename=%s:9615`
	fqdn := deployment.InternalFQDN

	if deployment.Network == "westend" {
		dashFmt += "&var-data_source=prometheus"
		fqdn = deployment.DeployedOn
	}

	deployment.Dashboards = map[string]string{
		"substrate_service_tasks":          fmt.Sprintf(dashFmt, "3LA6XNqZz/substrate-service-tasks", fqdn),
		"substrate_networking":             fmt.Sprintf(dashFmt, "vKVuiD9Zk/substrate-networking", fqdn),
		"kademlia_and_authority_discovery": fmt.Sprintf(dashFmt, "NMSIHdDGz/kademlia-and-authority-discovery", fqdn),
		"grandpa":                          fmt.Sprintf(dashFmt, "EzEZ60fMz/grandpa", fqdn),
	}

	return deployment
}

func validDeployment(diffs []burnin.CommitDiff) bool {
	return len(diffs) == 1 &&
		diffs[0].NewFile &&
		!diffs[0].DeletedFile &&
		!diffs[0].RenamedFile &&
		diffs[0].NewPath != nil &&
		strings.HasPrefix(*diffs[0].NewPath, "runs/run-") &&
		strings.HasSuffix(*diffs[0].NewPath, ".toml")
}

func parseRunFile(path string) (burnin.Deployment, error) {
	var deployment burnin.Deployment
	runFileContent, err := ioutil.ReadFile(path)
	if err != nil {
		return deployment, err
	}

	if err := toml.Unmarshal(runFileContent, &deployment); err != nil {
		return deployment, err
	}

	if u, err := url.Parse(deployment.CustomBinary); err != nil || !u.IsAbs() {
		return deployment, fmt.Errorf("invalid custom binary URL '%s' (%v)", deployment.CustomBinary, err)
	}
	return deployment, nil
}

// parseRunSuffix takes a string of the form "runs/run-kusama-fullnode-0-1602856340.toml" and returns
// "kusama-fullnode-0-1602856340".
func parseRunSuffix(path string) (string, error) {
	runSuffix := strings.Replace(path, "runs/run-", "", 1)
	runSuffix = strings.Replace(runSuffix, ".toml", "", 1)

	if len(runSuffix) == 0 {
		err := fmt.Errorf(
			"invalid path '%s' (must be 'runs/run-<network>-<node type>-<seq num>-<unix timestamp>.toml')",
			path,
		)
		return "", err
	}

	return runSuffix, nil
}

// parseRunID takes a string of the form "runs/run-kusama-fullnode-0-1602856340.toml" and returns "1602856340".
func parseRunID(path string) (string, error) {
	runSuffix, err := parseRunSuffix(path)
	if err != nil {
		return "", err
	}

	parts := strings.Split(runSuffix, "-")
	if len(parts) != 4 {
		err := fmt.Errorf(
			"invalid path '%s' (must be 'runs/run-<network>-<node type>-<seq num>-<unix timestamp>.toml')",
			path,
		)
		return "", err
	}

	return parts[3], nil
}
