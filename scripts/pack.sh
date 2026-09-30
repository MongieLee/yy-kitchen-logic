#!/usr/bin/env bash
# 在本机编译 Linux 二进制，并把 config.prod.yaml 打进 tar.gz。
# 服务器不需要 Go：解压后执行 install.sh 即可。
#
#   ./scripts/pack.sh              # 默认 linux/amd64
#   ./scripts/pack.sh --arch arm64
#
# 产物：dist/yy-kitchen-logic-linux-<arch>.tar.gz
#
# 配置约定：
#   config/config.yaml      本机开发
#   config/config.prod.yaml 线上（打包时写入发布包）

set -euo pipefail

APP_NAME="yy-kitchen-logic"
ARCH="amd64"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
PROD_CONFIG="${ROOT_DIR}/config/config.prod.yaml"

log() { printf '\033[0;32m[pack]\033[0m %s\n' "$*"; }
die() { printf '\033[0;31m[error]\033[0m %s\n' "$*" >&2; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --arch) ARCH="${2:-}"; shift 2 ;;
    -h|--help)
      sed -n '2,16p' "$0" | sed 's/^# \?//'
      exit 0
      ;;
    *) die "未知参数: $1" ;;
  esac
done

[[ "$ARCH" == "amd64" || "$ARCH" == "arm64" ]] || die "--arch 只支持 amd64 或 arm64"
command -v go >/dev/null 2>&1 || die "本机没有 go，请先安装 Go"
command -v tar >/dev/null 2>&1 || die "缺少 tar"
[[ -f "$PROD_CONFIG" ]] || die "缺少线上配置 ${PROD_CONFIG}，请先按 config.example.yaml 写一份"

PKG_NAME="${APP_NAME}-linux-${ARCH}"
STAGE="${DIST_DIR}/${PKG_NAME}"
TARBALL="${DIST_DIR}/${PKG_NAME}.tar.gz"
BIN_OUT="${STAGE}/${APP_NAME}"

rm -rf "$STAGE"
mkdir -p "$STAGE/config" "$STAGE/uploads"

log "交叉编译 ${APP_NAME} (linux/${ARCH})"
(
  cd "$ROOT_DIR"
  CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" \
    go build -trimpath -ldflags="-s -w" -o "$BIN_OUT" .
)
chmod +x "$BIN_OUT"

# 发布包里的文件名必须是 config.yaml，程序启动时只读这个名字
cp "$PROD_CONFIG" "$STAGE/config/config.yaml"
cp "$ROOT_DIR/scripts/install.sh" "$STAGE/install.sh"
chmod +x "$STAGE/install.sh"
touch "$STAGE/uploads/.gitkeep"

cat > "$STAGE/README.txt" <<EOF
yy-kitchen-logic 发布包 (${PKG_NAME})
====================================

1. 把这个 tar.gz 传到 Linux 服务器
2. 解压：
     tar -xzf ${PKG_NAME}.tar.gz
     cd ${PKG_NAME}
3. 安装并启动（需要 root）：
     sudo ./install.sh

包内 config/config.yaml 来自开发机的 config.prod.yaml。
install.sh 会把它装到 /opt/yy-kitchen-logic/config/config.yaml 并覆盖旧文件。
EOF

log "打包 ${TARBALL}"
mkdir -p "$DIST_DIR"
tar -C "$DIST_DIR" -czf "$TARBALL" "$PKG_NAME"
rm -rf "$STAGE"

log "完成"
log "文件: ${TARBALL}"
log "大小: $(du -h "$TARBALL" | awk '{print $1}')"
log "配置: 已打入 config.prod.yaml"
log "传到服务器后："
log "  tar -xzf ${PKG_NAME}.tar.gz && cd ${PKG_NAME} && sudo ./install.sh"
