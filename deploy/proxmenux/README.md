# ProxMenux OCI Deployment (`deploy/proxmenux`)

## Overview
The `deploy/proxmenux` package contains official ProxMenux OCI templates (schema version `0.5.0`, kind `proxmenux.oci-template`), Python catalog configuration scripts, and operational deployment documentation for provisioning Tsuruyomi on Proxmox VE as native OCI LXC containers. It supports three deployment profiles: Standalone (single LXC), Stack (2-LXC with PostgreSQL 17), and All-in-One (3-LXC with PostgreSQL 17 and Byparr Cloudflare solver).

### Catálogo de Perfiles Disponibles
1. **Tsuruyomi (Solo App / Standalone) — `tsuruyomi`**:
   - Contenedor LXC individual sin dependencias ni hookscripts secundarios.
   - Ideal para infraestructuras con PostgreSQL dedicado preexistente.
   - Actualizaciones limpias preservando volúmenes persistentes `/config` y `/data`.
2. **Tsuruyomi Stack (App + PostgreSQL) — `tsuruyomi-stack`**:
   - Stack multi-LXC automático de 2 contenedores (Tsuruyomi + PostgreSQL 17 en red privada interna).
   - Generación automática de credenciales seguras y fijación a `postgres:17-alpine`.
3. **Tsuruyomi All-in-One (App + PostgreSQL + Byparr) — `tsuruyomi-byparr`**:
   - Stack multi-LXC automático de 3 contenedores (Tsuruyomi + PostgreSQL 17 + Byparr en red privada interna).
   - Resolución automática de retos de Cloudflare Turnstile en fuentes protegidas.

## Interface
- `[tsuruyomi.json:image-tsuruyomi]`: Standalone OCI template for single-LXC Tsuruyomi deployment connecting to external PostgreSQL.
- `[tsuruyomi.json:container_contract]`: Contract declaring container image, required environment variables (`DATABASE_DSN`, `SERVER_PORT`, `TZ`), persistent volumes (`/config`, `/data`), and port `7654`.
- `[tsuruyomi.json:proxmox]`: Proxmox runtime specifications configuring unprivileged LXC defaults (2 cores, 512MB RAM, nesting=1, volume preparations).
- `[tsuruyomi-stack.json:image-tsuruyomi-stack]`: Multi-LXC stack template defining 2-container deployment (Tsuruyomi + PostgreSQL 17).
- `[tsuruyomi-stack.json:compose_stack]`: Orchestration topology declaring service dependencies, private bridge network, volume mappings, and auto-generated credentials.
- `[tsuruyomi-byparr.json:image-tsuruyomi-byparr]`: Multi-LXC stack template defining 3-container all-in-one deployment (Tsuruyomi + PostgreSQL 17 + Byparr).
- `[tsuruyomi-byparr.json:compose_stack]`: Orchestration topology declaring 3 services (`postgres`, `byparr`, `server`) with dedicated resource allocations and port mappings.
- `[configure_tsuruyomi_catalog.py]`: Python automation script configuring and updating ProxMenux catalog definitions.
- `[README.md:sync-tsuruyomi-catalog]`: Host command (`sync-tsuruyomi-catalog` or `python3 /usr/local/bin/sync-tsuruyomi-catalog`) to synchronize catalog definitions on Proxmox nodes.

## Invariants
- `[tsuruyomi.json:schema_version]`: Must adhere strictly to ProxMenux schema version `0.5.0`.
- `[tsuruyomi.json:kind]`: Template kind must be strictly `proxmenux.oci-template`.
- `[tsuruyomi.json:container_contract.environment]`: `DATABASE_DSN`, `SERVER_PORT`, and `TZ` are required environment variables (`required: true`).
- `[tsuruyomi.json:container_contract.volumes]`: Volumes `/config` and `/data` are mandatory persistent mounts (`required: true`).
- `[tsuruyomi.json:container_contract.ports]`: Port `7654` TCP is exposed with Proxmox behavior `listener-on-dedicated-lxc-address-no-nat`.
- `[tsuruyomi.json:proxmox.defaults.unprivileged]`: LXC containers must be created unprivileged (`unprivileged: true`) with `nesting=1`.
- `[tsuruyomi-stack.json:compose_stack.service_count]`: Total service count is strictly 2.
- `[tsuruyomi-stack.json:compose_stack.orchestration.reserve_vmids_atomically]`: Exactly 2 VMIDs must be atomically reserved.
- `[tsuruyomi-stack.json:compose_stack.orchestration.start_order]`: Container startup sequence is strictly `['postgres', 'server']`.
- `[tsuruyomi-stack.json:compose_stack.orchestration.stop_order]`: Container shutdown sequence is strictly `['server', 'postgres']`.
- `[tsuruyomi-stack.json:compose_stack.services[0].image]`: PostgreSQL dependency image is pinned to `postgres:17-alpine`.
- `[tsuruyomi-byparr.json:compose_stack.service_count]`: Total service count is strictly 3.
- `[tsuruyomi-byparr.json:compose_stack.orchestration.reserve_vmids_atomically]`: Exactly 3 VMIDs must be atomically reserved.
- `[tsuruyomi-byparr.json:compose_stack.orchestration.start_order]`: Container startup sequence is strictly `['postgres', 'byparr', 'server']`.
- `[tsuruyomi-byparr.json:compose_stack.orchestration.stop_order]`: Container shutdown sequence is strictly `['server', 'byparr', 'postgres']`.

## Side Effects
- `[tsuruyomi.json:proxmox.installer_profile.volume_preparations]`: Removes `lost+found` directory and applies mapped-root ownership on persistent volumes `/config` and `/data`.
- `[tsuruyomi.json:lifecycle.update_strategy]`: Replaces the container rootfs from the new OCI image while preserving persistent volumes.
- `[tsuruyomi-stack.json:compose_stack.installer_inputs.generated_secrets]`: Generates a cryptographically random secret at installation time and binds it to `POSTGRES_PASSWORD`.
- `[tsuruyomi-stack.json:compose_stack.orchestration.rollback_on_failure]`: Removes provisioned rootfs on deployment failure while preserving created persistent volumes.
- `[tsuruyomi-stack.json:lifecycle.dependency_lifecycle]`: Installs a Proxmox hookscript on `main-lxc-pre-start` that blocks main startup until dependency healthcheck passes.
- `[tsuruyomi-byparr.json:compose_stack.installer_inputs.generated_secrets]`: Generates a cryptographically random secret at installation time and binds it to `POSTGRES_PASSWORD`.
- `[tsuruyomi-byparr.json:compose_stack.orchestration.rollback_on_failure]`: Deletes provisioned rootfs across the 3 LXCs on failure while preserving persistent volumes.
- `[tsuruyomi-byparr.json:lifecycle.dependency_lifecycle]`: Configures a Proxmox hookscript on `main-lxc-pre-start` blocking server startup until PostgreSQL and Byparr healthchecks pass.
- `[README.md:sync-tsuruyomi-catalog]`: Updates catalog configuration files in `/usr/local/bin` and Proxmox catalog paths.

## Verification
- Run: `python3 -m py_compile configure_tsuruyomi_catalog.py`
