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
	"errors"
	"time"
)

type Poller struct {
	sleep func(time.Duration)
}

func (p Poller) Poll(
	timeout time.Duration,
	initialStatus string,
	updateStatus func() (string, error),
	additionalWaitStatus ...string,
) error {
	if p.sleep == nil {
		p.sleep = time.Sleep
	}

	waitStatus := map[string]bool{
		"created":              true,
		"waiting_for_resource": true,
		"preparing":            true,
		"pending":              true,
	}

	for _, ws := range additionalWaitStatus {
		waitStatus[ws] = true
	}

	var (
		err      error
		runtime  time.Duration
		interval = 5 * time.Second
		status   = initialStatus
	)

	for {
		if status, err = updateStatus(); err != nil {
			return err
		}

		if !waitStatus[status] {
			break
		}

		p.sleep(interval)
		runtime += interval

		if runtime >= timeout {
			return errors.New("timed out waiting for status change")
		}
	}

	return nil
}
