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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/pelletier/go-toml"
	burnin "gitlab.example.com/burn-in-tests/backend"
)

func ProcessUpdate(
	baseDirectory string,
	baseBranch string,
	gitlab burnin.Gitlab,
	alertmanager burnin.Alertmanager,
	ansible burnin.AnsibleDriver,
	matrix burnin.Matrix,
) error {
	diffs, err := diffsToCurrentCommit(baseBranch, gitlab)
	if err != nil {
		return err
	}

	// FIXME For some reason valid update commits are rejected.
	if !validUpdateCommit(diffs) {
		log.Printf(
			"this CI job requires the last commit on branch '%s' to update exactly one file in folder 'runs'\n",
			baseBranch,
		)

		jsonDiff, err := json.MarshalIndent(diffs, "", "    ")
		if err != nil {
			return err
		}
		log.Printf("DEBUG Diff received from Gitlab API: %s\n", string(jsonDiff))
	}

	repoRunFilePath := *diffs[0].NewPath // path of the "run" file relative to the repository root
	localRunFilePath := path.Join(baseDirectory, repoRunFilePath)
	log.Printf("parsing file %s\n", localRunFilePath) // log the local path once to assist debugging
	deployment, err := parseRunFile(localRunFilePath)
	if err != nil {
		return err
	}

	if deployment.DeployedOn == "" {
		return errors.New(
			"seems like this has not been deployed yet. maybe the request was updated too soon. please consult a human",
		)
	}
	log.Printf("updating ongoing burn-in test for '%s' on host '%s'...\n", deployment.PullRequest, deployment.DeployedOn)

	customBinaryURL, err := url.Parse(deployment.CustomBinary)
	if err != nil {
		return err
	}

	log.Printf("creating silence for host %s\n", deployment.DeployedOn)
	comment := fmt.Sprintf("Updating burn-in test for %s on %s", deployment.PullRequest, deployment.DeployedOn)
	silenceID, err := createSilence(alertmanager, deployment.DeployedOn, comment)
	if err != nil {
		return err
	}
	log.Printf("silence id: %s\n", silenceID)

	playbook := fmt.Sprintf("%s-nodes.yml", deployment.Network)
	log.Printf("running ansible playbook %s on host %s\n", playbook, deployment.DeployedOn)
	err = ansible.RunPlaybook(
		playbook,
		deployment.PublicFQDN,
		customBinaryURL,
		false,
		deployment.DeployedOn,
		deployment.CustomOptions,
	)
	if err != nil {
		return err
	}

	deployment, err = updateDeploymentInfo(repoRunFilePath, deployment, deployment.DeployedOn, gitlab, baseBranch)
	if err != nil {
		return err
	}

	return matrix.SendUpdateNotification(deployment)
}

func updateDeploymentInfo(
	path string,
	deployment burnin.Deployment,
	targetHostname string,
	gitlab burnin.Gitlab,
	branch string,
) (burnin.Deployment, error) {
	deployment.UpdatedAt = time.Now().UTC()

	runFileContent, err := toml.Marshal(deployment)
	if err != nil {
		return deployment, err
	}
	commitMsg := gitlab.PrefixSkipCI(fmt.Sprintf("Update 'updated_at' for current burn-in on %s", targetHostname))
	return deployment, gitlab.UpdateFile(path, branch, commitMsg, runFileContent)
}

func validUpdateCommit(diffs []burnin.CommitDiff) bool {
	if len(diffs) != 1 {
		return false
	}

	diff := diffs[0].Diff
	customBinaryChanged := strings.Contains(diff, "-custom_binary") && strings.Contains(diff, "+custom_binary")
	customOptionsChanged := strings.Contains(diff, "-custom_options") && strings.Contains(diff, "+custom_options")
	commitSHAChanged := strings.Contains(diff, "-commit_sha") && strings.Contains(diff, "+commit_sha")

	return !diffs[0].NewFile &&
		!diffs[0].DeletedFile &&
		!diffs[0].RenamedFile &&
		diffs[0].NewPath == diffs[0].OldPath &&
		(customBinaryChanged || customOptionsChanged || commitSHAChanged) &&
		strings.HasPrefix(*diffs[0].NewPath, "runs/run-") &&
		strings.HasSuffix(*diffs[0].NewPath, ".toml")
}
