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
	"log"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	burnin "gitlab.example.com/burn-in-tests/backend"
)

const polkadotNightlyBuildURL = "https://releases.example.com/builds/polkadot/x86_64-debian:stretch/master/polkadot"

// AddPathsToURL takes "https://gitlab.example.com/foo/bar?spam=eggs" and "more", "path", "segments" and returns
// "https://gitlab.example.com/foo/bar/more/path/segments?spam=eggs".
// The function is public and in the "job" package because it is used by job.buildPolkadotBinary() as well as in
// gitlab.Client. Importing from the "business logic" package "job" is the lesser evil compared to importing from the
// "gitlab" package, which is mocked out in tests.
func AddPathsToURL(u *url.URL, paths ...string) (*url.URL, error) {
	count := len(paths) + 1
	parts := make([]string, count, count)
	parts[0] = u.Path
	for i, p := range paths {
		parts[i+1] = p
	}

	fullPath := path.Join(parts...)
	fpURL, err := url.Parse(fullPath)
	if err != nil {
		return nil, err
	}

	rawQuery := u.RawQuery
	u = u.ResolveReference(fpURL)
	u.RawQuery = rawQuery
	return u, nil
}

func createSilence(alertmanager burnin.Alertmanager, targetHostname, comment string) (string, error) {
	matchers := []burnin.AlertMatcher{
		{
			Name:    "instance",
			Value:   fmt.Sprintf(".*%s.*", targetHostname),
			IsRegex: true,
		},
	}

	startsAt := time.Now()
	endsAt := startsAt.Add(time.Minute * 5)
	return alertmanager.CreateSilence(matchers, startsAt, endsAt, "Burn-in Automator", comment)
}

func diffsToCurrentCommit(baseBranch string, gitlab burnin.Gitlab) ([]burnin.CommitDiff, error) {
	ref := baseBranch
	currentCommit := os.Getenv("CI_COMMIT_SHA")

	if currentCommit != "" {
		ref = currentCommit
		log.Printf("fetching most recent commit diffs for branch '%s' (commit: '%s')\n", baseBranch, currentCommit)
	} else {
		log.Printf("fetching most recent commit diffs for branch '%s'\n", baseBranch)
	}

	return gitlab.GetLastCommitDiffs(ref)
}

func hostnameToFQDNs(hostname string) (publicFQDN string, internalFQDN string) {
	subdomainSuffix := "chains"
	if strings.Contains(hostname, "westend") {
		subdomainSuffix = "testnet"
	}

	return fmt.Sprintf("%s.foo-%s.example.com", hostname, subdomainSuffix),
		fmt.Sprintf("%s-int.foo-%s.example.com", hostname, subdomainSuffix)
}
