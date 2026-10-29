#!/usr/bin/env bash
# Executado NA EC2 pela pipeline (via SSH) depois de copiar a imagem para
# /opt/lojagamer/releases/<tag>/. Carrega a imagem, troca o container e espera o
# health check. Se a versão nova não ficar saudável, volta para a anterior.
#
# Uso: bash deploy.sh <tag>
set -euo pipefail

TAG="${1:?informe a tag da imagem (ex.: o BuildId da pipeline)}"
APP_DIR=/opt/lojagamer
RELEASE_DIR="$APP_DIR/releases/$TAG"
ENV_FILE="$APP_DIR/.env"
NAME=lojagamer
IMAGE="lojagamer:$TAG"
HOST_PORT="${HOST_PORT:-80}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-120}"
KEEP_RELEASES=3

log() { echo "[deploy $(date -u +%H:%M:%S)] $*"; }

[[ -f "$ENV_FILE" ]] || { log "ERRO: $ENV_FILE não existe (rode o setup-ec2.sh)"; exit 1; }
[[ -f "$RELEASE_DIR/image.tar.gz" ]] || { log "ERRO: $RELEASE_DIR/image.tar.gz não encontrado"; exit 1; }

log "Carregando imagem $IMAGE"
gunzip -c "$RELEASE_DIR/image.tar.gz" | docker load

PREVIOUS_IMAGE="$(docker inspect --format '{{.Config.Image}}' "$NAME" 2>/dev/null || true)"
log "Versão anterior: ${PREVIOUS_IMAGE:-nenhuma}"

# Logs no CloudWatch são opcionais: basta definir CLOUDWATCH_LOG_GROUP no .env.
LOG_ARGS=()
LOG_GROUP="$(grep -E '^CLOUDWATCH_LOG_GROUP=' "$ENV_FILE" | cut -d= -f2- | tr -d '"' || true)"
REGION="$(grep -E '^AWS_REGION=' "$ENV_FILE" | cut -d= -f2- | tr -d '"' || true)"
if [[ -n "$LOG_GROUP" ]]; then
  LOG_ARGS=(--log-driver=awslogs --log-opt "awslogs-group=$LOG_GROUP" --log-opt "awslogs-region=${REGION:-us-east-1}"
    --log-opt awslogs-create-group=true --log-opt "awslogs-stream=$NAME-$TAG")
  log "Logs vão para o CloudWatch: $LOG_GROUP"
fi

start() {
  docker run -d --name "$NAME" --restart unless-stopped \
    --env-file "$ENV_FILE" -p "$HOST_PORT:8080" \
    "${LOG_ARGS[@]}" "$1" >/dev/null
}

wait_healthy() {
  local waited=0 status
  while (( waited < HEALTH_TIMEOUT )); do
    if [[ "$(docker inspect --format '{{.State.Running}}' "$NAME" 2>/dev/null)" != "true" ]]; then
      return 1 # o processo caiu (ex.: configuração inválida)
    fi
    status="$(docker inspect --format '{{.State.Health.Status}}' "$NAME" 2>/dev/null || echo missing)"
    case "$status" in
      healthy) return 0 ;;
      unhealthy | missing) return 1 ;;
    esac
    sleep 3
    waited=$((waited + 3))
  done
  return 1
}

log "Substituindo o container"
docker rm -f "$NAME" >/dev/null 2>&1 || true
start "$IMAGE"

if wait_healthy && curl -fsS "http://127.0.0.1:$HOST_PORT/healthz" >/dev/null; then
  log "Versão $TAG saudável:"
  curl -fsS "http://127.0.0.1:$HOST_PORT/healthz"
  echo
else
  log "ERRO: versão $TAG não ficou saudável. Últimos logs:"
  docker logs --tail 60 "$NAME" 2>&1 || true
  curl -sS "http://127.0.0.1:$HOST_PORT/healthz" || true
  echo
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  if [[ -n "$PREVIOUS_IMAGE" ]]; then
    log "ROLLBACK para $PREVIOUS_IMAGE"
    start "$PREVIOUS_IMAGE"
    wait_healthy && log "Rollback concluído" || log "ATENÇÃO: rollback também não ficou saudável"
  fi
  exit 1
fi

log "Limpando releases antigas (mantém $KEEP_RELEASES)"
ls -1dt "$APP_DIR"/releases/*/ 2>/dev/null | tail -n +$((KEEP_RELEASES + 1)) | xargs -r rm -rf
docker images lojagamer --format '{{.Repository}}:{{.Tag}}' | grep -v -x "$IMAGE" \
  | tail -n +"$KEEP_RELEASES" | xargs -r docker rmi >/dev/null 2>&1 || true

log "Deploy concluído"
