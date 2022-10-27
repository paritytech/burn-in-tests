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
import {useState} from "react";

function RequestForm({onNewRequest}) {
    const [pullRequest, setPullRequest] = useState("");
    const [commitSHA, setCommitSHA] = useState("");
    const [customBinary, setCustomBinary] = useState("");
    const [requestedBy, setRequestedBy] = useState("");
    const [syncFromScratch, setSyncFromScratch] = useState(false);
    const [customOptions, setCustomOptions] = useState([]);
    const [kusamaNode, setKusamaNode] = useState(false);
    const [polkadotNode, setPolkadotNode] = useState(false);
    const [westendNodes, setWestendNodes] = useState(0);
    const [submitDisabled, setSubmitDisabled] = useState(false);

    const resetForm = () => {
        setPullRequest("");
        setCommitSHA("");
        setCustomBinary("");
        setRequestedBy("");
        setSyncFromScratch(false);
        setCustomOptions([]);
        setSubmitDisabled(false);
        setKusamaNode(false);
        setPolkadotNode(false);
        setWestendNodes(0);
    }

    const onSubmit = (e) => {
        e.preventDefault();
        setSubmitDisabled(true);

        // TODO more validation
        if (pullRequest === "" && customBinary === "") {
            setSubmitDisabled(false);
            alert("You need to provide at least a pull request URL or a custom binary URL.");
            return;
        }

        if (!kusamaNode && !polkadotNode && !westendNodes) {
            setSubmitDisabled(false);
            alert("You need to select at least one target network (Kusama, Polkadot or Westend).");
            return;
        }

        const nodes = kusamaNode ? {"kusama": {"fullnode": 1}} : {};
        if (polkadotNode) {
            nodes["polkadot"] = {"fullnode": 1}
        }
        if (westendNodes > 0) {
            nodes["westend"] = {"validator": westendNodes}
        }

        onNewRequest(pullRequest, commitSHA, customBinary, requestedBy, syncFromScratch, customOptions, nodes, resetForm);
    }

    return (
        <form onSubmit={onSubmit}>
            <div className="form-group">
                <label>Pull Request URL</label>
                <input
                    type="text"
                    disabled={submitDisabled}
                    placeholder='https://github.com/paritytech/polkadot/pull/2098 (can be a "fake" PR URL if "Custom Binary URL" is given)'
                    value={pullRequest}
                    onChange={(e) => setPullRequest(e.target.value)}
                />
            </div>
            <div className="form-group">
                <label>Commit SHA</label>
                <input
                    type="text"
                    disabled={submitDisabled}
                    placeholder="optional (defaults to HEAD on the PR branch)"
                    value={commitSHA}
                    onChange={(e) => setCommitSHA(e.target.value)}
                />
            </div>
            <div className="form-group">
                <label>Custom Binary URL</label>
                <input
                    type="text"
                    disabled={submitDisabled}
                    placeholder="(deploys the linked binary instead of trying to build/download an artifact on Gitlab)"
                    value={customBinary}
                    onChange={(e) => setCustomBinary(e.target.value)}
                />
            </div>
            <div className="form-group">
                <label>Custom Options (one flag per line)</label>
                <textarea
                    disabled={submitDisabled}
                    placeholder='(e.g. "--wasm-execution Compiled")'
                    value={customOptions.join("\n")}
                    onChange={(e) => setCustomOptions(e.target.value.split("\n"))}
                />
            </div>
            <div className="form-group">
                <label>Requested By</label>
                <input
                    type="text"
                    disabled={submitDisabled}
                    placeholder="optional, arbitrary string (defaults to email of logged in user)"
                    value={requestedBy}
                    onChange={(e) => setRequestedBy(e.target.value)}
                />
            </div>
            <div className="form-group">
                <label>Sync from Scratch (only applies to full nodes)</label>
                <input
                    type="checkbox"
                    disabled={submitDisabled}
                    checked={syncFromScratch}
                    value={syncFromScratch}
                    onChange={(e) => setSyncFromScratch(e.currentTarget.checked)}
                />
            </div>

            <div className="form-group">
                <label>Run on this many Westend Validators</label>
                <input
                    type="number"
                    disabled={submitDisabled}
                    value={westendNodes}
                    min="0"
                    step="0"
                    onChange={(e) => setWestendNodes(e.currentTarget.value)}
                />
            </div>

            <div className="form-group">
                <label>Run on a Kusama Full Node</label>
                <input
                    type="checkbox"
                    disabled={submitDisabled}
                    checked={kusamaNode}
                    value={kusamaNode}
                    onChange={(e) => setKusamaNode(e.currentTarget.checked)}
                />
            </div>

            <div className="form-group">
                <label>Run on a Polkadot Full Node</label>
                <input
                    type="checkbox"
                    disabled={submitDisabled}
                    checked={polkadotNode}
                    value={polkadotNode}
                    onChange={(e) => setPolkadotNode(e.currentTarget.checked)}
                />
            </div>

            <input type="submit" value="Submit" className="button" disabled={submitDisabled}/>
        </form>
    )
}

export default RequestForm;