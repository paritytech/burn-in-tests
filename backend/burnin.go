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
package burnin

import (
	"errors"
	"net/url"
	"time"
)

type NodeType string

const (
	FullNode  NodeType = "fullnode"
	Sentry             = "sentry"
	Validator          = "validator"
)

var ErrPipelineNotFound = errors.New("no matching pipeline found")

type NodesPerNetworkMap map[string]map[NodeType]int

type Request struct {
	PullRequest     string             `toml:"pull_request"`             // e.g. https://github.com/paritytech/polkadot/pull/2013
	CommitSHA       string             `toml:"commit_sha"`               // optional, only considered if 'custom_binary' is not provided
	CustomBinary    *string            `toml:"custom_binary,omitempty"`  // optional URL to the polkadot binary, usually on gitlab.example.com
	CustomOptions   []string           `toml:"custom_options,omitempty"` // optional custom CLI flags to pass to Ansible
	RequestedBy     string             `toml:"requested_by"`             // github/matrix handle or email address
	SyncFromScratch bool               `toml:"sync_from_scratch"`        // if true, chain db will be deleted before updating the binary
	Nodes           NodesPerNetworkMap `toml:"nodes"`                    // e.g. m["kusama"][FullNode] = 2, m["polkadot"][Validator] = 1
}

type Deployment struct {
	PullRequest     string            `toml:"pull_request"`
	CommitSHA       string            `toml:"commit_sha"`
	CustomBinary    string            `toml:"custom_binary"`
	CustomOptions   []string          `toml:"custom_options,omitempty"`
	RequestedBy     string            `toml:"requested_by"`
	SyncFromScratch bool              `toml:"sync_from_scratch"`
	Network         string            `toml:"network"`
	NodeType        NodeType          `toml:"node_type"`
	DeployedAt      time.Time         `toml:"deployed_at,omitempty"`
	UpdatedAt       time.Time         `toml:"updated_at,omitempty"`
	DeployedOn      string            `toml:"deployed_on,omitempty"`
	PublicFQDN      string            `toml:"public_fqdn,omitempty"`
	InternalFQDN    string            `toml:"internal_fqdn,omitempty"`
	LogViewer       string            `toml:"log_viewer,omitempty"`
	Dashboards      map[string]string `toml:"dashboards,omitempty"`

	Filename string `toml:"-"` // name of the "run" file
}

type Gitlab interface {
	GetLastCommitDiffs(branch string) ([]CommitDiff, error)
	GetPipelinesForBranch(branch string) ([]Pipeline, error)
	GetPipelineForCommit(sha string) (Pipeline, error)
	GetPipeline(pipelineID int) (Pipeline, error)
	GetPipelineJobs(pipelineID int) ([]Job, error)
	GetJob(jobID int) (Job, error)
	StartJob(jobID int) error
	CreateBranch(name, fromBranch string) error
	ListDirectory(path, branch string) ([]FileInfo, error)
	CreateFile(path, branch, commitMsg string, content []byte) error
	UpdateFile(path, branch, commitMsg string, content []byte) error
	DeleteFile(path, branch, commitMsg string) error
	CreateMergeRequest(title, sourceBranch, targetBranch string) (MergeRequest, error)
	GetRunners() ([]Runner, error)
	GetRunnerTags(id int) ([]string, error)
	PauseRunner(hostname string) error
	UnPauseRunner(hostname string) error
	WebURLForBranch(branch string) (*url.URL, error)
	WebURLForJob(id int) (*url.URL, error)
	PrefixSkipCI(string) string
	PrefixDeploy(network string, n NodeType, msg string) string
	PrefixUpdateDeployment(msg string) string
	PrefixCleanup(msg string) string
}

type Pipeline struct {
	ID        int    `json:"id"`
	Status    string `json:"status"`
	Ref       string `json:"ref"`
	SHA       string `json:"sha"`
	WebURL    string `json:"web_url"`
	CreatedAt string `json:"created_at"`
}

type Job struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	WebURL    string `json:"web_url"`
	CreatedAt string `json:"created_at"`
}

type MergeRequest struct {
	ID        int    `json:"id"`
	ProjectID int    `json:"project_id"`
	WebURL    string `json:"web_url"`
}

type CommitDiff struct {
	Diff        string  `json:"diff"`
	NewPath     *string `json:"new_path"`
	OldPath     *string `json:"old_path"`
	AMode       *string `json:"a_mode"`
	BMode       *string `json:"b_mode"`
	NewFile     bool    `json:"new_file"`
	RenamedFile bool    `json:"renamed_file"`
	DeletedFile bool    `json:"deleted_file"`
}

type FileInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
	Mode string `json:"mode"`
}

type Runner struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
	IPAddress   string `json:"ip_address"`
	IsShared    bool   `json:"is_shared"`
	Online      bool   `json:"online"`
	Status      string `json:"status"`
}

type AlertMatcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
}

type Alertmanager interface {
	CreateSilence(matchers []AlertMatcher, startsAt, endsAt time.Time, createdBy, comment string) (string, error)
	DeleteSilence(id string) error
}

type AnsibleDriver interface {
	RunPlaybook(
		name string,
		runOn string,
		nodeBinary *url.URL,
		wipeChainDB bool,
		nodePublicName string,
		customOptions []string,
	) error
}

// Poller only exists to avoid time.Sleep() calls in tests.
type Poller interface {
	Poll(
		timeout time.Duration,
		initialStatus string,
		updateStatus func() (string, error),
		additionalWaitStatus ...string,
	) error
}

type Matrix interface {
	SendRequestNotification(request Request) error
	SendDeploymentNotification(deployment Deployment) error
	SendUpdateNotification(deployment Deployment) error
	SendCleanupNotification(deployment Deployment) error
	SendErrorNotification(err error) error
}
