# What is this?

This repository contains a pipeline for automated management of burn-in tests. It is intended to follow the
[GitOps](https://www.gitops.tech/) approach to infrastructure management. In particular, the following principles:

1. This git repository contains the desired state of the infrastructure.
2. An agent (here Gitlab CI) performs reconciliation of desired and actual state.
3. Operations can be rolled back by reverting a commit.

Unfortunately point 3 does not really work in all cases. Oh well, it was a nice idea...

## Requirements

The pipeline in this repository requires two kinds of Gitlab runners. One for general tasks like parsing & generating
files. These should be handled by Kubernetes runners. The other kind of runners are blockchain nodes of different types
(full node, sentry, validator), on which the burn-in tests will be performed. Those runners only pick up jobs with tags
that correspond to their network and node type (e.g. `kusama-fullnode`).

## Repository structure

The `master` branch represents the desired state. The repository contains two folders that are relevant to the
automation: `requests` and `runs`.

## File format

The files in those two folders are formatted in [ToML](https://toml.io/).

Files in the `requests` folder and named `request-<unix timestamp>.toml`. Below is an example with all recognized
attributes:

```toml
pull_request = "https://github.com/paritytech/polkadot/pull/2013"
commit_sha = "a7810560c0f62dd6d347e710a5e2a64da465c109"
custom_binary = "https://gitlab.example.com/parity/polkadot/-/jobs/752482/artifacts/raw/artifacts/polkadot"
custom_options = ["--wasm-execution Compiled", "--rpc-methods Unsafe"]
requested_by = "mxinden"
sync_from_scratch = false

[nodes.westend]
validator = 1

[nodes.kusama]
fullnode = 1
validator = 0

[nodes.polkadot]
fullnode = 0
validator = 1
```

The fields `commit_sha`, `custom_binary` and `sync_from_scratch` are optional. If `custom_binary` is present, it will be
used for downloading the client binary. Otherwise, the behaviour of the request processing job depends on the URL in
`pull_request`:

* If `pull_request` points to [the repository `paritytech/polkadot`](https://github.com/paritytech/polkadot), it tries
  to find the pipeline for `commit_sha`, if provided, or the most recent one for this pull request on
  `gitlab.example.com`, and in that pipeline the CI job `test-linux-stable`. If the `test-linux-stable` job has not
  run yet but is ready to be started, the request processing job will do so and poll for the build to finish.
  If the `test-linux-stable` job is not available yet, because the pipeline on `gitlab.example.com` is still running,
  it will poll the pipeline until the job becomes available, previous jobs fail, or a timeout is hit. If the job failed
  or is unavailable because a previous job failed, the `request` processing job is aborted.
* If `pull_request` points to [the repository `paritytech/substrate`](https://github.com/paritytech/substrate), the
  request processing is aborted with an error message explaining why it is not possible to run a burn-in test for
  changes in Substrate, without a pull request that uses those changes in Polkadot.

`requested_by` is only included for documentation purposes at the moment. Depending on the method used for submitting
the request, the value in `requested_by` can be a Github username, an email address, or a Matrix handle.

If `sync_from_scratch` is `true`, the chain db directory deleted before the client binary is updated. This flag only
applies to full nodes. The attribute is optional and defaults to `false`.

Files in the `runs` folder are named `run-<network>-<node type>-<sequential number>-<unix timestamp>.toml` and have the
following schema:

```toml
pull_request = "https://github.com/paritytech/polkadot/pull/2013"
commit_sha = "a7810560c0f62dd6d347e710a5e2a64da465c109"
custom_binary = "https://gitlab.example.com/parity/polkadot/-/jobs/752482/artifacts/raw/artifacts/polkadot"
custom_options = ["--wasm-execution Compiled", "--rpc-methods Unsafe"]
requested_by = "mxinden"
sync_from_scratch = false
network = "kusama"
node_type = "fullnode"
deployed_on = "kusama-burnin-fullnode-0"
public_fqdn = "kusama-burnin-fullnode-0.example.com"
internal_fqdn = "kusama-burnin-fullnode-0-int.example.com"
deployed_at = 2020-10-21T19:50:00Z
updated_at = 2020-10-23T14:23:42Z
log_viewer = "http://grafana.example.com/explore?orgId=1&left=%5B%22now-1h%22%2C%22now%22%2C%22loki%22%2C%7B%22expr%22%3A%22%7Bhost%3D%5C%22kusama-burnin-fullnode-0%5C%22%7D%22%7D%5D"

[dashboards]
  grandpa = "http://grafana.example.com/d/EzEZ60fMz/grandpa?orgId=1&refresh=1m&var-nodename=kusama-fullnode-uw1-0-int.example.com:9615"
  kademlia_and_authority_discovery = "http://grafana.example.com/d/NMSIHdDGz/kademlia-and-authority-discovery?orgId=1&refresh=1m&var-nodename=kusama-burnin-fullnode-0-int.example.com:9615"
  substrate_networking = "http://grafana.example.com/d/vKVuiD9Zk/substrate-networking?orgId=1&refresh=1m&var-nodename=kusama-burnin-fullnode-0-int.example.com:9615"
  substrate_service_tasks = "http://grafana.example.com/d/3LA6XNqZz/substrate-service-tasks?orgId=1&refresh=1m&var-nodename=kusama-burnin-fullnode-0-int.example.com:9615"
```
There will be one of these files for each node the burn-in is deployed on. The timestamp in the name is the same as in
the corresponding `request` file. The sequential number is used to generate unique file names and is incremented from 0
to N per node type.
In the above example, assuming the file in `requests` was named `request-1602856340.toml`, there will be these four
files in `runs`: `run-kusama-fullnode-0-1602856340.toml`, `run-kusama-sentry-0-1602856340.toml`,
`run-kusama-sentry-1-1602856340.toml` and `run-polkadot-validator-1-1602856340.toml`.

The fields `deployed_at`, `deployed_on`, `public_fqdn` and `internal_fqdn` will be populated after the deployment has
actually happened. The field `updated_at` only gets added to the file if the deployment is ever updated.

## Workflow

### Requesting a burn-in

1. A `request` file is added to `requests` and committed either directly to `master` or to a branch, with a
   corresponding merge request.
2. Once merged to `master`, a CI job is started that
  - tries to start the `test-linux-stable` job on `gitlab.example.com`, if no `custom_binary` URL is included
  - generates `run` files from the `request` file
  - commits `run` files for full nodes to `master`, with the prefix `[deploy-<network>-<node type>]` in the commit
    message
4. One CI job per `run` file is started that
  - is executed on a Gitlab runner that is tagged with the desired network and node type
  - performs the actual deployment by running the Ansible playbook `kusama-nodes.yml`/`polkadot-nodes.yml` on
    localhost, with the custom binary URL
  - adds a commit on `master` that adds `deployed_on` and `deployed_at` in the `run` file
  - pauses the Gitlab runner that picked up the job

There is one job definition in `.gitlab-ci.yml` for each network and node type.

### Updating a burn-in

For `request` files that contain `commit_sha`, an ongoing burn-in test can be updated with a binary built from that
revision by updating `commit_sha`.
An update of `custom_binary` in request files that have it, will cause an update to `custom_binary` in the `run` files.
If the request file contains `commit_sha`, but that was *not* updated, it will be removed from `run` files (since it
can no longer be determined). If the request file contains `commit_sha` and it was also modified in the commit that
updated `custom_binary`, it is assumed that they match up and both will be updated in `run` files.

This is all pretty confusing and error prone unfortunately. The reason why it works this way is to always have the
option of setting `custom_binary` directly, for example when a binary was built specifically for debugging purposes,
outside of the regular CI pipeline. This flexibility comes at the cost of complex rules (which might be implemented
incorrectly) and in some cases, loss of information from which revision of the code a binary was built.

Therefore, it is best whenever possible to just set `commit_sha` in the initial request and update it when necessary and
leave managing `custom_binary` in the `run` files to the automation.

The only other parameter of a burn-in that can be updated is `custom_options`. Changing the number of nodes or flipping
the value of `sync_from_scratch` will have no effect (other than bringing the `request` file out of sync with the `run`
files).

### Removing a burn-in

There is no automatic cleanup yet. In order to remove a burn-in test, delete the corresponding `run` file and prefix
the commit message with `[cleanup]`. The cleanup CI job will run `kusama-nodes.yml`/`polkadot-nodes.yml` without a
custom binary and unpause the runner in Gitlab. The last cleanup job for the associated burn-in test also removes the
`request` file from `master`.

Idle burn-in nodes run `https://releases.example.com/builds/polkadot/x86_64-debian:stretch/master/polkadot`, which is
updated on them daily at 09:00 UTC.

Eventually, closing a "burn-in" PR in `paritytech/polkadot` will remove the corresponding deployment.

