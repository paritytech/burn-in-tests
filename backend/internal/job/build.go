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
	"strings"
	"time"

	burnin "gitlab.example.com/burn-in-tests/backend"
)

var canContinue = map[string]bool{
	"created": true,
	"pending": true,
	"running": true,
	"success": true,
	"failed":  true, // If pipeline status is "failed", the "build-linux-stable" job might still work.
}

func buildPolkadotBinary(
	pullRequestURL *url.URL,
	commitSHA string,
	gitlab burnin.Gitlab,
	poller burnin.Poller,
) (*url.URL, string, error) {
	pipeline, err := findPipeline(pullRequestURL, commitSHA, gitlab, poller)
	if err != nil {
		return nil, "", err
	}
	log.Printf("found %s (status: %s)\n", pipeline.WebURL, pipeline.Status)

	if !canContinue[pipeline.Status] {
		return nil, "", fmt.Errorf("cannot continue with pipeline status: '%s'", pipeline.Status)
	}

	log.Println("looking for 'build-linux-stable' job...")
	buildJob, err := findBuildJobInPipeline(pipeline.ID, gitlab)
	if err != nil {
		return nil, "", err
	}
	log.Printf("found %s (status: %s)\n", buildJob.WebURL, buildJob.Status)

	switch buildJob.Status {
	case "success":
		break
	case "failed":
		return nil, "", errors.New("'build-linux-stable' job failed")
	case "created":
		fallthrough
	case "waiting_for_resource":
		fallthrough
	case "preparing":
		fallthrough
	case "pending":
		fallthrough
	case "running":
		if buildJob, err = pollJob(buildJob, gitlab, poller); err != nil {
			return nil, "", err
		}
	}

	if buildJob.Status == "manual" || buildJob.Status == "canceled" || buildJob.Status == "skipped" {
		log.Println("starting job...")
		if err = gitlab.StartJob(buildJob.ID); err != nil {
			return nil, "", fmt.Errorf("failed to start 'build-linux-stable' job: %v", err)
		}
		log.Println("'build-linux-stable' job started")
		if buildJob, err = pollJob(buildJob, gitlab, poller); err != nil {
			return nil, "", err
		}
	}

	webURL, err := url.Parse(buildJob.WebURL)
	if err != nil {
		return nil, "", err
	}

	binaryURL, err := AddPathsToURL(webURL, "artifacts/raw/artifacts/polkadot")
	if err != nil {
		return nil, "", err
	}

	return binaryURL, pipeline.SHA, nil
}

func findPipeline(
	pullRequestURL *url.URL,
	commitSHA string,
	gitlab burnin.Gitlab,
	poller burnin.Poller,
) (burnin.Pipeline, error) {
	if commitSHA != "" {
		log.Printf("searching for CI pipeline for commit '%s'\n", commitSHA)
		return findPipelineForCommit(commitSHA, gitlab, poller)
	}

	pathParts := strings.Split(pullRequestURL.Path, "/")
	prID := pathParts[len(pathParts)-1]
	branchURL, err := gitlab.WebURLForBranch(prID)
	if err != nil {
		return burnin.Pipeline{}, err
	}

	log.Printf("searching for CI pipeline for branch '%s' (%s)\n", prID, branchURL.String())
	return findPipelineForPullRequest(prID, gitlab, poller)
}

func findPipelineForCommit(
	commitSHA string,
	gitlab burnin.Gitlab,
	poller burnin.Poller,
) (burnin.Pipeline, error) {
	var pipeline burnin.Pipeline
	var err error

	pollErr := poller.Poll(5*time.Minute, "nonexistent", func() (string, error) {
		pipeline, err = gitlab.GetPipelineForCommit(commitSHA)
		if err == burnin.ErrPipelineNotFound {
			return "nonexistent", nil
		}

		if err != nil {
			return pipeline.Status, err
		}

		return "", err
	}, "nonexistent")

	return pipeline, pollErr
}

func findPipelineForPullRequest(
	pullRequestID string,
	gitlab burnin.Gitlab,
	poller burnin.Poller,
) (burnin.Pipeline, error) {
	var pipeline burnin.Pipeline

	pollErr := poller.Poll(5*time.Minute, "nonexistent", func() (string, error) {
		pipelines, err := gitlab.GetPipelinesForBranch(pullRequestID)
		if err != nil {
			return "", err
		}

		if len(pipelines) > 0 {
			pipeline = pipelines[0] // if there is more than one, they should be ordered by 'updated_at'
			return pipeline.Status, nil
		}

		return "nonexistent", nil
	}, "nonexistent")

	return pipeline, pollErr
}

func findBuildJobInPipeline(pipelineID int, gitlab burnin.Gitlab) (burnin.Job, error) {
	jobs, err := gitlab.GetPipelineJobs(pipelineID)
	if err != nil {
		return burnin.Job{}, err
	}

	for _, j := range jobs {
		if j.Name == "build-linux-stable" {
			return j, nil
		}
	}

	return burnin.Job{}, fmt.Errorf("no job named 'build-linux-stable' found in pipeline '%d'", pipelineID)
}

func pollJob(job burnin.Job, gitlab burnin.Gitlab, poller burnin.Poller) (burnin.Job, error) {
	endedStatus := map[string]bool{
		"succeess": true,
		"failed":   true,
		"canceled": true,
		"skipped":  true,
	}

	if endedStatus[job.Status] {
		return job, nil
	}

	pollErr := poller.Poll(45*time.Minute, job.Status, func() (string, error) {
		var err error
		job, err = gitlab.GetJob(job.ID)
		if err != nil {
			return "", err
		}

		log.Printf("polling job %d (status: %s)\n", job.ID, job.Status)
		return job.Status, nil
	}, "running")

	return job, pollErr
}
