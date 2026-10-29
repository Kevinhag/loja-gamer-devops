#!/usr/bin/env bash
# Prepara uma EC2 nova (Amazon Linux 2023 ou Ubuntu) para receber os deploys da pipeline.
# Uso (da sua máquina):
#   scp -i chave.pem deploy/setup-ec2.sh deploy/.env.example ec2-user@<IP>:~
#   ssh -i chave.pem ec2-user@<IP> "sudo bash setup-ec2.sh"
set -euo pipefail

APP_DIR=/opt/lojagamer
DEPLOY_USER="${SUDO_USER:-ec2-user}"

if [[ $EUID -ne 0 ]]; then
  echo "Rode com sudo." >&2
  exit 1
fi

echo "==> Instalando Docker"
if ! command -v docker >/dev/null 2>&1; then
  if command -v dnf >/dev/null 2>&1; then
    dnf install -y docker
  elif command -v apt-get >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y docker.io
  else
    echo "Distribuição não suportada; instale o Docker manualmente." >&2
    exit 1
  fi
fi
systemctl enable --now docker
usermod -aG docker "$DEPLOY_USER"

echo "==> Criando $APP_DIR"
mkdir -p "$APP_DIR/releases"
chown -R "$DEPLOY_USER:$DEPLOY_USER" "$APP_DIR"

if [[ ! -f "$APP_DIR/.env" ]]; then
  if [[ -f "$(dirname "$0")/.env.example" ]]; then
    cp "$(dirname "$0")/.env.example" "$APP_DIR/.env"
  else
    touch "$APP_DIR/.env"
  fi
  chown "$DEPLOY_USER:$DEPLOY_USER" "$APP_DIR/.env"
  chmod 600 "$APP_DIR/.env"
  echo "==> Criado $APP_DIR/.env (edite com os dados do RDS e do S3)"
fi

echo
docker --version
echo
echo "Pronto. Próximos passos:"
echo "  1. nano $APP_DIR/.env    # preencha DB_HOST, DB_PASSWORD, S3_BUCKET, ADMIN_TOKEN"
echo "  2. saia e entre de novo na sessão SSH (para o grupo docker valer)"
echo "  3. rode a pipeline no Azure DevOps"
