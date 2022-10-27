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

function buildDeployedAt() {
    const today = new Date();
    const month = String(today.getMonth()).padStart(2, "0");
    const day = String(today.getDay()).padStart(2, "0");
    return `${today.getFullYear()}-${month}-${day}`;
}

function ManualForm({onSubmitManualRun}) {

    const [deployedOn, setDeployedOn] = useState("");
    const [deployedAt, setDeployedAt] = useState(buildDeployedAt());
    const [network, setNetwork] = useState("");
    const [branch, setBranch] = useState("");
    const [requestedBy, setRequestedBy] = useState("");
    const [comment, setComment] = useState("");
    const [submitDisabled, setSubmitDisabled] = useState(false);

    const resetForm = () => {
        setDeployedOn("");
        setDeployedAt(buildDeployedAt());
        setNetwork("");
        setBranch("");
        setRequestedBy("");
        setComment("");
        setSubmitDisabled(false);
    }

    const onSubmit = (e) => {
        e.preventDefault();
        setSubmitDisabled(true);

        onSubmitManualRun(deployedOn, deployedAt, network, branch, requestedBy, comment, resetForm);
    }

    return (
        <form onSubmit={onSubmit}>
            <div className="form-group">
                <label>Deployed On</label>
                <input
                    type="text"
                    disabled={submitDisabled}
                    value={deployedOn}
                    onChange={(e) => setDeployedOn(e.target.value)}
                />
            </div>
            <div className="form-group">
                <label>Deployed At</label>
                <input
                    type="text"
                    disabled={submitDisabled}
                    value={deployedAt}
                    onChange={(e) => setDeployedAt(e.target.value)}
                />
            </div>
            <div className="form-group">
                <label>Network</label>
                <input
                    type="text"
                    disabled={submitDisabled}
                    value={network}
                    onChange={(e) => setNetwork(e.target.value)}
                />
            </div>
            <div className="form-group">
                <label>Branch/Pull Request</label>
                <input
                    type="text"
                    disabled={submitDisabled}
                    value={branch}
                    onChange={(e) => setBranch(e.target.value)}
                />
            </div>

            <div className="form-group">
                <label>Requested By</label>
                <input
                    type="text"
                    disabled={submitDisabled}
                    value={requestedBy}
                    onChange={(e) => setRequestedBy(e.target.value)}
                />
            </div>

            <div className="form-group">
                <label>Comment</label>
                <input
                    type="text"
                    disabled={submitDisabled}
                    value={comment}
                    onChange={(e) => setComment(e.target.value)}
                />
            </div>

            <input type="submit" value="Submit" className="button" disabled={submitDisabled}/>
        </form>
    )
}

export default ManualForm;