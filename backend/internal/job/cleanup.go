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
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml"
	burnin "gitlab.example.com/burn-in-tests/backend"
)

func ProcessCleanup(
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

	if !validCleanupCommit(diffs) {
		return fmt.Errorf(
			"this CI job requires the last commit on branch '%s' to remove exactly one file from folder 'runs'",
			baseBranch,
		)
	}

	deployment, err := parseDeletedDeploymentDiff(diffs[0].Diff)
	if err != nil {
		return err
	}

	if deployment.DeployedOn != "" {
		log.Printf("creating silence for host %s\n", deployment.DeployedOn)
		comment := fmt.Sprintf("Cleaning up burn-in test for %s on %s", deployment.PullRequest, deployment.DeployedOn)
		silenceID, err := createSilence(alertmanager, deployment.DeployedOn, comment)
		if err != nil {
			return err
		}
		log.Printf("silence id: %s\n", silenceID)

		playbook := fmt.Sprintf("%s-nodes.yml", deployment.Network)
		customBinaryURL, _ := url.Parse(polkadotNightlyBuildURL) // safe to ignore errors as the input is a constant
		fqdn := deployment.PublicFQDN
		log.Printf(
			"running ansible playbook %s on host %s with 'node_binary' %v\n",
			playbook,
			fqdn,
			customBinaryURL,
		)
		if err := ansible.RunPlaybook(playbook, fqdn, customBinaryURL, false, deployment.DeployedOn, nil); err != nil {
			return err
		}

		log.Printf("unpausing gitlab runner on %s\n", deployment.DeployedOn)
		if err := gitlab.UnPauseRunner(deployment.DeployedOn); err != nil {
			return err
		}
	}

	repoRunFilePath := *diffs[0].OldPath
	runID, err := parseRunID(repoRunFilePath)
	if err != nil {
		return err
	}

	cleanup, err := requestNeedsCleaningUp(runID, baseBranch, gitlab)
	if err != nil {
		return err
	}

	if cleanup {
		repoRequestFilePath := fmt.Sprintf("requests/request-%s.toml", runID)
		log.Printf("deleting file %s on branch '%s'\n", repoRequestFilePath, baseBranch)
		commitMsg := gitlab.PrefixSkipCI(fmt.Sprintf("Delete %s", repoRequestFilePath))
		if err := gitlab.DeleteFile(repoRequestFilePath, baseBranch, commitMsg); err != nil {
			log.Printf(
				"deleting file %s on branch '%s' failed: %s. it was probably deleted by a concurrent cleanup job\n",
				repoRequestFilePath,
				baseBranch,
				err,
			)
		}
	}

	if deployment.DeployedOn != "" {
		return matrix.SendCleanupNotification(deployment)
	}

	return nil
}

// requestNeedsCleaningUp returns true, if there are no more "run" files left with a certain "run ID" (e.g. unix
// timestamp) on the given branch.
func requestNeedsCleaningUp(runID, baseBranch string, gitlab burnin.Gitlab) (bool, error) {
	files, err := gitlab.ListDirectory("runs", baseBranch)
	if err != nil {
		return false, err
	}

	nameRegex, err := regexp.Compile(fmt.Sprintf("run-(fullnode|sentry|validator)-\\d-%s.toml", runID))
	if err != nil {
		return false, err
	}

	for _, f := range files {
		if f.Type == "blob" && nameRegex.MatchString(f.Name) {
			return false, nil
		}
	}

	return true, nil
}

// parseDeletedDeploymentDiff expects a diff of a burnin.Deployment yaml where all lines are removed. It removes the
// diff characters and parses the resulting valid yaml.
func parseDeletedDeploymentDiff(diff string) (burnin.Deployment, error) {
	var deployment burnin.Deployment

	diffLines := strings.Split(diff, "\n")
	if len(diffLines) < 2 {
		return deployment, fmt.Errorf("invalid diff '%s'", diff)
	}

	tomlLines := make([]string, len(diffLines)-1)

	for i := 1; i < len(diffLines); i++ {
		dl := diffLines[i]

		if dl == "" {
			tomlLines[i-1] = ""
			continue
		}

		if dl[0] != '-' {
			return deployment, errors.New("invalid diff, expected all lines to be removed")
		}

		tomlLines[i-1] = dl[1:]
	}

	y := strings.Join(tomlLines, "\n")
	if err := toml.Unmarshal([]byte(y), &deployment); err != nil {
		return deployment, err
	}

	return deployment, nil
}

func validCleanupCommit(diffs []burnin.CommitDiff) bool {
	return len(diffs) == 1 &&
		!diffs[0].NewFile &&
		diffs[0].DeletedFile &&
		!diffs[0].RenamedFile &&
		diffs[0].OldPath != nil &&
		diffs[0].Diff != "" &&
		strings.HasPrefix(*diffs[0].OldPath, "runs/run-") &&
		strings.HasSuffix(*diffs[0].OldPath, ".toml")
}
