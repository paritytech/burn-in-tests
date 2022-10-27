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
import toml from "toml";

import {getCurrentUser} from "./oauth";

export const GITLAB_URL = "https://gitlab.example.com";

const API_PATH = "/api/v4/projects/burn-in-tests%2Fdeployments/repository";
const BRANCH = "master";
const READONLY_TOKEN = "REDACTED";

export async function fetchRuns() {
    return fetchTOMLFiles("runs");
}

export async function fetchManualRuns() {
    return fetchTOMLFiles("manual");
}

export async function commitRequestFile(
    pullRequest,
    commitSHA,
    customBinary,
    requestedBy,
    syncFromScratch,
    customOptions,
    nodes,
    onSuccess,
) {
    const user = getCurrentUser();

    let lines = [
        `pull_request = "${pullRequest}"`,
    ];

    if (commitSHA) {
        lines.push(`commit_sha = "${commitSHA}"`);
    }

    if (customBinary) {
        lines.push(`custom_binary = "${customBinary}"`);
    }

    if (customOptions) {
        const quotedOptions = customOptions.map(o => `"${o}"`);
        lines.push(`custom_options = [${quotedOptions.join(", ")}]`);
    }

    lines = lines.concat([
        `requested_by = "${requestedBy || user.email}"`,
        `sync_from_scratch = ${syncFromScratch}`,
        "",
    ]);

    lines = lines.concat(buildNodeSections(nodes));

    const u = new URL(GITLAB_URL);
    u.pathname = `${API_PATH}/files/${encodeURIComponent(`requests/request-${timestamp()}.toml`)}`;

    const body = {
        branch: BRANCH,
        commit_message: `Request ${pullRequest}`,
        author_name: user.name,
        author_email: user.email,
        content: lines.join("\n")
    }

    const response = await fetch(u.toString(), buildFetchOpts("POST", body));
    if (!response.ok) {
        console.error(`POST ${u.toString()} failed with ${response.status} ${await response.text()}`);
        alert("Submitting your request failed. Please contact someone from Staking Ops.");
    } else {
        onSuccess();
    }
}

export async function commitManualRunFile(deployedOn, deployedAt, network, branch, requestedBy, comment, onSuccess) {
    const user = getCurrentUser();

    const content = `deployed_on = "${deployedOn}"
deployed_at = "${deployedAt}"
network = "${network}"
branch = "${branch}"
requested_by = "${requestedBy}"
comment = "${comment}"
`;

    const u = new URL(GITLAB_URL);
    u.pathname = `${API_PATH}/files/${encodeURIComponent(`manual/run-${timestamp()}.toml`)}`;

    const body = {
        branch: BRANCH,
        commit_message: `Add manual deployment on ${deployedOn}`,
        author_name: user.name,
        author_email: user.email,
        content: content
    }

    const response = await fetch(u.toString(), buildFetchOpts("POST", body));
    if (!response.ok) {
        console.error(`POST ${u.toString()} failed with ${response.status} ${await response.text()}`);
        alert("Submitting your manual deployment failed, sorry.");
    } else {
        onSuccess();
    }
}

export async function removeRun(path, deployedOn) {
    const user = getCurrentUser();
    const u = new URL(GITLAB_URL);
    u.pathname = `${API_PATH}/files/${encodeURIComponent(path)}`;

    let msg = `[cleanup] ${deployedOn}`;
    if (path.startsWith("manual")) {
        msg = `Remove manual deployment on ${deployedOn}`;
    }

    const body = {
        "branch": BRANCH,
        commit_message: msg,
        author_name: user.name,
        author_email: user.email
    }

    const response = await fetch(u.toString(), buildFetchOpts("DELETE", body));

    if (!response.ok) {
        console.error(`DELETE ${u.toString()} failed with ${response.status} ${await response.text()}`);
        alert("Deleting the file failed.");
    }
}

export function buildFetchOpts(method, body) {
    let opts = {
        method: method || "GET",
        headers: {
            "Content-Type": "application/json"
        }
    }

    if (body) {
        opts["body"] = JSON.stringify(body);
    }

    const user = getCurrentUser();
    opts["headers"]["Authorization"] = user ? `Bearer ${user.accessToken}` : `Bearer ${READONLY_TOKEN}`;

    return opts;
}

function buildNodeSections(nodes) {
    const lines = [];

    for (let network in nodes) {
        if (!nodes.hasOwnProperty(network)) {
            continue;
        }

        lines.push(`[nodes.${network}]`);

        for (let nodeType in nodes[network]) {
            if (!nodes[network].hasOwnProperty(nodeType)) {
                continue;
            }

            lines.push(`  ${nodeType} = ${nodes[network][nodeType]}`);
        }
    }

    return lines;
}

async function fetchTOMLFiles(path) {
    const paths = await listFilesInPath(path);

    return await Promise.all(paths.map(p => fetchFile(p).then(
        f => {
            return {path: p, content: toml.parse(f)}
        })));
}

async function listFilesInPath(path) {
    const u = new URL(GITLAB_URL);
    u.pathname = `${API_PATH}/tree`;
    u.searchParams.append("ref", BRANCH);
    u.searchParams.append("path", path);
    u.searchParams.append("per_page", "100");

    const response = await fetch(u.toString(), buildFetchOpts());

    if (!response.ok) {
        console.error(`GET ${u.toString()} failed with ${response.status} ${await response.text()}`);
        return Promise.resolve([]);
    }

    return (await response.json())
        .filter(f => f.name.startsWith("run-") && f.name.endsWith(".toml"))
        .map(f => `${path}/${f.name}`);
}

async function fetchFile(path) {
    const u = new URL(GITLAB_URL);
    u.pathname = `${API_PATH}/files/${encodeURIComponent(path)}/raw`;
    u.searchParams.append("ref", BRANCH);

    const response = await fetch(u.toString(), buildFetchOpts());

    if (!response.ok) {
        console.error(`GET ${u.toString()} failed with ${response.status} ${await response.text()}`);
        return Promise.resolve("");
    }

    return (await response.text());
}

function timestamp() {
    return Math.round(Date.now() / 1000);
}
