#!/usr/bin/env bash
set -Eeuo pipefail

BLUE=$'\033[0;34m'
YELLOW=$'\033[1;33m'
RED=$'\033[0;31m'
NC=$'\033[0m'

log() { echo -e "${BLUE}->${NC} $*"; }
warn() { echo -e "${YELLOW}WARN:${NC} $*"; }
fail() { echo -e "${RED}ERROR:${NC} $*"; exit 1; }

require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || fail "Required command not found: $cmd"
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TF_DIR="${REPO_ROOT}/../../terraform/oci"
SSH_USER="${ARCH_DEPLOY_SSH_USER:-ubuntu}"
SSH_KEY="${ARCH_DEPLOY_SSH_KEY:-$HOME/.ssh/oci_terraform_ed25519}"
AWS_ENV_FILE=""
HOSTS_OVERRIDE=""

usage() {
  cat <<'USAGE'
Usage: scripts/deploy-oci-stack.sh [options]

Options:
  --terraform-dir <path>  Terraform OCI directory (default: ../../terraform/oci)
  --hosts <csv|space>     Override target hosts instead of terraform output
  --ssh-user <user>       SSH user for OCI instances (default: ubuntu)
  --ssh-key <path>        SSH private key path (default: ~/.ssh/oci_terraform_ed25519)
  --aws-env-file <path>   Optional env file appended to /etc/arch/arch.env
  --help                  Show this help

Behavior:
  1) Builds backend binary (linux/amd64) and frontend static bundle.
  2) Deploys both to each target VM under /opt/arch/releases/<timestamp>.
  3) Configures systemd service (arch-backend) and nginx reverse proxy.
  4) Exposes frontend on port 80 and proxies /api,/healthz,/readyz to 127.0.0.1:8081.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --terraform-dir)
      TF_DIR="${2:-}"
      shift 2
      ;;
    --hosts)
      HOSTS_OVERRIDE="${2:-}"
      shift 2
      ;;
    --ssh-user)
      SSH_USER="${2:-}"
      shift 2
      ;;
    --ssh-key)
      SSH_KEY="${2:-}"
      shift 2
      ;;
    --aws-env-file)
      AWS_ENV_FILE="${2:-}"
      shift 2
      ;;
    --help)
      usage
      exit 0
      ;;
    *)
      fail "Unknown argument: $1"
      ;;
  esac
done

require_cmd ssh
require_cmd rsync
require_cmd jq
require_cmd terraform
require_cmd go
require_cmd npm

[[ -f "$SSH_KEY" ]] || fail "SSH key not found: $SSH_KEY"
[[ -d "$TF_DIR" ]] || fail "Terraform OCI directory not found: $TF_DIR"
if [[ -n "$AWS_ENV_FILE" ]]; then
  [[ -f "$AWS_ENV_FILE" ]] || fail "AWS env file not found: $AWS_ENV_FILE"
fi

declare -a HOSTS=()
if [[ -n "$HOSTS_OVERRIDE" ]]; then
  mapfile -t HOSTS < <(echo "$HOSTS_OVERRIDE" | tr ',' ' ' | xargs -n1)
else
  mapfile -t HOSTS < <(terraform -chdir="$TF_DIR" output -json instance_public_ips | jq -r '.[]')
fi
[[ ${#HOSTS[@]} -gt 0 ]] || fail "No target hosts resolved."

RELEASE_TAG="$(date -u +'%Y%m%d%H%M%S')"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/arch-oci-deploy.${RELEASE_TAG}.XXXXXX")"
RELEASE_DIR="${WORK_DIR}/release"
mkdir -p "${RELEASE_DIR}/backend" "${RELEASE_DIR}/frontend" "${RELEASE_DIR}/config"
trap 'rm -rf "$WORK_DIR"' EXIT

log "Building arch backend binary (linux/amd64)..."
(
  cd "$REPO_ROOT"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "${RELEASE_DIR}/backend/arch" ./cmd/arch
)
chmod +x "${RELEASE_DIR}/backend/arch"

log "Building frontend bundle..."
(
  cd "${REPO_ROOT}/frontend"
  npm ci
  npm run build
)

[[ -x "${RELEASE_DIR}/backend/arch" ]] || fail "Backend binary missing: ${RELEASE_DIR}/backend/arch"
[[ -d "${REPO_ROOT}/frontend/dist" ]] || fail "Frontend dist not found: ${REPO_ROOT}/frontend/dist"
rsync -a --delete "${REPO_ROOT}/frontend/dist/" "${RELEASE_DIR}/frontend/"

cat > "${RELEASE_DIR}/config/arch.env" <<'EOF'
ARCH_HTTP_ADDR=127.0.0.1:8081
ARCH_DISCOVERY_MODE=static
ARCH_AWS_VALIDATE_ON_START=false
EOF

if [[ -n "$AWS_ENV_FILE" ]]; then
  log "Appending AWS env overlay from ${AWS_ENV_FILE}"
  cat "$AWS_ENV_FILE" >> "${RELEASE_DIR}/config/arch.env"
fi

cat > "${RELEASE_DIR}/config/arch-backend.service" <<'EOF'
[Unit]
Description=Arch GoCools Backend
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/arch/arch.env
ExecStart=/opt/arch/current/backend/arch
Restart=always
RestartSec=3
User=root
Group=root
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

cat > "${RELEASE_DIR}/config/nginx-arch.conf" <<'EOF'
server {
  listen 80 default_server;
  listen [::]:80 default_server;
  server_name _;

  root /opt/arch/current/frontend;
  index index.html;

  location /api/ {
    proxy_pass http://127.0.0.1:8081;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
  }

  location = /healthz {
    proxy_pass http://127.0.0.1:8081/healthz;
  }

  location = /readyz {
    proxy_pass http://127.0.0.1:8081/readyz;
  }

  location / {
    try_files $uri /index.html;
  }
}
EOF

SSH_OPTS=(
  -i "$SSH_KEY"
  -o BatchMode=yes
  -o StrictHostKeyChecking=accept-new
  -o ConnectTimeout=15
)

for host in "${HOSTS[@]}"; do
  log "Deploying release ${RELEASE_TAG} to ${host}..."
  REMOTE_TMP="/tmp/arch-release-${RELEASE_TAG}"
  REMOTE_RELEASE="/opt/arch/releases/${RELEASE_TAG}"

  ssh "${SSH_OPTS[@]}" "${SSH_USER}@${host}" "mkdir -p '${REMOTE_TMP}'"
  rsync -az --delete -e "ssh ${SSH_OPTS[*]}" "${RELEASE_DIR}/" "${SSH_USER}@${host}:${REMOTE_TMP}/"

  ssh "${SSH_OPTS[@]}" "${SSH_USER}@${host}" "bash -s" <<EOF
set -Eeuo pipefail
if command -v apt-get >/dev/null 2>&1; then
  sudo DEBIAN_FRONTEND=noninteractive apt-get update -y
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y nginx curl rsync ca-certificates
fi

# OCI Ubuntu images ship with restrictive iptables defaults (SSH-only).
if command -v iptables >/dev/null 2>&1; then
  for port in 80 443; do
    sudo iptables -C INPUT -p tcp --dport "\$port" -j ACCEPT 2>/dev/null || sudo iptables -I INPUT 4 -p tcp --dport "\$port" -j ACCEPT
  done
  sudo mkdir -p /etc/iptables
  sudo sh -c "iptables-save > /etc/iptables/rules.v4"
fi

sudo mkdir -p /opt/arch/releases /etc/arch /etc/nginx/sites-available /etc/nginx/sites-enabled
sudo rm -rf "${REMOTE_RELEASE}"
sudo mkdir -p "${REMOTE_RELEASE}"
sudo rsync -a "${REMOTE_TMP}/" "${REMOTE_RELEASE}/"
sudo ln -sfn "${REMOTE_RELEASE}" /opt/arch/current

sudo install -m 0644 /opt/arch/current/config/arch-backend.service /etc/systemd/system/arch-backend.service
sudo install -m 0600 /opt/arch/current/config/arch.env /etc/arch/arch.env
sudo install -m 0644 /opt/arch/current/config/nginx-arch.conf /etc/nginx/sites-available/arch
sudo ln -sfn /etc/nginx/sites-available/arch /etc/nginx/sites-enabled/arch
sudo rm -f /etc/nginx/sites-enabled/default

sudo systemctl daemon-reload
sudo systemctl enable --now arch-backend
sudo systemctl restart arch-backend
sudo nginx -t
sudo systemctl enable --now nginx
sudo systemctl restart nginx

curl -fsS http://127.0.0.1/healthz >/dev/null
curl -fsS http://127.0.0.1/readyz >/dev/null
EOF

  log "Host ${host} deployed successfully."
done

echo
log "Deployment complete."
for host in "${HOSTS[@]}"; do
  echo "  http://${host}/"
  echo "  http://${host}/healthz"
  echo "  http://${host}/api/v1/graph"
done
