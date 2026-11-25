# Koja

## Purpose

Koja provides a secure, isolated environment for running AI coding agents against software projects.

It allows AI coding agents to access, modify, build, test and inspect software within a controlled environment, while limiting their access to the host system, credentials, network and other resources.

Koja is designed to support multiple agent instances working independently or collaboratively on the same workspace.

Koja initially supports Claude Code and OpenAI Codex.

## Operating Principles

Koja follows these principles:

- **Isolation:** Agents execute inside isolated containers with restricted access to the host system and other resources.
- **Controlled networking:** Agents have no direct network access. Outbound connections are mediated by a dedicated proxy.
- **Shared workspaces:** Agents and the host can access the same workspace without requiring source code to be copied between environments.
- **Independent agent state:** Each agent instance has its own persistent home directory, authentication state and agent-specific configuration.
- **Host-controlled Git:** Agents can inspect Git history and changes but cannot modify Git metadata. The host user retains control over staging and committing.
- **Least privilege:** Containers are configured with the minimum permissions and resources necessary.
- **Fail closed:** Koja must not start an agent when required security controls cannot be established.
- **Simplicity:** Koja should favour straightforward, portable solutions over unnecessary complexity, dependencies or abstractions.

## Architecture

Koja uses containers to isolate agent execution from the host.

Each agent instance runs in its own container. Agent containers share access to their configured workspace but have independent persistent state.

The architecture consists of:

- **Host:** Runs Koja, manages configuration, workspaces, container lifecycle and Git operations.
- **Agent container:** Runs the AI coding agent and development tools.
- **Proxy container:** Mediates outbound network connections on behalf of agents.
- **Workspace:** A host directory containing a Git repository, mounted into the agent container.
- **Agent state:** Persistent storage containing agent-specific configuration, authentication and other state.

Agent containers and proxy containers are separate. The agent container must not have direct network access.

The agent container communicates with the proxy through a Unix-domain socket. The proxy container provides the network access permitted by the effective configuration.

Koja must not depend on a privileged container, host networking, or unrestricted host access to provide its functionality.

### Container engines

Koja supports both Podman and Docker.

Koja automatically selects a container engine according to the following preference:

1. Podman, if available.
2. Docker, if Podman is unavailable and Docker is available.

If neither engine is available, Koja must report an error and exit.

The container engine may be explicitly selected through configuration. When an engine is explicitly selected, Koja must use that engine rather than silently switching to another engine.

Koja must verify that the selected engine is usable before starting an agent session.

Koja must use the selected engine consistently for managing agent containers, proxy containers, images, persistent state and associated resources.

Koja must support the required security controls and functionality on both engines. Differences between Podman and Docker must not result in weakened security requirements.

The container engine abstraction should remain limited to the functionality Koja requires.

### Container security

Agent containers must:

- Run as an unprivileged user.
- Drop unnecessary Linux capabilities.
- Enable `no-new-privileges`.
- Use a read-only container root filesystem.
- Have no direct network access.
- Access the workspace through an explicitly configured mount.
- Access Git metadata read-only.
- Access persistent agent state through an explicitly configured volume or mount.
- Have access to outbound networking only through the Koja proxy.

Koja must configure container security explicitly rather than relying solely on container runtime defaults.

## Workspace

A workspace is an existing host directory containing a Git repository.

Koja does not create, initialise or manage workspace directories. The user is responsible for creating and maintaining them.

The workspace is mounted into the agent container, allowing the agent to read and modify project files. The same workspace remains accessible from the host and may be shared between multiple agent instances.

Workspace access may be configured as read-write or read-only.

Read-only workspace access prevents the agent from modifying workspace files through the workspace mount. It does not guarantee a consistent snapshot if another process modifies the workspace concurrently.

Koja does not provide concurrency control between agents sharing a workspace. Users are responsible for coordinating simultaneous modifications.

### Git

Git is mandatory for all workspaces.

A configured workspace must be an existing Git working tree. Koja must refuse to open a workspace that is not a Git repository.

Koja does not initialise, configure or manage Git repositories.

The initial implementation supports conventional Git repositories with a repository root and Git metadata located in that repository's `.git` directory. Git worktrees and submodules are not required to be supported initially.

The workspace is mounted read-write or read-only according to its configuration. Git metadata is mounted read-only inside the agent container.

Agents may inspect Git history, status, diffs and other repository information. They must not be able to modify Git metadata through the mounted repository.

The host user retains control over staging, committing, branch management and other Git operations that modify repository metadata.

Git is the primary mechanism for reviewing changes made by agents.

Users must review agent changes before accepting them. This includes tracked changes, untracked files, ignored files, changes to `.gitignore`, scripts, build configuration, dependencies and other content that may affect the host or subsequent execution.

Git does not provide complete filesystem integrity monitoring. Koja does not guarantee that all changes made within a workspace will be represented by ordinary Git diffs.

## Networking

Agent containers have no direct network access.

All outbound network access is mediated by a dedicated proxy container. The proxy enforces the effective network policy independently of the agent software.

The agent container communicates with the proxy through a Unix-domain socket. Koja must prevent agents from bypassing the proxy to obtain direct network access.

The proxy provides HTTP proxy functionality and HTTP CONNECT tunnelling. CONNECT tunnelling transports TCP data and is not restricted to HTTPS payloads.

Koja does not provide direct UDP access, direct DNS access from agent containers, unrestricted raw networking or direct IP networking.

DNS resolution is performed by the proxy.

### Network modes

Koja supports three network modes.

#### none

No proxy access is permitted.

Agent containers cannot access external network services, including AI agent backend services.

#### custom

Custom mode is the default.

Custom mode permits access to the supported AI agent backend services and explicitly configured additional destinations.

Both Claude Code and OpenAI Codex backend destinations are permitted by default. The user does not need to select a backend-specific network policy when choosing which agent software to run.

Additional destinations may be configured to support development tools, package repositories, APIs and other services required by the workspace.

#### full

Full mode permits general public outbound TCP access through HTTP proxy requests and HTTP CONNECT tunnelling, subject to proxy policy and destination validation.

It does not provide direct IP networking, UDP, or unrestricted raw network access.

### Proxy enforcement

The proxy must enforce destination restrictions independently of agent software.

The proxy must:

- Apply hostname and port policy before resolving destinations.
- Resolve destinations using proxy-controlled DNS.
- Validate resolved IP addresses before connecting.
- Reject prohibited destinations, including private, loopback, link-local and reserved addresses, as applicable.
- Prevent DNS rebinding and resolution-related bypasses.
- Connect to validated destination addresses without performing an uncontrolled second hostname resolution.
- Reject invalid, ambiguous or prohibited destination representations.
- Fail closed if policy evaluation, DNS resolution, address validation or connection establishment fails.
- Prevent access to prohibited destinations even when a hostname resolves to multiple addresses.
- Ensure that agent processes cannot alter the effective network policy.

Destination validation must account for IPv4 and IPv6, alternate address representations, mixed permitted and prohibited DNS responses, hostname matching, port restrictions and address changes.

Network policy enforcement must not depend on whether an agent uses Claude Code, Codex or another supported application.

## Configuration

Koja uses YAML configuration files.

Configuration is manually created and maintained by the user. Koja does not provide configuration initialisation or mutation commands in the initial implementation.

### Configuration files

Koja supports two configuration files:

- Global configuration: `~/.config/koja/koja.yaml`
- Repository-local configuration: `config/koja.yaml`, located in the Koja repository.

An example configuration is provided at:

`config/koja.yaml.example`

The example configuration may be copied and modified by the user.

### Configuration discovery

Koja always attempts to load the global configuration when it exists.

When Koja is run from its own repository context, Koja also loads the repository-local configuration if it exists.

Repository-local configuration is associated with the Koja repository, not with the workspace being opened.

The repository-local configuration is resolved relative to the Koja repository. It must not be discovered by searching the target workspace or arbitrary directories.

When Koja is run through its executable from a directory outside the Koja repository, only the global configuration is loaded.

The global configuration is optional. Repository-local configuration may be used without a global configuration.

If neither configuration file exists, Koja exits with a warning explaining how to create a configuration by copying `config/koja.yaml.example`.

If a configuration file exists but cannot be parsed or validated, Koja exits with an error. It must not silently fall back to another configuration.

### Configuration merging

When both configurations are present, repository-local configuration overrides the global configuration.

Configuration merging follows these rules:

- Global configuration establishes the baseline.
- Repository-local configuration overrides global settings where both configurations define the same setting.
- Settings not defined in repository-local configuration inherit their global values.
- Workspace definitions are merged by name. A repository-local definition replaces a global definition with the same name.
- Repository-local configuration may override network modes and permitted destinations.
- Repository-local configuration may be used without a global configuration.

The effective configuration must be validated before starting an agent or proxy.

Configuration parsing, merging and validation must be separate operations.

Security requirements must be enforced independently of configuration merging. Overriding a network mode or destination policy must not circumvent container isolation, proxy enforcement, destination validation or other fundamental security controls.

### Workspace configuration

Each configured workspace has a name, directory and associated settings.

Workspace configuration must allow the user to specify:

- Workspace name.
- Host directory.
- Workspace access mode.
- Network mode.
- Additional permitted network destinations, where applicable.

Workspace directories must refer to existing Git repositories.

Koja must validate workspace paths and repository requirements before starting an agent.

The configuration format must support multiple workspaces and allow them to be referenced by name.

### Container engine configuration

Container engine configuration may specify:

- `auto`: Automatically select an available engine, preferring Podman over Docker.
- `podman`: Use Podman.
- `docker`: Use Docker.

Automatic selection is the default.

An explicitly selected engine must be available and usable. Koja must not silently fall back to another engine when an explicitly selected engine cannot be used.

### Network configuration

Network configuration defines:

- Network mode.
- Permitted additional destinations.
- Destination hostname and port restrictions, as applicable.

The global configuration establishes the baseline network policy.

Repository-local configuration may override global network settings. The effective network policy is determined by the merged configuration.

Both supported AI agent backend services must be permitted by default in custom mode unless overridden by the effective configuration.

## Security Requirements

Koja must enforce the following security requirements:

- Agent containers must not have direct network access.
- Outbound network access must be mediated by the proxy.
- Agents must not be able to bypass proxy enforcement.
- Proxy access must be restricted to the effective network policy.
- Destination IP validation must prevent access to prohibited addresses.
- Proxy enforcement must fail closed.
- Agent containers must run with restricted privileges.
- Agent containers must use a read-only root filesystem.
- Git metadata must be read-only inside agent containers.
- Workspaces must be validated as existing Git repositories.
- Agent state and authentication must be isolated between agent instances.
- Agents must not access the persistent state or credentials of other agent instances.
- Agents must not be able to change their own network permissions.
- Koja must not start an agent if required security controls cannot be established.
- Container and proxy lifecycle management must not leave unintended resources or network access behind when an instance terminates.

Koja must be designed to minimise the risk of an agent compromising the host or accessing resources outside its permitted environment.

The workspace and its contents must be treated as untrusted when accessed outside the isolated container.

## Agents

Koja initially supports:

- Claude Code.
- OpenAI Codex.

Both agents are installed in the initial agent container image.

The user may run either agent within a configured workspace. Network policy must not depend on which agent is selected.

Each agent instance has its own persistent agent state, including:

- Agent-specific configuration.
- Authentication state.
- Agent home directory.
- Other persistent state required by the agent.

Agent state must not be shared between instances unless explicitly supported and configured.

Credentials must be accessible only to the agent instance to which they belong.

An agent instance may be opened against a workspace shared with another agent instance. Sharing a workspace does not imply sharing agent state, authentication or configuration.

## Command-line Interface

Koja provides the following commands.

### `koja build`

Builds the Koja container image using the selected container engine.

### `koja list`

Lists configured workspaces.

### `koja open <project>`

Opens a configured workspace by name and starts an interactive agent session.

### `koja open`

Opens the current directory as a workspace, provided it is configured and satisfies workspace requirements.

### `koja status`

Lists and inspects running Koja instances, if any.

### `koja config`

Displays the effective configuration after configuration discovery, merging and validation.

The output must make it possible to inspect the effective workspace, container engine and network configuration.

The CLI must report configuration and operational errors clearly.

Koja must not silently ignore invalid configuration, security failures or unavailable resources.

## Development Environment

The initial Koja image should provide a general-purpose development environment suitable for AI-assisted software development.

It must include:

- A shell and essential command-line utilities.
- Git and basic repository inspection tools.
- Claude Code and OpenAI Codex.
- The dependencies required to run the supported agents.
- Basic tools for downloading, inspecting and working with source code through the Koja proxy.

The base image should remain reasonably small. It must not include complete development toolchains for every supported programming language by default.

Development environments for languages such as Go, Ruby, Python, C and C++ should be installable or configurable as required by individual projects.

Project-specific compilers, SDKs, libraries and build tools should not be included in the base image unless they are needed by Koja itself or are justified as common baseline dependencies.

Development tools installed outside the base image should preferably reside in persistent storage where appropriate, so that they remain available between sessions.

The environment must support accessing HTTP services through the Koja proxy, including a diagnostic service such as `http://ip.spotwise.net`, which may be used to verify proxy connectivity and observe the externally visible IP address of a request.

## Running Koja

Koja supports the following usage patterns.

### Running from the Koja repository

When running Koja from its own repository, repository-local configuration is available in addition to the global configuration.

For example:

`koja open <project>`

This uses the global configuration and repository-local overrides when available.

The global configuration is not mandatory if the repository-local configuration is sufficient.

### Running from PATH

When Koja is installed or made available through PATH and run outside its repository context, Koja uses the global configuration.

For example:

`koja open <project>`

### Opening the current directory

A user may change to a configured workspace and open it directly:

`koja open`

The current directory must be listed in the effective configuration and must satisfy the workspace requirements.

Koja must not automatically register, initialise or modify a workspace merely because the user invokes the command from that directory.

## Session Lifecycle

`koja open` launches an interactive container session attached to the terminal from which Koja is invoked.

The workspace is mounted into the container according to its configuration. The agent container runs in the foreground, providing an interactive shell environment in which the user can run Claude Code, Codex and other development tools.

The user terminates the interactive session by exiting the shell or session.

When the interactive session terminates, Koja must clean up the associated container and temporary resources.

Persistent agent state is retained between sessions.

Koja must not leave unintended containers, proxy resources, sockets or network access behind when an interactive session terminates.

Koja must handle unexpected session termination and failures without leaving unintended resources behind.

## Design Constraints

Koja must favour:

- Straightforward implementation.
- Portable solutions where practical.
- Minimal external dependencies.
- Explicit configuration.
- Clear separation between host management, agent execution and network enforcement.
- Security controls that do not depend on agent behaviour.
- Predictable resource and container lifecycle management.

Koja must avoid:

- Unnecessary abstractions.
- Configuration mutation commands in the initial implementation.
- Direct network access from agent containers.
- Privileged agent containers.
- Dependence on a particular AI agent's internal behaviour for security enforcement.
- Unnecessary host services.
- Unnecessary complexity in workspace or agent management.
- Features that do not directly support the initial use cases.

## Requirements and Implementation

This document is the authoritative specification for Koja.

Implementation must satisfy the requirements defined here.

Where implementation details are not specified, the implementation should choose the simplest approach consistent with the architecture, security requirements and design constraints.

Requirements must not be weakened or bypassed merely to simplify implementation.

Changes to the architecture, security requirements, configuration semantics or supported use cases must be reflected in this document.