#!/usr/bin/env bash
# Santaizi Dashboard 本地 Primary 启动脚本
#
# 用法:
#   ./script/start-local.sh              # go run（默认）
#   ./script/start-local.sh --build      # 先编译到 ./bin/santaizi-dashboard 再运行
#   ./script/start-local.sh --reset-config
#   HTTP_PORT=8080 GRPC_PORT=5556 ./script/start-local.sh --reset-config
#
# 数据与配置落在仓库根目录 data/（已 gitignore）。
# 登录：debug + oauth2 mock，访问 /oauth2/login 即以 admin 身份进入后台。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

DATA_DIR="${SANTAIZI_LOCAL_DATA:-$ROOT/data}"
CONFIG_PATH="${SANTAIZI_LOCAL_CONFIG:-$DATA_DIR/dashboard.yaml}"
DB_PATH="${SANTAIZI_LOCAL_DB:-$DATA_DIR/sqlite.db}"
TEMPLATE="$ROOT/script/config.local.yaml"
BIN_PATH="${SANTAIZI_LOCAL_BIN:-$ROOT/bin/santaizi-dashboard}"

HTTP_PORT="${HTTP_PORT:-8000}"
GRPC_PORT="${GRPC_PORT:-5555}"

DO_BUILD=0
RESET_CONFIG=0
PASS_ARGS=()

usage() {
  sed -n '2,12p' "$0" | sed 's/^# \?//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --build)
      DO_BUILD=1
      shift
      ;;
    --reset-config)
      RESET_CONFIG=1
      shift
      ;;
    --)
      shift
      PASS_ARGS+=("$@")
      break
      ;;
    *)
      PASS_ARGS+=("$1")
      shift
      ;;
  esac
done

if ! command -v go >/dev/null 2>&1; then
  echo "error: 需要已安装 Go（go.mod 要求见仓库根目录）" >&2
  exit 1
fi

mkdir -p "$DATA_DIR" "$(dirname "$BIN_PATH")"

seed_config() {
  if [[ ! -f "$TEMPLATE" ]]; then
    echo "error: 缺少模板 $TEMPLATE" >&2
    exit 1
  fi
  # 将模板复制到本地配置，并把 data_dir / 端口写成当前运行参数
  # 配置加载时文件会覆盖 SANTAIZI_* 环境变量，因此端口写进 yaml。
  local tmp
  tmp="$(mktemp)"
  sed \
    -e "s|^  data_dir:.*|  data_dir: \"${DATA_DIR//\\/\\\\}\"|" \
    -e "s|^httpport:.*|httpport: ${HTTP_PORT}|" \
    -e "s|^grpcport:.*|grpcport: ${GRPC_PORT}|" \
    "$TEMPLATE" >"$tmp"
  mv "$tmp" "$CONFIG_PATH"
  echo "已生成配置: $CONFIG_PATH"
}

if [[ "$RESET_CONFIG" -eq 1 ]]; then
  seed_config
elif [[ ! -f "$CONFIG_PATH" ]]; then
  seed_config
else
  # 已有配置时以 yaml 为准展示端口（文件会覆盖 SANTAIZI_*）
  cfg_http="$(awk '/^httpport:/{print $2; exit}' "$CONFIG_PATH" || true)"
  cfg_grpc="$(awk '/^grpcport:/{print $2; exit}' "$CONFIG_PATH" || true)"
  [[ -n "$cfg_http" ]] && HTTP_PORT="$cfg_http"
  [[ -n "$cfg_grpc" ]] && GRPC_PORT="$cfg_grpc"
fi

export CGO_ENABLED="${CGO_ENABLED:-1}"

echo "SANTAIZI local Primary"
echo "  config : $CONFIG_PATH"
echo "  db     : $DB_PATH"
echo "  web    : http://127.0.0.1:${HTTP_PORT}"
echo "  grpc   : 127.0.0.1:${GRPC_PORT}"
echo "  login  : http://127.0.0.1:${HTTP_PORT}/oauth2/login  (mock admin)"
echo

if [[ "$DO_BUILD" -eq 1 ]]; then
  echo "编译 -> $BIN_PATH"
  go build -o "$BIN_PATH" ./cmd/dashboard
  exec "$BIN_PATH" -c "$CONFIG_PATH" -db "$DB_PATH" "${PASS_ARGS[@]}"
fi

exec go run ./cmd/dashboard -c "$CONFIG_PATH" -db "$DB_PATH" "${PASS_ARGS[@]}"
