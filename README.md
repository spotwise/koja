# Koja

Koja is a Docker-based, isolated development workspace for AI coding agents.
The host-side interface is a small Bash script; the development runtime and
network egress proxy run in containers.

## Status

This is an experimental security-oriented prototype, not a hardened
multi-tenant sandbox. The agent container has no external network interface.
Public HTTP and HTTPS access is provided through a separate egress proxy that
checks destination IP addresses before connecting.

## Requirements

- Bash
- Docker CLI and a running Docker Engine (Docker Desktop on macOS is suitable)

No Ruby, Python, Node.js, Go, Bundler or project-specific host dependencies
are required. Ruby, Node.js, npm, Claude Code, Codex CLI, `socat` and the proxy
runtime are built into their respective container images.

## Quick start

From the repository root:

```sh
chmod +x bin/koja container/entrypoint
./bin/koja init
```

Edit the generated configuration at `~/.config/koja/workspaces.conf`, then:

```sh
./bin/koja build
./bin/koja list
./bin/koja open demo-service
```

The example alias below is synthetic. Replace it with a directory that exists
on your own machine:

```ini
demo-service=~/workspaces/demo-service
```

The workspace is mounted at `/workspace`. Exit the interactive shell to stop
the session and remove its proxy resources. A separate per-workspace home
volume is retained for CLI credentials and settings; see [Persistent CLI
state](#persistent-cli-state).

## Commands

```text
koja init
koja build
koja list
koja open ALIAS
koja status
koja close ALIAS
koja config
koja help
```

`close` requires an alias because sessions are named after their workspace.
A normal external CLI cannot change the current directory of its parent shell;
`open` therefore starts an interactive container shell in `/workspace`.

## Configuration format

The host wrapper reads a deliberately small `alias=path` format so that the
host needs no YAML parser or other runtime dependency. Blank lines and lines
starting with `#` are ignored. Paths must be absolute or start with `~/`.
Paths are canonicalised before being mounted. Do not put secrets in this file.

The registry is private to the user and defaults to:

```text
~/.config/koja/workspaces.conf
```

Override it with `KOJA_CONFIG`. Override the agent image tag with
`KOJA_IMAGE` and the proxy image tag with `KOJA_PROXY_IMAGE`.

## AI coding CLIs

The agent image includes both:

- `claude` — Claude Code, installed from the official Anthropic npm package.
- `codex` — OpenAI Codex CLI, installed using OpenAI's standalone Linux installer.

Neither CLI is authenticated during image construction. Start either command
inside a workspace and complete its sign-in flow when you are ready to use it.
For Codex in a terminal without a browser, use `codex login --device-auth` and
complete the device flow in a browser on the host. For Claude Code, run
`claude` and follow its account sign-in flow. Claude Code supports Claude Pro
and Max subscriptions as well as Anthropic Console authentication; the latter
may incur API usage charges. Codex supports ChatGPT sign-in for eligible plans
and API-key sign-in, which is usage-billed. Choose the subscription sign-in
method if you want plan-based access rather than API billing.

Claude Code's automatic updater is disabled. Update both CLIs by rebuilding
the Koja image with `./bin/koja build`; the installers resolve current
releases at build time.

## Persistent CLI state

Each workspace has a separate Docker volume named koja-home-ALIAS, mounted at /koja-home. The agent's HOME environment variable is set to this location. The installed CLI executables remain in the image-owned /home/koja directory, so the persistent volume does not hide them.

This preserves Claude and Codex credentials, settings and histories across sessions while keeping them separate between workspaces. The volume is not removed when a session exits or when koja close is used.

These credentials are sensitive. An agent running in that workspace can read
and potentially exfiltrate credentials available to it. Do not authenticate
inside a workspace you do not trust. To erase a workspace's persisted CLI state,
first stop its session, then remove its volume explicitly, for example:

```sh
docker volume rm koja-home-demo-service
```

The example alias is synthetic. Removing the volume permanently deletes the
stored CLI credentials, settings and histories for that workspace.

## Network architecture

Each session gets its own proxy container, named volume and egress network.
The agent container is started with Docker `--network none`; it has no route
to the host, LAN, other containers or the public internet.

A small `socat` listener inside the agent container binds only to
`127.0.0.1:3128` and forwards HTTP proxy traffic over a read-only mounted Unix
socket to the separate proxy container. The proxy is the only session
component attached to a Docker network with external connectivity. No proxy
port is published on the host.

The proxy supports ordinary HTTP requests and TCP tunnels via HTTP CONNECT
(including HTTPS). CONNECT can reach any TCP port on a public IP. `NO_PROXY` and
`no_proxy` are explicitly empty: loopback destinations must go through the proxy
too, rather than silently bypassing the egress policy. The proxy
resolves destination hostnames itself, rejects the entire DNS answer if any
address is non-public, and connects directly to one of the validated IP
addresses. This avoids a second DNS lookup between validation and connection.

The policy denies private, loopback, link-local, carrier-grade NAT,
multicast, documentation, benchmarking, reserved and other special-purpose
address ranges in IPv4 and IPv6. IPv6 destinations are restricted to global
unicast space, with additional special-purpose ranges denied. IPv4-mapped
IPv6 addresses are checked as IPv4.

This is an IP-destination policy, not a service allow-list: arbitrary public
TCP destinations are permitted through HTTP CONNECT. It does **not** prevent
an agent from sending workspace data to a public service. It does not provide
UDP, QUIC, raw sockets or direct non-proxied network access. Programs that
ignore `HTTP_PROXY`/`HTTPS_PROXY` will fail closed rather than bypass the
proxy.

## Containment measures

The agent container:

- mounts only the selected workspace, read-write, at `/workspace`;
- runs as the invoking host UID/GID;
- uses a read-only container root filesystem;
- drops all Linux capabilities and enables `no-new-privileges`;
- applies a PID limit and a size-limited temporary filesystem;
- uses Docker `--network none` (loopback only);
- receives the proxy socket through a read-only named-volume mount;
- does not mount the Docker socket or other host directories.

The proxy container:

- runs as the invoking UID/GID, without Linux capabilities;
- uses a read-only root filesystem and a PID limit;
- has no published ports and listens only on the session's Unix socket;
- is the only component with external network connectivity;
- enforces the destination-IP policy in the proxy process.

These measures reduce exposure but do not make Docker a perfect security
boundary. The Docker daemon, Linux kernel and (on macOS) Docker Desktop VM and
integration remain trusted components. The proxy is also part of the trusted
computing base. Treat this as a useful development isolation layer, not a
hardened boundary for hostile multi-tenant workloads or sensitive production
data. Review the implementation and test it on the actual Docker Desktop
version before relying on it.

## Synthetic-data policy

All examples, fixtures, names, paths, domains, credentials, logs and scenarios
in this repository must be synthetic. Do not copy customer identifiers,
proprietary source, real credentials or real workspace paths into examples.
Use reserved example domains such as `example.com` and `.example`.
