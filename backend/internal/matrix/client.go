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
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	burnin "gitlab.example.com/burn-in-tests/backend"
	"gitlab.example.com/burn-in-tests/backend/internal/job"
)

type Client struct {
	homeserverURL *url.URL
	roomID        string
	accessToken   string
	ciJobURL      *url.URL
	httpClient    *http.Client
}

func NewClient(
	homeserverURL *url.URL,
	roomID string,
	accessToken string,
	ciJobURL *url.URL,
) *Client {
	return &Client{
		homeserverURL: homeserverURL,
		roomID:        roomID,
		accessToken:   accessToken,
		ciJobURL:      ciJobURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) Authenticate() error {
	syncURL, err := job.AddPathsToURL(c.homeserverURL, "/_matrix/client/r0/sync")
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodHead, syncURL.String(), nil)
	if err != nil {
		return err
	}

	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}

	return errorIfNot(http.StatusOK, request, nil, response, true)
}

func (c *Client) SendRequestNotification(request burnin.Request) error {
	vars := tmplVars{
		Request:     request,
		JobURL:      template.URL(c.ciJobURL.String()),
		PullRequest: formatPullRequest(request.PullRequest),
	}

	return c.sendHTMLMessage(requestTmpl, vars)
}

func (c *Client) SendDeploymentNotification(deployment burnin.Deployment) error {
	vars := tmplVars{
		Deployment:   deployment,
		JobURL:       template.URL(c.ciJobURL.String()),
		PullRequest:  formatPullRequest(deployment.PullRequest),
		DashboardURL: template.URL(deployment.Dashboards["substrate_networking"]),
	}

	vars.CommitURL = buildCommitURL(deployment.CommitSHA, deployment.PullRequest)
	return c.sendHTMLMessage(deployTmpl, vars)
}

func (c *Client) SendUpdateNotification(deployment burnin.Deployment) error {
	vars := tmplVars{
		Deployment:   deployment,
		JobURL:       template.URL(c.ciJobURL.String()),
		PullRequest:  formatPullRequest(deployment.PullRequest),
		DashboardURL: template.URL(deployment.Dashboards["substrate_networking"]),
	}

	vars.CommitURL = buildCommitURL(deployment.CommitSHA, deployment.PullRequest)

	return c.sendHTMLMessage(updateTmpl, vars)
}

func (c *Client) SendCleanupNotification(deployment burnin.Deployment) error {
	vars := tmplVars{
		Deployment:  deployment,
		JobURL:      template.URL(c.ciJobURL.String()),
		PullRequest: formatPullRequest(deployment.PullRequest),
	}

	return c.sendHTMLMessage(cleanupTmpl, vars)
}

func (c *Client) SendErrorNotification(err error) error {
	vars := tmplVars{
		Error:  err,
		JobURL: template.URL(c.ciJobURL.String()),
	}

	return c.sendHTMLMessage(errorTmpl, vars)
}

func (c *Client) sendHTMLMessage(tmpl *template.Template, vars tmplVars) error {
	formattedBody := new(bytes.Buffer)
	if err := tmpl.Execute(formattedBody, vars); err != nil {
		return err
	}

	u, err := job.AddPathsToURL(c.homeserverURL, "/_matrix/client/r0/rooms/", c.roomID, "/send/m.room.message")
	if err != nil {
		return err
	}

	payload := struct {
		MsgType       string `json:"msgtype"`
		Format        string `json:"format"`
		Body          string `json:"body"`
		FormattedBody string `json:"formatted_body"`
	}{
		MsgType:       "m.text",
		Format:        "org.matrix.custom.html",
		Body:          "",
		FormattedBody: string(formattedBody.Bytes()),
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(buf))
	if err != nil {
		return err
	}

	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}

	return errorIfNot(http.StatusOK, request, buf, response, true)
}

const polkadotRepoURL = "https://github.com/paritytech/polkadot"

func formatPullRequest(pr string) string {
	if strings.HasPrefix(pr, polkadotRepoURL) {
		pr = fmt.Sprintf("polkadot#%s", strings.Replace(pr, polkadotRepoURL+"/pull/", "", 1))
	}

	return pr
}

func buildCommitURL(commitSHA string, pr string) template.URL {
	if commitSHA == "" || !strings.HasPrefix(pr, polkadotRepoURL) {
		return ""
	}

	return template.URL(fmt.Sprintf("%s/tree/%s", polkadotRepoURL, commitSHA))
}

func errorIfNot(status int, request *http.Request, payload []byte, response *http.Response, closeResponse bool) error {
	if closeResponse {
		defer response.Body.Close()
	}

	if response.StatusCode == status {
		return nil
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	return fmt.Errorf(`HTTP request to Matrix API failed.
Request: %s %s
Body: %s

Response: %s
Body: %s`, request.Method, request.URL.String(), payload, response.Status, string(responseBody))
}

type tmplVars struct {
	burnin.Request
	burnin.Deployment
	Error        error
	PullRequest  string
	JobURL       template.URL
	CommitURL    template.URL
	DashboardURL template.URL
}

var (
	requestTmpl = template.Must(template.New("request").Parse(
		`<a href="{{.JobURL}}">Processed burn-in request</a> for <a href="{{.Request.PullRequest}}">{{.PullRequest}}</a>
(requested by {{.Request.RequestedBy}})`))

	deployTmpl = template.Must(template.New("deploy").Parse(
		`<a href="{{.JobURL}}">Deployed burn-in</a> for <a href="{{.Deployment.PullRequest}}">{{.PullRequest}}</a>
(requested by {{.Deployment.RequestedBy}}) on {{.Deployment.DeployedOn}}<br />
<ul>
<li><a href="https://burnins.example.com/">Burn-in Test Overview</a></li>
{{if .CommitURL}}<li>Commit SHA: <a href="{{.CommitURL}}"><code>{{.Deployment.CommitSHA}}</code></a></li>{{end}}
<li><a href="{{.Deployment.CustomBinary}}">Client Binary</a></li>
<li><a href="{{.Deployment.LogViewer}}">Logs</a></li>
<li>{{if .DashboardURL}}<a href="{{.DashboardURL}}">Substrate Networking Dashboard</a></li>{{end}}
</ul>
`))

	updateTmpl = template.Must(template.New("update").Parse(
		`<a href="{{.JobURL}}">Updated burn-in</a> for <a href="{{.Deployment.PullRequest}}">{{.PullRequest}}</a>
(requested by {{.Deployment.RequestedBy}}) on {{.Deployment.DeployedOn}}<br />
<ul>
<li><a href="https://burnins.example.com/">Burn-in Test Overview</a></li>
{{if .CommitURL}}<li>Commit SHA: <a href="{{.CommitURL}}"><code>{{.Deployment.CommitSHA}}</code></a></li>{{end}}
<li><a href="{{.Deployment.CustomBinary}}">Client Binary</a></li>
<li><a href="{{.Deployment.LogViewer}}">Logs</a></li>
<li>{{if .DashboardURL}}<a href="{{.DashboardURL}}">Substrate Networking Dashboard</a></li>{{end}}
</ul>
`))

	cleanupTmpl = template.Must(template.New("cleanup").Parse(
		`<a href="{{.JobURL}}">Removed burn-in</a> for <a href="{{.Deployment.PullRequest}}">{{.PullRequest}}</a>
(requested by {{.Deployment.RequestedBy}}) from {{.Deployment.DeployedOn}}`))

	errorTmpl = template.Must(template.New("error").Parse(
		`<a href="{{.JobURL}}">Burn-in CI job failed</a> with the following error:<br /><pre>{{.Error}}</pre>`))
)
