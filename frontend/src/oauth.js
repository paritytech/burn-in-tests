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
export const USER = "burninFrontendUser"; // used as key in local storage for user details
const OAUTH_CLIENT_ID = "REDACTED";
const OAUTH_CLIENT_SECRET = "REDACTED";
const REDIRECT_URL = "https://burnins.example.com/";

const ADMINS = [
    "jane@example.com",
    "alice@example.com",
    "dave@example.com",
    "bob@example.com",
];

export function buildLoginURL(gitlabURL) {
    const u = new URL(gitlabURL);
    u.pathname = "/oauth/authorize";
    u.searchParams.append("client_id", OAUTH_CLIENT_ID);
    u.searchParams.append("redirect_uri", REDIRECT_URL);
    u.searchParams.append("response_type", "code");
    u.searchParams.append("scope", "api");
    return u.toString();
}

export function isLoggedIn() {
    return getCurrentUser() !== null;
}

export function isAdmin() {
    const u = getCurrentUser();
    if (!u) {
        return false;
    }

    return ADMINS.includes(u.email);
}

export function getCurrentUser() {
    const item = localStorage.getItem(USER);
    if (item === null) {
        return null;
    }

    return JSON.parse(item);
}

export async function fetchAccessToken(gitlabURL, code) {
    const body = {
        client_id: OAUTH_CLIENT_ID,
        client_secret: OAUTH_CLIENT_SECRET,
        redirect_uri: REDIRECT_URL,
        grant_type: "authorization_code",
        code: code,
    }

    const url = `${gitlabURL}/oauth/token`;
    const response = await fetch(url, {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify(body)
    });

    if (!response.ok) {
        console.error(`POST ${url} failed with ${response.status} ${await response.text()}`);
        return;
    }

    const data = await response.json();
    return data.access_token;
}

export async function fetchUserDetails(gitlabURL, accessToken) {
    const url = `${gitlabURL}/api/v4/user`;

    const response = await fetch(url, {
        "method": "GET",
        "headers": {"Authorization": `Bearer ${accessToken}`},
    });

    if (!response.ok) {
        console.error(`GET ${url} failed with ${response.status} ${await response.text()}`);
        return;
    }

    const body = await response.json();
    return {
        name: body.name,
        email: body.email,
    }
}
