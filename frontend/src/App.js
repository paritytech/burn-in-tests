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
import {useState, useEffect} from "react";

import RequestForm from "./components/RequestForm";
import Runs from "./components/Runs";
import ManualForm from "./components/ManualForm";
import ManualRuns from "./components/ManualRuns";
import {USER, buildLoginURL, isLoggedIn, isAdmin, fetchAccessToken, fetchUserDetails} from "./oauth";
import {GITLAB_URL, commitRequestFile, fetchRuns, fetchManualRuns, commitManualRunFile, removeRun} from "./gitlab";

function App() {
    const [runs, setRuns] = useState([]);
    const [manualRuns, setManualRuns] = useState([]);

    const updateAll = async () => {
        fetchRuns().then(setRuns);
        fetchManualRuns().then(setManualRuns);
    }

    const finalizeLogin = async (code) => {
        const u = new URL(document.location);
        u.searchParams.delete("code");
        window.history.replaceState(null, document.title, u.toString());

        const accessToken = await fetchAccessToken(GITLAB_URL, code);
        const userDetails = await fetchUserDetails(GITLAB_URL, accessToken);

        if (!userDetails) {
            alert("Unable to exchange auth code for access token. Please contact Staking Ops.");
            return;
        }

        userDetails.accessToken = accessToken;
        localStorage.setItem(USER, JSON.stringify(userDetails));
    }

    const logout = () => {
        localStorage.removeItem(USER);
        startPolling();
    }

    const startPolling = () => {
        let id = Number(localStorage.getItem("burninPollID"));
        if (id) {
            clearInterval(id);
        }

        id = setInterval(updateAll, 5000);
        localStorage.setItem("burninPollID", String(id));
        setTimeout(updateAll, 0);
    }

    useEffect(() => {
        const u = new URL(document.location);
        const code = u.searchParams.get("code");

        if (code) {
            finalizeLogin(code).then(startPolling);
        } else {
            startPolling();
        }
    }, []);


    return (
        <>
            <header>
                <div className="box-wrapper">
                    <div className="box-left">
                        <h1>Burn-in Test Overview</h1>
                    </div>
                    <div className="box-right">
                        {(() => {
                            if (isLoggedIn()) {
                                return <button className="button" onClick={logout}>Logout</button>
                            } else {
                                return <a className="button" href={buildLoginURL(GITLAB_URL)}>Login</a>
                            }
                        })()}
                    </div>
                </div>
            </header>

            {(() => {
                if (isLoggedIn()) {
                    return <>
                        <h2>Request a Burn-in Test</h2>
                        <RequestForm onNewRequest={commitRequestFile}/>
                    </>
                }
            })()}

            <h2>Automated Deployments</h2>
            <Runs runs={runs} onRemoveRun={isLoggedIn() ? removeRun : null}/>

            {(() => {
                if (isAdmin()) {
                    return <>
                        <h2>Add a manually managed deployment</h2>
                        <ManualForm onSubmitManualRun={commitManualRunFile}/>
                    </>
                }
            })()}

            {(() => {
                if (manualRuns.length > 0) {
                    return <>
                        <h2>Manual Deployments</h2>
                        <ManualRuns runs={manualRuns} onRemoveRun={isLoggedIn() ? removeRun : null}/>
                    </>
                }
            })()}
        </>
    )
}

export default App;
