# Security policy and development rules

## Synthetic material

All example projects, sample configuration, test fixtures, documentation
scenarios, names, paths, identifiers, credentials, network addresses and
demonstration data must be synthetic. Never use customer information,
proprietary project details or real secrets in this repository.

Use reserved example domains (for example, `example.com` and `.example`) and
clearly fictitious project names such as `demo-service` and `sample-api`.

## Current security posture

Koja is an experimental prototype. The agent container has no direct network
interface. HTTP and HTTPS requests are forwarded through a separate proxy over
a Unix socket in a per-session named volume. The proxy resolves names, rejects
non-public destination IPs and connects to the validated address. The agent
cannot directly access the proxy's network interface or Docker bridge.

This does not prevent data exfiltration to arbitrary public HTTP/HTTPS
services. The Docker daemon, kernel, Docker Desktop VM/integration and proxy
remain trusted components. Do not treat the prototype as a hardened
multi-tenant security boundary.

## Changes that require security review

Review any change that:
- adds a host mount or broadens the workspace mount;
- gives the agent container a network interface or direct network route;
- changes proxy IP classification, DNS resolution or dial behaviour;
- exposes the proxy over a published TCP port or changes its Unix socket;
- adds Docker socket, privileged mode or Linux capabilities;
- changes container user identity or filesystem permissions;
- adds secrets, credentials or host environment variables to the container;
- adds agent integrations or host-side executable code.

## Network policy tests

Changes to the address policy must include tests for both IPv4 and IPv6,
including private, loopback, link-local, carrier-grade NAT, multicast,
documentation, benchmarking, reserved and IPv4-mapped IPv6 destinations.
A DNS answer containing any non-public address must be rejected in its
entirety. Never resolve a hostname once for validation and again for the
actual connection.
