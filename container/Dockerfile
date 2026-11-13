FROM ruby:3.4-slim-bookworm

ARG KOJA_UID=1000
ARG KOJA_GID=1000

RUN apt-get update \
    && apt-get install --yes --no-install-recommends \
       bash ca-certificates curl dnsutils git iproute2 iputils-ping netcat-openbsd nodejs npm passwd socat traceroute wget \
    && rm -rf /var/lib/apt/lists/* \
    && if ! getent group "${KOJA_GID}" >/dev/null; then \
         groupadd --gid "${KOJA_GID}" koja; \
       fi \
    && useradd --uid "${KOJA_UID}" --gid "${KOJA_GID}" \
       --create-home --shell /bin/bash koja

# Install Claude Code using its supported npm distribution. Keep the package in
# the immutable image rather than allowing self-updates inside a session.
RUN npm install --global --prefix /opt/claude-code @anthropic-ai/claude-code \
    && chmod -R a+rX /opt/claude-code

ENV PATH="/home/koja/.local/bin:/opt/claude-code/bin:${PATH}"
ENV DISABLE_AUTOUPDATER=1

# Install the standalone Codex CLI as the image's unprivileged build user.
USER koja
RUN curl -fsSL https://chatgpt.com/codex/install.sh -o /tmp/install-codex.sh \
    && sh /tmp/install-codex.sh \
    && rm -f /tmp/install-codex.sh
USER root

COPY entrypoint /usr/local/bin/koja-entrypoint
COPY --chmod=644 bashrc /etc/koja/bashrc
RUN chmod 0755 /usr/local/bin/koja-entrypoint

WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/koja-entrypoint"]
