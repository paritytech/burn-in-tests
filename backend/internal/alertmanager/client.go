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
package alertmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	burnin "gitlab.example.com/burn-in-tests/backend"
)

type Client struct {
	apiURL     *url.URL
	httpClient *http.Client
}

func NewClient(apiURL *url.URL) *Client {
	return &Client{
		apiURL: apiURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) CreateSilence(
	matchers []burnin.AlertMatcher,
	startsAt time.Time,
	endsAt time.Time,
	createdBy string,
	comment string,
) (string, error) {
	payload := struct {
		Matchers  []burnin.AlertMatcher `json:"matchers"`
		StartsAt  time.Time             `json:"startsAt"`
		EndsAt    time.Time             `json:"endsAt"`
		CreatedBy string                `json:"createdBy"`
		Comment   string                `json:"comment"`
	}{
		Matchers:  matchers,
		StartsAt:  startsAt,
		EndsAt:    endsAt,
		CreatedBy: createdBy,
		Comment:   comment,
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	u := fmt.Sprintf("%s/silences", c.apiURL.String())
	request, err := http.NewRequest(http.MethodPost, u, bytes.NewBuffer(buf))
	if err != nil {
		return "", err
	}

	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if err := errorIfNotOK(request, buf, response, false); err != nil {
		return "", err
	}

	createSilenceResponse := new(struct {
		SilenceID string `json:"silenceID"`
	})

	if err := json.NewDecoder(response.Body).Decode(createSilenceResponse); err != nil {
		return "", err
	}

	return createSilenceResponse.SilenceID, nil
}

func (c *Client) DeleteSilence(id string) error {
	u := fmt.Sprintf("%s/silence/%s", c.apiURL.String(), id)

	request, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return err
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	return errorIfNotOK(request, nil, response, true)
}

func errorIfNotOK(request *http.Request, payload []byte, response *http.Response, closeResponse bool) error {
	if closeResponse {
		defer response.Body.Close()
	}

	if response.StatusCode == http.StatusOK {
		return nil
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	return fmt.Errorf(`HTTP request to Alertmanager API failed.
Request: %s %s
Body: %s

Response: %s
Body: %s`, request.Method, request.URL.String(), payload, response.Status, string(responseBody))
}
