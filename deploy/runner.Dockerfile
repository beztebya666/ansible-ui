# --- build the Go runner ----------------------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY examples ./examples
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/runner ./cmd/runner

# --- runtime: ansible + the runner ------------------------------------
FROM python:3.12-slim
# ansible-core gives us ansible-playbook. Pinned to match the reference Semaphore
# (core 2.20.x) so verbose output (banner, task paths) lines up. ansible.posix /
# community collections cover the demo's modules.
ARG ANSIBLE_CORE_VERSION=2.20.6
RUN apt-get update \
    && apt-get install -y --no-install-recommends openssh-client sshpass ca-certificates git rsync \
    && rm -rf /var/lib/apt/lists/* \
    && pip install --no-cache-dir "ansible-core==${ANSIBLE_CORE_VERSION}" passlib boto3 botocore google-auth requests \
    && ansible-galaxy collection install ansible.posix -p /usr/share/ansible/collections \
    && ansible-galaxy collection install community.general -p /usr/share/ansible/collections \
    && ansible-galaxy collection install amazon.aws -p /usr/share/ansible/collections \
    && ansible-galaxy collection install community.aws -p /usr/share/ansible/collections \
    && ansible-galaxy collection install google.cloud -p /usr/share/ansible/collections

# Azure dynamic inventory (azure.azcollection.azure_rm). Heavy SDK, so it's a
# separate layer. The azure_rm plugin needs azure-cli-core (it does
# `from azure.cli.core import cloud`) + the collection's *pinned* azure-mgmt set —
# unpinned latest breaks with "name 'azure_cloud' is not defined". Install the
# collection's own requirements.txt for guaranteed-compatible versions.
RUN ansible-galaxy collection install azure.azcollection -p /usr/share/ansible/collections \
    && pip install --no-cache-dir -r \
       /usr/share/ansible/collections/ansible_collections/azure/azcollection/requirements.txt

# --- multi-app tooling: terraform / opentofu / terragrunt / powershell ----
# bash + python3 are already in the Debian base; this adds the IaC/script CLIs
# so a template's "app" can target any of them.
ARG TERRAFORM_VERSION=1.9.8
ARG OPENTOFU_VERSION=1.8.7
ARG TERRAGRUNT_VERSION=0.69.10
ARG POWERSHELL_VERSION=7.4.6
# releases.hashicorp.com is unreachable in some regions; a mirror keeps the
# real Terraform binary available. Override with --build-arg if needed.
ARG TERRAFORM_BASE=https://mirror.yandex.ru/mirrors/releases.hashicorp.com/terraform
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    apt-get update; \
    icu_pkg="$(apt-cache search --names-only '^libicu[0-9]+$' | sort -V | tail -n1 | cut -d' ' -f1)"; \
    [ -n "$icu_pkg" ] || icu_pkg=libicu-dev; \
    apt-get install -y --no-install-recommends curl unzip bash "$icu_pkg"; \
    # OpenTofu (also serves as the Terraform fallback below).
    curl -fsSL "https://github.com/opentofu/opentofu/releases/download/v${OPENTOFU_VERSION}/tofu_${OPENTOFU_VERSION}_linux_${arch}.zip" -o /tmp/tofu.zip; \
    unzip -o /tmp/tofu.zip -d /usr/local/bin tofu; \
    chmod +x /usr/local/bin/tofu; \
    # Terraform — from a mirror (releases.hashicorp.com is blocked in some
    # networks). If even the mirror is unreachable, fall back to the
    # CLI-compatible OpenTofu binary so the Terraform app still runs.
    if curl -fsSL "${TERRAFORM_BASE}/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_${arch}.zip" -o /tmp/tf.zip && unzip -o /tmp/tf.zip -d /usr/local/bin terraform; then \
      chmod +x /usr/local/bin/terraform; \
    else \
      echo "terraform download unavailable — symlinking terraform -> tofu"; \
      ln -sf /usr/local/bin/tofu /usr/local/bin/terraform; \
    fi; \
    curl -fsSL "https://github.com/gruntwork-io/terragrunt/releases/download/v${TERRAGRUNT_VERSION}/terragrunt_linux_${arch}" -o /usr/local/bin/terragrunt; \
    chmod +x /usr/local/bin/terragrunt; \
    if [ "$arch" = "amd64" ]; then \
      curl -fsSL "https://github.com/PowerShell/PowerShell/releases/download/v${POWERSHELL_VERSION}/powershell-${POWERSHELL_VERSION}-linux-x64.tar.gz" -o /tmp/pwsh.tar.gz; \
      mkdir -p /opt/microsoft/powershell/7; \
      tar zxf /tmp/pwsh.tar.gz -C /opt/microsoft/powershell/7; \
      chmod +x /opt/microsoft/powershell/7/pwsh; \
      ln -sf /opt/microsoft/powershell/7/pwsh /usr/local/bin/pwsh; \
    fi; \
    rm -rf /tmp/*.zip /tmp/*.tar.gz /var/lib/apt/lists/*

# --- Pulumi (IaC app) — the YAML language runtime is bundled in the CLI, so no
# extra language runtime is needed. Best-effort (like the Terraform fallback): a
# download failure just leaves the Pulumi app unavailable, it doesn't break the
# image. Uses a local file backend at run time (no Pulumi Cloud login).
ARG PULUMI_VERSION=3.140.0
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in amd64) parch=x64;; arm64) parch=arm64;; *) parch="";; esac; \
    if [ -n "$parch" ] && curl -fsSL "https://github.com/pulumi/pulumi/releases/download/v${PULUMI_VERSION}/pulumi-v${PULUMI_VERSION}-linux-${parch}.tar.gz" -o /tmp/pulumi.tar.gz; then \
      tar zxf /tmp/pulumi.tar.gz -C /usr/local/bin --strip-components=1; \
      chmod +x /usr/local/bin/pulumi*; \
      /usr/local/bin/pulumi version || true; \
    else \
      echo "pulumi download unavailable — the Pulumi app will be disabled"; \
    fi; \
    rm -rf /tmp/*.tar.gz

COPY --from=build /out/runner /usr/local/bin/runner

ENV TERM=xterm-256color \
    ANSIBLE_FORCE_COLOR=1 \
    PY_COLORS=1 \
    ANSIBLE_HOST_KEY_CHECKING=False \
    ANSIBLE_ASYNC_DIR=/tmp/.ansible_async \
    RUNNER_ADDR=:8081

EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/runner"]
