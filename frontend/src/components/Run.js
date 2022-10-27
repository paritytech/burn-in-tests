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
function Run({run, primary, onRemoveRun}) {
    const {path, content} = run;

    if (!content.pull_request) {
        return <></>;
    }

    const handleRemoveClick = (e) => {
        if (!window.confirm("Are you sure?")) {
            return;
        }

        e.target.disabled = true;
        onRemoveRun(path, content.deployed_on);
    }

    let commitSHA = "";
    if (content.commit_sha) {
        const commitURL = `https://github.com/paritytech/polkadot/tree/${content.commit_sha}`;
        commitSHA = <code><a href={commitURL}>{content.commit_sha.substring(0, 7)}</a></code>;
    }

    let pullRequest = content.pull_request;
    if (pullRequest.startsWith("https://github.com/paritytech/polkadot/pull/")) {
        const prNum = pullRequest.substring(pullRequest.lastIndexOf("/") + 1, pullRequest.length);
        pullRequest = <a href={content.pull_request}>polkadot#{prNum}</a>;
    }

    const deployedOn = content.deployed_on ? <code>{content.deployed_on}</code> : "<pending>";
    const deployedAt = content.deployed_at ? content.deployed_at.toUTCString() : "<pending>";
    const updatedAt = content.updated_at ? content.updated_at.toUTCString() : "<never updated>";
    const links = buildLinks(content);
    const customOptions = buildCustomOptions(content);

    return (
        <tr className={primary ? "primary-row" : "secondary-row"}>
            <td>{deployedOn}</td>
            <td>{content.network}</td>
            <td>{pullRequest}</td>
            <td style={{textAlign: "center"}}>{commitSHA}</td>
            <td style={{textAlign: "center"}}>{content.requested_by}</td>
            <td style={{textAlign: "center"}}>{deployedAt}</td>
            <td style={{textAlign: "center"}}>{updatedAt}</td>
            <td style={{textAlign: "center"}}>{content.sync_from_scratch.toString()}</td>
            <td>
                <ul>{customOptions}</ul>
            </td>
            <td>
                <ul>{links}</ul>
            </td>

            {(() => {
                if (onRemoveRun) {
                    return <td style={{textAlign: "center"}}>
                        <button
                            style={{color: "white", backgroundColor: "red"}}
                            onClick={handleRemoveClick}
                        >X
                        </button>
                    </td>
                }
            })()}

        </tr>
    )
}

function buildLinks(run) {
    const links = [];
    let key = 0;

    if (run.log_viewer) {
        links.push(<li key={key}><a href={run.log_viewer}>Logs</a></li>);
    }

    if (run.dashboards) {
        const db = run.dashboards;
        for (let dbName in db) {
            key++;
            links.push(<li key={key}><a href={db[dbName]}>{dbName.replaceAll("_", "-")} Dashboard</a></li>);
        }
    }

    if (run.custom_binary) {
        key++;
        links.push(<li key={key}><a href={run.custom_binary}>Client Binary</a></li>);
    }

    return links;
}

function buildCustomOptions(run) {
    if (!run['custom_options'])
        return [];

    const options = [];
    let key = 0;

    run.custom_options.forEach(opt => {
        key++;
        options.push(<li key={key}><code>{opt}</code></li>);
    });

    return options;
}

export default Run;
