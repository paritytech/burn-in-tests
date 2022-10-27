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
function ManualRun({run, primary, onRemoveRun}) {
    const {path, content} = run;

    const handleRemoveClick = (e) => {
        e.target.disabled = true;
        onRemoveRun(path, content.deployed_on);
    }

    return (
        <tr className={primary ? "primary-row" : "secondary-row"}>
            <td>{content.deployed_on}</td>
            <td style={{textAlign: "center"}}>{content.deployed_at}</td>
            <td style={{textAlign: "center"}}>{content.network}</td>
            <td style={{textAlign: "center"}}>{content.branch}</td>
            <td style={{textAlign: "center"}}>{content.requested_by}</td>
            <td>{content.comment}</td>

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

export default ManualRun;
