# Koja

## Purpose

Koja provides a secure, isolated environment for running AI coding agents against software projects.

It allows agents to work directly with a project's source files while restricting their access to the host system, credentials, and network resources. The environment should be straightforward to use, secure by default, and portable across development machines.

Koja initially supports Claude Code and OpenAI Codex.

## Operating Principles

- **Isolation:** Agents run inside containers rather than directly on the host.
- **Least privilege:** Agents receive only the permissions and resources they need.
- **Controlled networking:** All agent network access is mediated by a separate egress proxy.
- **Shared workspace:** Agents work directly in the selected project directory, which is also accessible from the host.
- **Independent state:** Each Koja instance has its own agent home directory, authentication state, and configuration.
- **Portability:** Prefer conventional, well-supported technologies and avoid unnecessary dependencies.
- **Simplicity:** Keep the implementation and user interface as straightforward as possible without compromising security.

## Architecture

Each Koja instance consists of an agent container and the supporting infrastructure required to run it.

The agent container:

- Runs as an unprivileged user.
- Has Linux capabilities dropped and `no-new-privileges` enabled.
- Uses a read-only root filesystem.
- Has no direct network access.
- Has access to the selected project through a shared workspace mount.
- Has separate persistent storage for its home directory and authentication state.

The egress proxy runs separately from the agent container. Communication between the agent and the proxy is mediated through a Unix socket.

The agent container must not be able to bypass the proxy to obtain network access.

Multiple Koja instances may operate on the same project directory. Each instance retains separate agent state and credentials. Concurrent modifications to source files and Git operations must be treated as potential coordination issues.

## Workspace

An agent works directly in the selected project directory.

Changes made by the agent are therefore immediately visible to the host and to other Koja instances using the same directory.

The shared workspace must be treated as untrusted input when files are subsequently executed or used outside the container. Container isolation does not make workspace contents safe to execute on the host.

Koja must keep agent-specific state and credentials outside the shared workspace.

## Networking

### General requirements

Agents must not have direct network access.

All permitted network access goes through an HTTP proxy. This supports HTTP and HTTPS traffic from applications that understand and use an HTTP proxy.

Koja does not provide general-purpose IP networking to agents. In particular:

- No direct DNS access.
- No direct TCP or UDP connections.
- No unrestricted raw networking.
- No direct access to the host or local networks.
- No UDP-based QUIC access.

Applications that do not support HTTP proxy configuration cannot use the network unless they are adapted to do so.

### Network modes

Koja supports three mutually exclusive network modes.

**`none`**

No proxy access is available to the agent, including access to the services normally required by the selected agent.

**`custom`**

The default mode.

The selected agent's required backend services are automatically permitted. Additional destinations can be explicitly permitted through configuration.

The effective policy is determined by the selected agent profile and the configured additional destinations.

**`full`**

Allows general public HTTP and HTTPS access through the proxy.

This mode does not provide direct network access or unrestricted non-HTTP networking. Destination IP validation remains mandatory.

### Agent network requirements

Agent-specific network requirements belong in the corresponding Koja agent profile, not in the proxy implementation.

Each profile specifies the destinations required by that agent. Koja selects the appropriate profile automatically when an agent is selected.

Configuration examples should document the available destinations, with unused agent entries commented out where appropriate.

### Proxy enforcement

The proxy is responsible for enforcing network policy independently of the agent.

It must:

1. Apply the configured hostname and port policy before resolving a destination.
2. Resolve the destination hostname.
3. Validate the resulting IP addresses.
4. Reject prohibited destinations, including private, loopback, link-local, and reserved address ranges.
5. Connect directly to a validated IP address rather than performing a second hostname resolution during connection establishment.

The proxy must fail closed if policy evaluation, DNS resolution, or destination validation fails.

Hostname validation must not be bypassed by alternative IP representations, DNS rebinding, or changes in DNS responses.

The proxy must not contain hardcoded destination lists for individual agents. It receives the effective policy from Koja and enforces it.

## Configuration

Koja supports global configuration and a local configuration associated with the Koja repository itself.

### Global configuration

The global configuration is stored at:

`~/.config/koja/config.yaml`

It contains user-specific defaults, configured projects, and permissions.

It is used whenever Koja is run outside its own repository. It can also be used when Koja is run from its repository.

### Local configuration

The local configuration belongs to the Koja repository.

It provides overrides to the global configuration when Koja is run from that repository.

This allows separate copies or forks of the Koja repository to have independent configurations. It also allows Koja to be used directly from its repository without requiring a global configuration.

The local configuration is not automatically loaded when Koja is run from elsewhere, even if the selected project happens to contain a Koja-related configuration file.

### Configuration precedence and permissions

Global configuration establishes the user's defaults and permissions.

Local configuration may override global configuration when Koja is run from its own repository.

Project-specific requirements and permission requests must not silently weaken the user's global security settings or override Koja's security invariants.

Any request to expand permissions beyond those already granted requires explicit user approval.

Koja must make the effective configuration inspectable, including the source of each applicable setting.

Example configurations should be provided and installed by `koja init` and `koja init --global`, as appropriate.

## Security Requirements

Security is a fundamental property of Koja, not something delegated to the coding agent.

The implementation must:

- Fail closed when security policy cannot be established or enforced.
- Prevent agents from bypassing the controlled network path.
- Reject prohibited IP address ranges.
- Validate destinations against the applicable hostname and port policy before connecting.
- Enforce network restrictions independently of the agent software.
- Keep agent credentials and persistent state separate from the shared workspace.
- Run agents without unnecessary privileges.
- Treat workspace contents as untrusted when accessed outside the container.
- Preserve security invariants regardless of configuration overrides.

The security model must not rely on an agent voluntarily following instructions or respecting its own configuration.

## Agents

The initial supported agents are:

- Claude Code
- OpenAI Codex

Koja manages the configuration and network requirements for each supported agent.

Agent-specific settings, including required network destinations, belong in agent profiles. The proxy remains independent of the agent implementation.

The architecture should allow additional agents to be supported without requiring changes to the underlying isolation or proxy enforcement mechanisms.

## Command-line Interface

Koja provides a command-line interface for managing the development environments.

The interface should support:

- `koja init` — initialise a project for use with Koja.
- `koja init --global` — initialise global Koja configuration.
- `koja build` — build the required container image.
- `koja list` — list configured projects.
- `koja open <project>` — open an agent workspace for the specified project.
- `koja open` — open an agent workspace for the current directory, subject to the requirements in the Running section.
- `koja status` — inspect the status of Koja instances.
- `koja close` — close a Koja instance.
- `koja config` — inspect the effective configuration.

Commands should have predictable behaviour and provide useful error messages when their prerequisites are not met.

## Running

Koja can be run in a few different ways.

1. Change directory to the repository containing Koja, then type `koja open <project>`.
   - This uses the global configuration, if present, with the overrides specified in the local configuration.
   - A global configuration is not required in this mode.

2. Ensure that `bin/koja` is in the `PATH`, then type `koja open <project>`.
   - This uses only the global configuration.

3. Ensure that `bin/koja` is in the `PATH`, change to a project directory and type `koja open`.
   - This also uses only the global configuration.
   - The current directory must be specified in the global configuration. Otherwise, Koja exits with an error message.

This provides two main ways of using Koja:

1. **Repository-based usage:** Run Koja from its own repository to open a workspace in another directory. In this case, there will most likely be no global configuration. This also allows entirely separate configurations by creating multiple copies or forks of the Koja repository.

2. **PATH-based usage:** Run Koja from anywhere, provided its executable is in the `PATH`. This allows users to quickly start working by typing `koja list`, followed by `koja open <project>`.

The availability of the local configuration is determined by running Koja from its own repository. It must not be loaded merely because the executable was found there or because the target project contains a configuration file.

## Design Constraints

- Prefer simple, conventional, portable solutions.
- Avoid unnecessary dependencies and infrastructure.
- Keep security enforcement independent of the agent.
- Keep agent-specific configuration separate from generic proxy enforcement.
- Do not compromise isolation for convenience.
- Avoid architecture changes that add complexity without a clear security or functional benefit.

## Requirements and Implementation

This document defines the product requirements and is the authority for implementation decisions.

The implementation must conform to these requirements.

When a requirement is ambiguous or conflicts with another requirement, resolve the ambiguity before implementing the affected behaviour.

Changes to requirements should be reflected in this document so that it remains an accurate description of the intended product.