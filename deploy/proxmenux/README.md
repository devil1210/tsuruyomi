# Tsuruyomi en ProxMenux (Proxmox VE OCI Manager)

ProxMenux permite desplegar imágenes OCI oficiales directamente como contenedores LXC nativos en Proxmox VE.

Hemos configurado **3 versiones oficiales** en el catálogo de ProxMenux para adaptarse a cada infraestructura:

---

## 1. Tsuruyomi (Solo App / Standalone) — `tsuruyomi`
> **Recomendado si ya tienes tu propio servidor de base de datos** (como tu contenedor LXC `150 postgres-core`).

- **Tipo:** Contenedor LXC individual (1 solo CT).
- **Ventajas:**
  - **Sin dependencias ni hookscripts:** No crea un CT secundario ni añade scripts de enganche a Proxmox. Puedes reiniciar, apagar o modificar tus otros contenedores sin que Tsuruyomi falle.
  - **Actualizaciones 100% limpias:** Al actualizar desde ProxMenux, solo descarga la nueva imagen de Tsuruyomi (`ghcr.io/devil1210/tsuruyomi:latest`) y actualiza el rootfs manteniendo intactos tus volúmenes persistentes (`/config` y `/data`). **Nunca más dará error de versión mayor de PostgreSQL.**
- **Variables configurables al instalar:**
  - `DATABASE_DSN`: `postgres://tsuruyomi:tu_clave@192.168.1.150:5432/tsuruyomi?sslmode=disable`
  - `SERVER_PORT`: `7654`
  - `TZ`: `America/Santiago`
  - `FLARESOLVERR_ENABLED`: `true`
  - `FLARESOLVERR_URL`: `http://192.168.1.136:8191` (o tu instancia de Byparr)

---

## 2. Tsuruyomi Stack (App + PostgreSQL) — `tsuruyomi-stack`
> Para instalaciones autónomas donde no existe un servidor PostgreSQL previo.

- **Tipo:** Stack multi-LXC automático (2 contenedores CT: Tsuruyomi + PostgreSQL 17).
- **Ventajas:**
  - Levanta una red privada interna y crea automáticamente la base de datos PostgreSQL 17 con contraseñas seguras generadas.
  - Se fijó la imagen a `postgres:17-alpine` tanto en el contrato como en el stack para prevenir saltos imprevistos de versión mayor.

---

## 3. Tsuruyomi All-in-One (App + PostgreSQL + Byparr) — `tsuruyomi-byparr`
> Stack completo todo en uno con bypass de Cloudflare integrado.

- **Tipo:** Stack multi-LXC automático (3 contenedores CT: Tsuruyomi + PostgreSQL 17 + Byparr).
- **Ventajas:**
  - Incluye `byparr` (FlareSolverr v3) preconfigurado en la red privada interna para resolver automáticamente retos de Cloudflare Turnstile en fuentes protegidas.

---

## Sincronización en Proxmox

Si alguna vez actualizas ProxMenux (`install_proxmenux_beta.sh --update`) y se sobreescribe el catálogo, puedes restaurar las 3 opciones ejecutando en el nodo Proxmox:

```bash
sync-tsuruyomi-catalog
```
O manualmente:
```bash
python3 /usr/local/bin/sync-tsuruyomi-catalog
```
