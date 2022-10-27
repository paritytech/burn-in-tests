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
package ansible

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Driver struct {
	path    string // absolute path to ansible files such as playbooks, inventory, etc.
	debug   bool   // enabled by setting environment variable DEBUG_ANSIBLE=1
	verbose bool   // enabled by setting environment variable DEBUG_ANSIBLE=2
}

func NewDriver(path string) *Driver {
	debug := false
	verbose := false

	debugEnv, err := strconv.ParseInt(os.Getenv("DEBUG_ANSIBLE"), 10, 64)
	if err == nil {
		debug = debugEnv > 0
		verbose = debugEnv > 1
	}

	return &Driver{
		path,
		debug,
		verbose,
	}
}

func (d *Driver) RunPlaybook(
	name string,
	runOn string,
	nodeBinary *url.URL,
	wipeChainDB bool,
	nodePublicName string,
	customOptions []string,
) error {
	args, err := buildArgs(name, runOn, nodeBinary, wipeChainDB, nodePublicName, customOptions)
	if err != nil {
		return err
	}

	if d.debug {
		args = append(args, "--diff")
	}

	if d.verbose {
		args = append(args, "-vvvv")
	}

	cmd := exec.Command("ansible-playbook", args...)
	cmd.Dir = d.path

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	log.Println(strings.Join(cmd.Args, " "))
	if err := cmd.Start(); err != nil {
		return err
	}

	stdoutBuf, err := ioutil.ReadAll(stdoutPipe)
	if err != nil {
		return err
	}
	stdout := string(stdoutBuf)

	stderrBuf, err := ioutil.ReadAll(stderrPipe)
	if err != nil {
		return err
	}
	stderr := string(stderrBuf)

	err = cmd.Wait()
	if err != nil || strings.Contains(stdout, "no hosts matched") || strings.Contains(stderr, "[WARNING]") {
		return fmt.Errorf("err: %v\nstdout: %s\nstderr: %s\n", err, stdout, stderr)
	} else if d.debug {
		fmt.Println("stdout:", stdout)
		fmt.Println("stderr:", stderr)
	}
	return nil
}

type ansibleVars struct {
	InventoryHostname string   `json:"inventory_hostname,omitempty"`
	PublicName        string   `json:"node_public_name"`
	Binary            string   `json:"node_binary,omitempty"`
	CustomOptions     []string `json:"node_custom_options"`
	ForceWipe         bool     `json:"node_force_wipe,omitempty"`
}

func buildArgs(
	playbook string,
	runOn string,
	nodeBinary *url.URL,
	wipeChainDB bool,
	nodePublicName string,
	customOptions []string,
) ([]string, error) {
	args := []string{playbook, "-i", "inventory.yaml", "-l", nodePublicName}

	if runOn == "localhost" {
		args = append(
			args,
			"--connection=local",
		)
	} else {
		args = append(
			args,
			"-u",
			"gitlab",
		)
	}

	vars := buildVars(runOn, nodeBinary, wipeChainDB, nodePublicName, customOptions)
	b, err := json.Marshal(vars)
	if err != nil {
		return nil, err
	}

	return append(args, "-e", string(b)), nil
}

func buildVars(
	runOn string,
	nodeBinary *url.URL,
	wipeChainDB bool,
	nodePublicName string,
	customOptions []string,
) ansibleVars {
	vars := ansibleVars{
		PublicName:    nodePublicName,
		ForceWipe:     wipeChainDB,
		CustomOptions: []string{},
	}

	if nodeBinary != nil {
		vars.Binary = nodeBinary.String()
	}

	if customOptions != nil {
		vars.CustomOptions = customOptions
	}

	if runOn == "localhost" {
		vars.InventoryHostname = nodePublicName
	}

	return vars
}
