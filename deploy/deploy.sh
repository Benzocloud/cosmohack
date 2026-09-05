#!/usr/bin/env bash
# Развёртывание одного выпуска из двух immutable digest с проверкой
# готовности обеих служб и откатом предыдущей совместимой пары с моделью.
# Порядок по .agent/plan.md (CI/CD): применить миграции, пока текущий Go
# остаётся доступен → остановить приём анализов → обновить ML и проверить
# /readyz с версиями →
# запустить Go и проверить API.
#
# Использование:
#   GO_IMAGE=ghcr.io/org/go@sha256:... ML_IMAGE=ghcr.io/org/ml@sha256:... \
#   MODEL_VERSION=... ./deploy/deploy.sh
#
# Требуется: docker, docker compose, curl. Порт ML наружу не публикуется,
# поэтому его готовность проверяется через compose exec внутри сети.
# Первая неудачная установка без предыдущей версии считается ошибкой.
set -euo pipefail

cd "$(dirname "$0")"

# CI places registry and database credentials in this 0600 file. Loading it
# before validation avoids passing DATABASE_URL through sudo's environment.
if [[ -f release-ghcr.env ]]; then
  # shellcheck disable=SC1091
  source release-ghcr.env
fi

GO_IMAGE=${GO_IMAGE:?set GO_IMAGE to the go image digest}
ML_IMAGE=${ML_IMAGE:?set ML_IMAGE to the ml image digest}
MODEL_VERSION=${MODEL_VERSION:?set MODEL_VERSION to the model version in the release manifest}
EXPECTED_SCHEMA=${EXPECTED_SCHEMA:-1.0}
EXPECTED_PROFILE=${EXPECTED_PROFILE:-ndvi-weather-v1}
APP_PORT=${APP_PORT:-8080}
DATABASE_URL=${DATABASE_URL:?set DATABASE_URL for the PostgreSQL deployment}
CDSE_CLIENT_ID=${CDSE_CLIENT_ID:?set CDSE_CLIENT_ID for the satellite provider}
CDSE_CLIENT_SECRET=${CDSE_CLIENT_SECRET:?set CDSE_CLIENT_SECRET for the satellite provider}
DB_TIMEOUT=${DB_TIMEOUT:-5s}
MIGRATIONS_DIR_HOST=${MIGRATIONS_DIR_HOST:-../backend/migrations}
CURRENT_MANIFEST=current-manifest.json
PREVIOUS_MANIFEST=previous-manifest.json
COMPOSE=(docker compose --env-file release.env -f compose.yaml)

# Приватный GHCR: для pull пакетов серверу нужен токен с read:packages.
# GHCR_USER — имя владельца или machine user, GHCR_TOKEN — read-only токен.
GHCR_USER=${GHCR_USER:?set GHCR_USER for the private registry}
GHCR_TOKEN=${GHCR_TOKEN:?set GHCR_TOKEN (read:packages) for the private registry}

log() { printf '[deploy] %s\n' "$*"; }

write_release_env() {
  umask 077
  cat > release.env <<EOF
GO_IMAGE=${GO_IMAGE}
ML_IMAGE=${ML_IMAGE}
MODEL_VERSION=${MODEL_VERSION}
APP_PORT=${APP_PORT}
DATABASE_URL=${DATABASE_URL}
CDSE_CLIENT_ID=${CDSE_CLIENT_ID}
CDSE_CLIENT_SECRET=${CDSE_CLIENT_SECRET}
DB_TIMEOUT=${DB_TIMEOUT}
MIGRATIONS_DIR_HOST=${MIGRATIONS_DIR_HOST}
EOF
  chmod 600 release.env
}

write_manifest() {
  cat > "$CURRENT_MANIFEST" <<EOF
{
  "go_image": "${GO_IMAGE}",
  "ml_image": "${ML_IMAGE}",
  "schema_version": "${EXPECTED_SCHEMA}",
  "feature_profile": "${EXPECTED_PROFILE}",
  "model_version": "${MODEL_VERSION}",
  "deployed_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF
}

wait_http() {
  local name=$1 url=$2 attempts=$3
  for _ in $(seq 1 "$attempts"); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  log "${name} readiness failed: ${url}"
  return 1
}

# check_ml_versions выполняется внутри контейнера ml: наружу порт закрыт.
# Тело разбирается как JSON: состав feature_profiles может расширяться.
check_ml_versions() {
  local script="import json, urllib.request
body = json.load(urllib.request.urlopen('http://127.0.0.1:8000/readyz'))
print(body)
assert body.get('status') == 'ready', body
assert body.get('schema_version') == '${EXPECTED_SCHEMA}', body
assert '${EXPECTED_PROFILE}' in body.get('feature_profiles', []), body
assert body.get('model_version') == '${MODEL_VERSION}', body"
  "${COMPOSE[@]}" exec -T ml python -c "$script"
}

check_migration_state() {
  local state version dirty
  state=$("${COMPOSE[@]}" exec -T postgres \
    psql "$DATABASE_URL" -Atc "SELECT version || ':' || CASE WHEN dirty THEN 't' ELSE 'f' END FROM schema_migrations ORDER BY version DESC LIMIT 1")
  version=${state%%:*}
  dirty=${state##*:}
  [[ "$version" =~ ^[1-9][0-9]*$ && "$dirty" == "f" ]]
}

rollback() {
  if [[ ! -f $PREVIOUS_MANIFEST ]]; then
    log "ROLLBACK IMPOSSIBLE: no previous manifest; first install failed"
    exit 1
  fi
  log "rolling back to the previous compatible pair"
  GO_IMAGE=$(sed -n 's/.*"go_image": *"\([^"]*\)".*/\1/p' "$PREVIOUS_MANIFEST")
  ML_IMAGE=$(sed -n 's/.*"ml_image": *"\([^"]*\)".*/\1/p' "$PREVIOUS_MANIFEST")
  MODEL_VERSION=$(sed -n 's/.*"model_version": *"\([^"]*\)".*/\1/p' "$PREVIOUS_MANIFEST")
  write_release_env
  "${COMPOSE[@]}" up -d
  if wait_http "go(rollback)" "http://127.0.0.1:${APP_PORT}/readyz" 30; then
    log "rollback finished; investigate the failed release"
  else
    log "rollback did not become ready; manual intervention required"
  fi
  exit 1
}

main() {
  local img
  for img in "$GO_IMAGE" "$ML_IMAGE"; do
    case $img in
      *@sha256:*) ;;
      *) log "images must be pinned by digest: ${img}"; exit 1 ;;
    esac
  done

  command -v docker >/dev/null || { log "docker not found"; exit 1; }
  docker compose version >/dev/null || { log "docker compose not found"; exit 1; }

  log "logging in to the private ghcr.io"
  if ! echo "$GHCR_TOKEN" | docker login ghcr.io --username "$GHCR_USER" --password-stdin; then
    log "registry login failed; current release remains running"
    exit 1
  fi

  log "pulling images"
  if ! docker pull "$GO_IMAGE"; then
    log "go image pull failed; current release remains running"
    exit 1
  fi
  if ! docker pull "$ML_IMAGE"; then
    log "ml image pull failed; current release remains running"
    exit 1
  fi

  [[ -f $CURRENT_MANIFEST ]] && cp "$CURRENT_MANIFEST" "$PREVIOUS_MANIFEST"
  write_release_env

  install -d -m 0755 "$MIGRATIONS_DIR_HOST"

  log "applying database migrations"
  if ! "${COMPOSE[@]}" run --rm migrate; then
    log "database migration failed; current release remains running"
    exit 1
  fi
  if ! check_migration_state; then
    log "database migration state is dirty or unavailable; current release remains running"
    exit 1
  fi

  log "stopping go (analysis intake paused)"
  "${COMPOSE[@]}" stop go || true

  log "updating ml"
  "${COMPOSE[@]}" up -d ml || rollback

  log "checking ml readiness and versions"
  if ! check_ml_versions; then
    log "ml readiness/versions check failed"
    rollback
  fi

  log "starting go"
  "${COMPOSE[@]}" up -d --no-deps go || rollback
  wait_http "go" "http://127.0.0.1:${APP_PORT}/readyz" 30 || rollback
  if ! curl -fsS "http://127.0.0.1:${APP_PORT}/api/areas" >/dev/null; then
    log "go api check failed"
    rollback
  fi

  # Манифест фиксируется только после успешных проверок, чтобы провалившийся
  # выпуск не попал в previous-manifest следующего деплоя.
  write_manifest
  log "release deployed: go=${GO_IMAGE} ml=${ML_IMAGE} model=${MODEL_VERSION}"
}

main "$@"
