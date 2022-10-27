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
import ManualRun from "./ManualRun";
import {isAdmin} from "../oauth";

function ManualRuns({runs, onRemoveRun}) {
    return (
        <table>
            <thead>
            <tr>
                <th>Deployed On</th>
                <th>Deployed At</th>
                <th>Network</th>
                <th>Branch/Pull Request</th>
                <th>Requested By</th>
                <th>Comment</th>

                {(() => {
                    if (onRemoveRun) {
                        return <th>Remove</th>
                    }
                })()}
            </tr>
            </thead>
            <tbody>
            {runs.map((run, index) => (
                <ManualRun key={index} run={run} primary={index % 2 === 0} onRemoveRun={onRemoveRun}/>
            ))}
            </tbody>
        </table>
    );
}

export default ManualRuns;