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
package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path"

	"github.com/caarlos0/env/v6"
	burnin "gitlab.example.com/burn-in-tests/backend"
	"gitlab.example.com/burn-in-tests/backend/internal/alertmanager"
	"gitlab.example.com/burn-in-tests/backend/internal/ansible"
	"gitlab.example.com/burn-in-tests/backend/internal/gitlab"
	"gitlab.example.com/burn-in-tests/backend/internal/job"
	"gitlab.example.com/burn-in-tests/backend/internal/matrix"
)

type config struct {
	GitlabServerURL         *url.URL `env:"CI_SERVER_URL"`
	GitlabProjectID         int      `env:"CI_PROJECT_ID"`
	GitlabToken             string   `env:"GITLAB_TOKEN"`
	GitlabDefaultBranch     string   `env:"CI_COMMIT_BRANCH"`
	GitlabJobID             int      `env:"CI_JOB_ID"`
	PolkadotGitlabProjectID int      `env:"POLKADOT_GITLAB_PROJECT_ID" envDefault:"42"`

	AlertmanagerAPIURL *url.URL `env:"ALERTMANAGER_API_URL" envDefault:"http://alertmanager.example.com/api/v2"`
	BaseDirectory      string   `env:"-"`
	TargetHostname     string   `env:"-"`

	MatrixHomeserverURL *url.URL `env:"MATRIX_HOMESERVER_URL" envDefault:"https://matrix.example.com/"`
	// default room is "Burn-in Monitoring"
	MatrixRoomID      string `env:"MATRIX_ROOM_ID" envDefault:"!someroom:matrix.example.com"`
	MatrixAccessToken string `env:"MATRIX_TOKEN"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	cfg := parseConfig()
	ansiblePath := path.Join(cfg.BaseDirectory, ".maintain", "ansible")

	var cmdErr error
	var matrixClient burnin.Matrix

	switch os.Args[1] {
	case "request":
		matrixClient, cmdErr = cmdRequest(cfg)
	case "deploy":
		matrixClient, cmdErr = cmdDeploy(cfg, ansiblePath)
	case "update":
		matrixClient, cmdErr = cmdUpdate(cfg, ansiblePath)
	case "cleanup":
		matrixClient, cmdErr = cmdCleanup(cfg, ansiblePath)
	case "refresh":
		matrixClient, cmdErr = cmdRefresh(cfg, ansiblePath)
	default:
		usage()
	}

	if cmdErr != nil {
		log.Printf("job '%s' failed: %v\n", os.Args[1], cmdErr)

		if matrixClient != nil {
			if err := matrixClient.SendErrorNotification(cmdErr); err != nil {
				log.Fatalf("sending error notification to matrix failed: %v\n", err)
			}
		}
		os.Exit(1)
	}

	log.Println("done")
}

func cmdRequest(cfg config) (burnin.Matrix, error) {
	burninGitlab := makeGitlabClient(cfg.GitlabServerURL, cfg.GitlabProjectID, cfg.GitlabToken)
	buildGitlab := makeGitlabClient(cfg.GitlabServerURL, cfg.PolkadotGitlabProjectID, cfg.GitlabToken)

	jobURL, err := burninGitlab.WebURLForJob(cfg.GitlabJobID)
	if err != nil {
		return nil, err
	}

	matrixClient := matrix.NewClient(cfg.MatrixHomeserverURL, cfg.MatrixRoomID, cfg.MatrixAccessToken, jobURL)

	return matrixClient, job.ProcessRequest(
		cfg.BaseDirectory,
		cfg.GitlabDefaultBranch,
		burninGitlab,
		buildGitlab,
		job.Poller{},
		matrixClient,
	)
}

func cmdDeploy(cfg config, ansiblePath string) (burnin.Matrix, error) {
	glClient := makeGitlabClient(cfg.GitlabServerURL, cfg.GitlabProjectID, cfg.GitlabToken)
	jobURL, err := glClient.WebURLForJob(cfg.GitlabJobID)
	if err != nil {
		return nil, err
	}

	matrixClient := matrix.NewClient(cfg.MatrixHomeserverURL, cfg.MatrixRoomID, cfg.MatrixAccessToken, jobURL)
	alertmgr := alertmanager.NewClient(cfg.AlertmanagerAPIURL)
	ansibleDriver := ansible.NewDriver(ansiblePath)

	return matrixClient, job.ProcessDeploy(
		cfg.BaseDirectory,
		cfg.GitlabDefaultBranch,
		cfg.TargetHostname,
		glClient,
		alertmgr,
		ansibleDriver,
		matrixClient,
	)
}

func cmdUpdate(cfg config, ansiblePath string) (burnin.Matrix, error) {
	glClient := makeGitlabClient(cfg.GitlabServerURL, cfg.GitlabProjectID, cfg.GitlabToken)
	jobURL, err := glClient.WebURLForJob(cfg.GitlabJobID)
	if err != nil {
		return nil, err
	}

	matrixClient := matrix.NewClient(cfg.MatrixHomeserverURL, cfg.MatrixRoomID, cfg.MatrixAccessToken, jobURL)
	alertmgr := alertmanager.NewClient(cfg.AlertmanagerAPIURL)
	ansibleDriver := ansible.NewDriver(ansiblePath)

	return matrixClient, job.ProcessUpdate(
		cfg.BaseDirectory,
		cfg.GitlabDefaultBranch,
		glClient,
		alertmgr,
		ansibleDriver,
		matrixClient,
	)
}

func cmdCleanup(cfg config, ansiblePath string) (burnin.Matrix, error) {
	glClient := makeGitlabClient(cfg.GitlabServerURL, cfg.GitlabProjectID, cfg.GitlabToken)
	jobURL, err := glClient.WebURLForJob(cfg.GitlabJobID)
	if err != nil {
		return nil, err
	}

	matrixClient := matrix.NewClient(cfg.MatrixHomeserverURL, cfg.MatrixRoomID, cfg.MatrixAccessToken, jobURL)
	alertmgr := alertmanager.NewClient(cfg.AlertmanagerAPIURL)
	ansibleDriver := ansible.NewDriver(ansiblePath)

	return matrixClient, job.ProcessCleanup(
		cfg.GitlabDefaultBranch,
		glClient,
		alertmgr,
		ansibleDriver,
		matrixClient,
	)
}

func cmdRefresh(cfg config, ansiblePath string) (burnin.Matrix, error) {
	glClient := makeGitlabClient(cfg.GitlabServerURL, cfg.GitlabProjectID, cfg.GitlabToken)
	jobURL, err := glClient.WebURLForJob(cfg.GitlabJobID)
	if err != nil {
		return nil, err
	}

	matrixClient := matrix.NewClient(cfg.MatrixHomeserverURL, cfg.MatrixRoomID, cfg.MatrixAccessToken, jobURL)
	alertmgr := alertmanager.NewClient(cfg.AlertmanagerAPIURL)
	ansibleDriver := ansible.NewDriver(ansiblePath)
	return matrixClient, job.ProcessRefresh(glClient, alertmgr, ansibleDriver)
}

func usage() {
	fmt.Printf("usage: %s <request|deploy|update|cleanup|refresh>\n", os.Args[0])
	os.Exit(1)
}

func parseConfig() config {
	var (
		cfg config
		err error
	)

	if err = env.Parse(&cfg); err != nil {
		log.Fatal(err)
	}

	cfg.BaseDirectory, err = os.Getwd()
	if err != nil {
		log.Fatalf("unable to obtain current working directory: %v\n", err)
	}

	if cfg.TargetHostname, err = os.Hostname(); err != nil {
		log.Fatalf("unable to determine hostname: %v\n", err)
	}

	return cfg
}

func makeGitlabClient(url *url.URL, projectID int, token string) burnin.Gitlab {
	glClient, err := gitlab.NewClient(url, projectID, token)
	if err != nil {
		log.Fatalf("creating gitlab client for %s failed: %v\n", url, err)
	}

	if err := glClient.Authenticate(); err != nil {
		log.Fatalf("gitlab auth failed: %v\n", err)
	}

	return glClient
}
