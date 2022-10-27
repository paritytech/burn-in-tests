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
package gitlab

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebURLHelpers(t *testing.T) {
	serverURL, err := url.Parse("https://gitlab.example.com")
	require.Nil(t, err)

	client := &Client{
		serverURL: serverURL,
		project: Project{
			ID:                42,
			Name:              "mockproject",
			PathWithNamespace: "mocks/mockproject",
		},
	}

	branchURL, err := client.WebURLForBranch("master")
	require.Nil(t, err)
	require.Equal(t, "https://gitlab.example.com/mocks/mockproject/-/tree/master", branchURL.String())

	jobURL, err := client.WebURLForJob(23)
	require.Nil(t, err)
	require.Equal(t, "https://gitlab.example.com/mocks/mockproject/-/jobs/23", jobURL.String())
}
