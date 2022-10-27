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
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml"
	burnin "gitlab.example.com/burn-in-tests/backend"
)

type requestKind int

const (
	newRequest requestKind = iota
	updatedCommitSHA
	updatedCustomBinary
	updatedCommitSHAAndCustomBinary
	updatedCustomOptionsOnly
	invalidRequest
)

func ProcessRequest(
	baseDirectory string,
	baseBranch string,
	burninGitlab burnin.Gitlab,
	buildGitlab burnin.Gitlab,
	poller burnin.Poller,
	matrix burnin.Matrix,
) error {
	diffs, err := diffsToCurrentCommit(baseBranch, burninGitlab)
	if err != nil {
		return err
	}

	kind := validateRequest(diffs)
	if kind == invalidRequest {
		return fmt.Errorf(
			"this CI job requires the last commit on branch '%s' to add or update exactly one file in folder 'requests'. in case of an update, only changes to 'commit_sha' or 'custom_binary' are currently supported",
			baseBranch,
		)
	}

	repoRequestFilePath := *diffs[0].NewPath // path of the "request" file relative to the repository root
	requestID, err := parseRequestID(repoRequestFilePath)
	if err != nil {
		return err
	}

	localRequestFilePath := path.Join(baseDirectory, repoRequestFilePath)
	log.Printf("parsing file %s\n", localRequestFilePath) // log local path once to assist debugging
	request, err := parseRequestFile(localRequestFilePath)
	if err != nil {
		return err
	}

	if kind == newRequest {
		return processNewRequest(
			requestID,
			request,
			baseBranch,
			burninGitlab,
			buildGitlab,
			poller,
			matrix,
		)
	}

	return processUpdatedRequest(
		requestID,
		request,
		kind,
		baseDirectory,
		baseBranch,
		burninGitlab,
		buildGitlab,
		poller,
	)
}

func processNewRequest(
	requestID string,
	request burnin.Request,
	branch string,
	burninGitlab burnin.Gitlab,
	buildGitlab burnin.Gitlab,
	poller burnin.Poller,
	matrix burnin.Matrix,
) error {
	log.Println("processing new burn-in request...")

	deployment := burnin.Deployment{
		PullRequest:     request.PullRequest,
		CommitSHA:       request.CommitSHA,
		RequestedBy:     request.RequestedBy,
		SyncFromScratch: request.SyncFromScratch,
		CustomOptions:   request.CustomOptions,
	}

	if request.CustomBinary == nil {
		log.Println("no 'custom_binary' provided. trying to retrieve it...")

		prURL, err := url.Parse(request.PullRequest)
		if err != nil {
			return err
		}

		customBinary, commitSHA, err := buildPolkadotBinary(prURL, request.CommitSHA, buildGitlab, poller)
		if err != nil {
			return err
		}

		deployment.CommitSHA = commitSHA
		deployment.CustomBinary = customBinary.String()
	} else {
		deployment.CustomBinary = *request.CustomBinary
	}

	for network, nodeTypes := range request.Nodes {
		for nodeType, count := range nodeTypes {
			for i := 0; i < count; i++ {
				deployment.Network = network
				deployment.NodeType = nodeType
				deploymentToml, err := toml.Marshal(deployment)
				if err != nil {
					return err
				}

				commitMsg := burninGitlab.PrefixDeploy(network, nodeType, request.PullRequest)
				runPath := fmt.Sprintf("runs/run-%s-%s-%d-%s.toml", network, nodeType, i, requestID)
				log.Printf("committing file %s on branch '%s'\n", runPath, branch)
				if err := burninGitlab.CreateFile(runPath, branch, commitMsg, deploymentToml); err != nil {
					return err
				}
			}
		}
	}

	return matrix.SendRequestNotification(request)
}

func processUpdatedRequest(
	requestID string,
	request burnin.Request,
	kind requestKind,
	baseDirectory string,
	baseBranch string,
	burninGitlab burnin.Gitlab,
	buildGitlab burnin.Gitlab,
	poller burnin.Poller,
) error {
	log.Println("processing update to an existing burn-in request...")

	var (
		customBinary *url.URL
		commitSHA    string
		err          error
	)

	if kind == updatedCommitSHA {
		prURL, err := url.Parse(request.PullRequest)
		if err != nil {
			return err
		}

		customBinary, commitSHA, err = buildPolkadotBinary(prURL, request.CommitSHA, buildGitlab, poller)
		if err != nil {
			return err
		}
	} else if kind == updatedCustomBinary && request.CommitSHA != "" {
		log.Println("'custom_binary' was updated, but 'commit_sha' was not. removing 'commit_sha' from \"run\" files")
		commitSHA = ""
	} else if kind == updatedCommitSHAAndCustomBinary || kind == updatedCustomOptionsOnly {
		commitSHA = request.CommitSHA
	}

	// Regardless whether "custom_binary" was added or present in the original request, we need to pass a valid URL to
	// updateDeployment().
	if customBinary == nil && request.CustomBinary != nil {
		customBinary, err = url.Parse(*request.CustomBinary)
		if err != nil {
			return err
		}
	}

	deployments, err := findDeployments(requestID, baseDirectory)
	if err != nil {
		return err
	}

	for _, deployment := range deployments {
		err := updateDeployment(deployment, commitSHA, customBinary, request.CustomOptions, baseBranch, burninGitlab)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateDeployment(
	deployment burnin.Deployment,
	commitSHA string,
	customBinary *url.URL,
	customOptions []string,
	branch string,
	gitlab burnin.Gitlab,
) error {
	deployment.CommitSHA = commitSHA
	if customBinary != nil {
		deployment.CustomBinary = customBinary.String()
	}
	deployment.CustomOptions = customOptions

	runFileContent, err := toml.Marshal(deployment)
	if err != nil {
		return err
	}

	relPath := path.Join("runs", deployment.Filename)
	commitMsg := fmt.Sprintf("Update commit_sha and custom_binary in %s", relPath)

	return gitlab.UpdateFile(
		relPath,
		branch,
		gitlab.PrefixUpdateDeployment(commitMsg),
		runFileContent,
	)
}

func findDeployments(requestID, baseDirectory string) ([]burnin.Deployment, error) {
	runsDir := path.Join(baseDirectory, "runs")
	pattern := fmt.Sprintf("run-*-%s.toml", requestID)
	runFiles, err := filepath.Glob(path.Join(runsDir, pattern))
	if err != nil {
		return nil, err
	}

	if runFiles == nil {
		return nil, fmt.Errorf("no \"run\" files found for burn-in request '%s'", requestID)
	}

	deployments := make([]burnin.Deployment, len(runFiles))
	for i, runFile := range runFiles {
		fd, err := os.Open(runFile)
		if err != nil {
			return nil, err
		}

		var deployment burnin.Deployment
		err = toml.NewDecoder(fd).Decode(&deployment)
		_ = fd.Close()
		if err != nil {
			return nil, err
		}

		deployment.Filename = strings.Replace(runFile, runsDir, "", 1)
		deployments[i] = deployment
	}

	return deployments, nil
}

func validateRequest(diffs []burnin.CommitDiff) requestKind {
	common := len(diffs) == 1 &&
		!diffs[0].DeletedFile &&
		!diffs[0].RenamedFile &&
		diffs[0].NewPath != nil &&
		strings.HasPrefix(*diffs[0].NewPath, "requests/request-") &&
		strings.HasSuffix(*diffs[0].NewPath, ".toml")

	if !common {
		return invalidRequest
	}

	if diffs[0].NewFile {
		return newRequest
	}

	content := diffs[0].Diff
	customBinaryChanged := strings.Contains(content, "+custom_binary")
	customOptionsChanged := strings.Contains(content, "+custom_options")
	commitSHAChanged := strings.Contains(content, "+commit_sha")
	if commitSHAChanged && customBinaryChanged {
		return updatedCommitSHAAndCustomBinary
	} else if commitSHAChanged {
		return updatedCommitSHA
	} else if customBinaryChanged {
		return updatedCustomBinary
	} else if customOptionsChanged {
		return updatedCustomOptionsOnly
	}

	return invalidRequest
}

func parseRequestID(path string) (string, error) {
	requestID := strings.Replace(path, "requests/request-", "", 1)
	requestID = strings.Replace(requestID, ".toml", "", 1)

	if len(requestID) == 0 {
		return "", fmt.Errorf("invalid path '%s' (must be 'requests/request-<unix timestamp>.toml')", path)
	}

	return requestID, nil
}

func parseRequestFile(path string) (burnin.Request, error) {
	var request burnin.Request

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return request, err
	}

	if err = toml.Unmarshal(data, &request); err != nil {
		return request, err
	}

	if !strings.HasPrefix(request.PullRequest, "https://github.com/paritytech/polkadot/") {
		err = fmt.Errorf(
			"invalid pull request URL: '%s'. only https://github.com/paritytech/polkadot/ is currently supported",
			request.PullRequest,
		)
		return request, err
	}

	for network, nodeTypes := range request.Nodes {
		for nodeType, count := range nodeTypes {
			if count < 0 || count > 5 {
				return request, fmt.Errorf(
					"using %d %s nodes on %s for a burn-in seems a bit much. aborting",
					count,
					nodeType,
					network,
				)
			}
		}
	}

	if request.CustomBinary != nil {
		if u, err := url.Parse(*request.CustomBinary); err != nil || !u.IsAbs() {
			return request, fmt.Errorf("invalid custom binary URL '%s' (%v)", *request.CustomBinary, err)
		}
	}

	return request, nil
}
