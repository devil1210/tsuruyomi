import json
import copy
from pathlib import Path

BASE = Path("/usr/local/share/proxmenux/oci/engine/catalog")
APPS_DIR = BASE / "apps"
CURATED_DIR = BASE / "curated"
INDEX_FILE = BASE / "index.json"
CATEGORIES_FILE = BASE / "categories.json"

# Load base template
base_template = json.load(open("/root/tsuruyomi_original_backup.json"))

# -------------------------------------------------------------
# 1. Template: tsuruyomi (Solo App / Standalone)
# -------------------------------------------------------------
standalone = copy.deepcopy(base_template)
standalone["id"] = "image-tsuruyomi"
standalone["catalog_ui"]["title"] = {
    "en_US": "Tsuruyomi (Standalone)",
    "es_ES": "Tsuruyomi (Solo App / Standalone)"
}
standalone["catalog_ui"]["tagline"] = {
    "en_US": "Lightweight Manga Reader and OPDS Server (Standalone)",
    "es_ES": "Lector de manga y catálogo OPDS independiente (usa tu propio PostgreSQL)"
}
standalone["catalog_ui"]["description"] = {
    "en_US": "Tsuruyomi standalone container. Connects to your existing external PostgreSQL database (e.g. CT 150 postgres-core). No dependent containers, no hookscripts, independent and clean updates.",
    "es_ES": "Contenedor independiente de Tsuruyomi. Se conecta directamente a tu base de datos PostgreSQL existente (como tu CT 150 postgres-core). Sin dependencias de otros contenedores ni hookscripts, garantizando arranques y actualizaciones limpias sin errores."
}
standalone["catalog_ui"]["tips"] = [
    "Ideal para conectar a tu CT 150 de PostgreSQL existente o servidor externo.",
    "Compatible con KOReader y Moon+ Reader vía /api/opds/v1.2 y /api/opds/v2.0.",
    "Al actualizar por ProxMenux, solo actualiza la app Tsuruyomi sin tocar la base de datos."
]
standalone["container_contract"]["related_services"] = []
standalone["container_contract"]["environment"] = [
    {
        "name": "DATABASE_DSN",
        "example": "postgres://tsuruyomi:tsuruyomi_pass@192.168.1.150:5432/tsuruyomi?sslmode=disable",
        "required": True,
        "sensitive": False,
        "source": "docker-compose"
    },
    {
        "name": "SERVER_PORT",
        "example": "7654",
        "required": True,
        "sensitive": False,
        "source": "docker-compose"
    },
    {
        "name": "TZ",
        "example": "$TZ",
        "required": True,
        "sensitive": False,
        "source": "docker-compose"
    },
    {
        "name": "FLARESOLVERR_ENABLED",
        "example": "true",
        "required": False,
        "sensitive": False,
        "source": "docker-compose"
    },
    {
        "name": "FLARESOLVERR_URL",
        "example": "http://192.168.1.136:8191",
        "required": False,
        "sensitive": False,
        "source": "docker-compose"
    }
]
standalone["container_contract"]["original_compose"] = """name: tsuruyomi
services:
  tsuruyomi:
    image: ghcr.io/devil1210/tsuruyomi:latest
    container_name: tsuruyomi
    restart: unless-stopped
    ports:
    - target: 7654
      published:  7654
      protocol: tcp
    environment:
      SERVER_PORT: 7654
      DATABASE_DSN: postgres://tsuruyomi:tsuruyomi_pass@192.168.1.150:5432/tsuruyomi?sslmode=disable
      TZ: $TZ
      FLARESOLVERR_ENABLED: true
      FLARESOLVERR_URL: http://192.168.1.136:8191
    volumes:
    - type: bind
      source: /DATA/AppData/$AppID/config
      target: /config
    - type: bind
      source: /DATA/AppData/$AppID/data
      target: /data
"""

# Remove stack & multi-lxc properties
if "compose_stack" in standalone:
    del standalone["compose_stack"]

standalone["proxmox"]["installer_profile"] = {
    "volume_preparations": [
        {
            "container_path": "/config",
            "remove_lost_found": True,
            "owner_strategy": "mapped-root",
            "only_when_mount_type": "managed-volume"
        },
        {
            "container_path": "/data",
            "remove_lost_found": True,
            "owner_strategy": "mapped-root",
            "only_when_mount_type": "managed-volume"
        }
    ]
}

standalone["lifecycle"] = {
    "update_strategy": "replace-rootfs-from-new-oci-image-preserve-managed-volumes",
    "registry_state": {
        "resolved_architecture": None,
        "resolved_digest": None,
        "image_version_label": None,
        "image_created": None
    },
    "change_detection": "compare-resolved-architecture-digest",
    "automatic_unattended_updates": False
}

with open(APPS_DIR / "tsuruyomi.json", "w") as f:
    json.dump(standalone, f, indent=2)
with open(CURATED_DIR / "tsuruyomi.json", "w") as f:
    json.dump(standalone, f, indent=2)

print("[OK] Created tsuruyomi (standalone)")

# -------------------------------------------------------------
# 2. Template: tsuruyomi-stack (App + PostgreSQL)
# -------------------------------------------------------------
stack = copy.deepcopy(base_template)
stack["id"] = "image-tsuruyomi-stack"
stack["catalog_ui"]["title"] = {
    "en_US": "Tsuruyomi Stack",
    "es_ES": "Tsuruyomi Stack (App + PostgreSQL)"
}
stack["catalog_ui"]["tagline"] = {
    "en_US": "Tsuruyomi Manga Reader with Dedicated PostgreSQL 17",
    "es_ES": "Lector de manga Tsuruyomi con PostgreSQL 17 dedicado automático"
}
stack["catalog_ui"]["description"] = {
    "en_US": "Complete 2-LXC stack with Tsuruyomi and automatic private PostgreSQL 17 container.",
    "es_ES": "Stack completo de 2 contenedores LXC con Tsuruyomi y base de datos privada PostgreSQL 17 creada automáticamente."
}
# Fix postgres image to 17-alpine strictly
for svc in stack.get("compose_stack", {}).get("services", []):
    if svc.get("name") == "postgres":
        svc["image"] = "postgres:17-alpine"
        svc["compose"]["image"] = "postgres:17-alpine"

with open(APPS_DIR / "tsuruyomi-stack.json", "w") as f:
    json.dump(stack, f, indent=2)
with open(CURATED_DIR / "tsuruyomi-stack.json", "w") as f:
    json.dump(stack, f, indent=2)

print("[OK] Created tsuruyomi-stack (2 LXC)")

# -------------------------------------------------------------
# 3. Template: tsuruyomi-byparr (App + PostgreSQL + Byparr)
# -------------------------------------------------------------
byparr_stack = copy.deepcopy(base_template)
byparr_stack["id"] = "image-tsuruyomi-byparr"
byparr_stack["catalog_ui"]["title"] = {
    "en_US": "Tsuruyomi All-in-One",
    "es_ES": "Tsuruyomi All-in-One (App + PostgreSQL + Byparr)"
}
byparr_stack["catalog_ui"]["tagline"] = {
    "en_US": "Tsuruyomi with PostgreSQL 17 and Byparr Cloudflare Bypass",
    "es_ES": "Stack completo: Tsuruyomi + PostgreSQL 17 + Byparr (Bypass Cloudflare)"
}
byparr_stack["catalog_ui"]["description"] = {
    "en_US": "All-in-one 3-LXC stack including Tsuruyomi, dedicated PostgreSQL 17, and Byparr for automated Cloudflare Turnstile bypass.",
    "es_ES": "Stack todo en uno de 3 contenedores LXC: Tsuruyomi, PostgreSQL 17 dedicado y Byparr preconfigurado para bypass automático de Cloudflare."
}

# Update compose_stack to include Byparr
cs = byparr_stack.get("compose_stack", {})
cs["service_count"] = 3
cs["orchestration"]["reserve_vmids_atomically"] = 3
cs["orchestration"]["start_order"] = ["postgres", "byparr", "server"]
cs["orchestration"]["stop_order"] = ["server", "byparr", "postgres"]

# Update server environment
for svc in cs.get("services", []):
    if svc.get("name") == "server":
        svc["compose"]["environment"]["FLARESOLVERR_ENABLED"] = "true"
        svc["compose"]["environment"]["FLARESOLVERR_URL"] = "http://byparr:8191"
        if "depends_on" in svc["compose"]:
            svc["compose"]["depends_on"]["byparr"] = {"condition": "service_started"}

# Add Byparr service definition
byparr_service = {
    "name": "byparr",
    "image": "ghcr.io/thephaseless/byparr:latest",
    "is_main": False,
    "role": "dependency",
    "vmid_offset": 2,
    "depends_on": [],
    "frontend_network": False,
    "private_network": True,
    "compose": {
        "image": "ghcr.io/thephaseless/byparr:latest",
        "container_name": "byparr",
        "restart": "unless-stopped",
        "environment": {
            "LOG_LEVEL": "info",
            "PORT": "8191",
            "TZ": "$TZ"
        },
        "networks": ["tsuruyomi-net"],
        "ports": [
            {
                "target": 8191,
                "published": "8191",
                "protocol": "tcp"
            }
        ],
        "deploy": {
            "resources": {
                "reservations": {
                    "memory": "512m"
                }
            }
        }
    },
    "lxc_configuration": {
        "cores": 2,
        "memory_mb": 512,
        "swap_mb": 256,
        "rootfs_size_gb": 4,
        "unprivileged": True,
        "features": ["nesting=1"]
    }
}
cs["services"].append(byparr_service)

byparr_stack["container_contract"]["related_services"] = [
    {"name": "tsuruyomi-postgres", "image": "postgres:17-alpine"},
    {"name": "byparr", "image": "ghcr.io/thephaseless/byparr:latest"}
]

with open(APPS_DIR / "tsuruyomi-byparr.json", "w") as f:
    json.dump(byparr_stack, f, indent=2)
with open(CURATED_DIR / "tsuruyomi-byparr.json", "w") as f:
    json.dump(byparr_stack, f, indent=2)

print("[OK] Created tsuruyomi-byparr (3 LXC)")

# -------------------------------------------------------------
# 4. Update index.json
# -------------------------------------------------------------
index_data = json.load(open(INDEX_FILE))
apps = index_data.get("applications", [])

# Remove any existing tsuruyomi entries
apps = [a for a in apps if not a.get("id", "").startswith("tsuruyomi")]

# Add standalone
apps.append({
    "id": "tsuruyomi",
    "source_app_id": "tsuruyomi",
    "provider": "official",
    "template_family": "curated-profile",
    "variant": None,
    "title": "Tsuruyomi (Solo App / Standalone)",
    "repository": "https://github.com/devil1210/tsuruyomi",
    "description": "Lector de manga y catálogo OPDS en Go (Contenedor único independiente para conectar a tu propia base de datos PostgreSQL, ej. CT 150).",
    "website": "https://github.com/devil1210/tsuruyomi",
    "icon": "https://raw.githubusercontent.com/devil1210/tsuruyomi/main/data/webUI/apple-touch-icon.png",
    "architectures": ["amd64", "arm64"],
    "default_branch": "main",
    "source_path": "catalog/curated/tsuruyomi.json",
    "curated_path": "catalog/curated/tsuruyomi.json",
    "main_image": "ghcr.io/devil1210/tsuruyomi:latest",
    "category": "media",
    "category_label": "Media & Streaming",
    "template": "apps/tsuruyomi.json",
    "template_status": "laboratory-validated",
    "automatic_install_candidate": True,
    "multi_container": False,
    "untranslated_blockers": [],
    "requires_privileged_lxc": False,
    "optional_privileged_lxc": False,
    "requires_host_pid_namespace": False,
    "requires_relaxed_confinement": False,
    "optional_relaxed_confinement": False,
    "requires_security_confirmation": False,
    "hidden": False,
    "template_id": "image-tsuruyomi"
})

# Add stack (2 LXC)
apps.append({
    "id": "tsuruyomi-stack",
    "source_app_id": "tsuruyomi-stack",
    "provider": "official",
    "template_family": "curated-profile",
    "variant": None,
    "title": "Tsuruyomi Stack (App + PostgreSQL)",
    "repository": "https://github.com/devil1210/tsuruyomi",
    "description": "Stack de 2 contenedores LXC: Tsuruyomi con base de datos PostgreSQL 17 dedicada automática.",
    "website": "https://github.com/devil1210/tsuruyomi",
    "icon": "https://raw.githubusercontent.com/devil1210/tsuruyomi/main/data/webUI/apple-touch-icon.png",
    "architectures": ["amd64", "arm64"],
    "default_branch": "main",
    "source_path": "catalog/curated/tsuruyomi-stack.json",
    "curated_path": "catalog/curated/tsuruyomi-stack.json",
    "main_image": "ghcr.io/devil1210/tsuruyomi:latest",
    "category": "media",
    "category_label": "Media & Streaming",
    "template": "apps/tsuruyomi-stack.json",
    "template_status": "laboratory-validated",
    "automatic_install_candidate": True,
    "multi_container": True,
    "untranslated_blockers": [],
    "requires_privileged_lxc": False,
    "optional_privileged_lxc": False,
    "requires_host_pid_namespace": False,
    "requires_relaxed_confinement": False,
    "optional_relaxed_confinement": False,
    "requires_security_confirmation": False,
    "hidden": False,
    "template_id": "image-tsuruyomi-stack"
})

# Add byparr stack (3 LXC)
apps.append({
    "id": "tsuruyomi-byparr",
    "source_app_id": "tsuruyomi-byparr",
    "provider": "official",
    "template_family": "curated-profile",
    "variant": None,
    "title": "Tsuruyomi All-in-One (App + PostgreSQL + Byparr)",
    "repository": "https://github.com/devil1210/tsuruyomi",
    "description": "Stack todo en uno de 3 contenedores LXC: Tsuruyomi + PostgreSQL 17 + Byparr (Bypass FlareSolverr Cloudflare).",
    "website": "https://github.com/devil1210/tsuruyomi",
    "icon": "https://raw.githubusercontent.com/devil1210/tsuruyomi/main/data/webUI/apple-touch-icon.png",
    "architectures": ["amd64", "arm64"],
    "default_branch": "main",
    "source_path": "catalog/curated/tsuruyomi-byparr.json",
    "curated_path": "catalog/curated/tsuruyomi-byparr.json",
    "main_image": "ghcr.io/devil1210/tsuruyomi:latest",
    "category": "media",
    "category_label": "Media & Streaming",
    "template": "apps/tsuruyomi-byparr.json",
    "template_status": "laboratory-validated",
    "automatic_install_candidate": True,
    "multi_container": True,
    "untranslated_blockers": [],
    "requires_privileged_lxc": False,
    "optional_privileged_lxc": False,
    "requires_host_pid_namespace": False,
    "requires_relaxed_confinement": False,
    "optional_relaxed_confinement": False,
    "requires_security_confirmation": False,
    "hidden": False,
    "template_id": "image-tsuruyomi-byparr"
})

index_data["applications"] = apps
with open(INDEX_FILE, "w") as f:
    json.dump(index_data, f, indent=2)

print("[OK] Updated index.json with 3 Tsuruyomi catalog options")

# -------------------------------------------------------------
# 5. Update categories.json
# -------------------------------------------------------------
try:
    cats = json.load(open(CATEGORIES_FILE))
    cats["tsuruyomi"] = "media"
    cats["tsuruyomi-stack"] = "media"
    cats["tsuruyomi-byparr"] = "media"
    with open(CATEGORIES_FILE, "w") as f:
        json.dump(cats, f, indent=2)
    print("[OK] Updated categories.json")
except Exception as e:
    print(f"[WARN] categories.json: {e}")

