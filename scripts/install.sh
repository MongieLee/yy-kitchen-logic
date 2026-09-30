#!/usr/bin/env bash
# 在 Linux 服务器上安装 / 更新 yy-kitchen-logic。
# 把 pack.sh 打好的 tar.gz 解压后，在解压目录里执行：
#
#   sudo ./install.sh
#
# 可选：
#   sudo ./install.sh --dir /opt/yy-kitchen-logic
#   sudo ./install.sh --skip-mysql-check
#
# 配置来自发布包里的 config/config.yaml（打包时由 config.prod.yaml 复制而来），
# 每次安装都会覆盖服务器上的同名文件。

set -euo pipefail

APP_NAME="yy-kitchen-logic"
APP_DIR="/opt/yy-kitchen-logic"
APP_USER="yykitchen"
SKIP_MYSQL_CHECK=0
PKG_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

log()  { printf '\033[0;32m[install]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[warn]\033[0m %s\n' "$*"; }
die()  { printf '\033[0;31m[error]\033[0m %s\n' "$*" >&2; exit 1; }

yaml_value() {
  # 取某个一级段落下的标量，例如: yaml_value database host
  local section="$1" key="$2" file="$3"
  awk -v section="$section" -v key="$key" '
    $0 ~ "^" section ":" { in_section=1; next }
    in_section && /^[^[:space:]#]/ { in_section=0 }
    in_section && $1 == key ":" {
      $1=""
      sub(/^[[:space:]]+/, "")
      gsub(/^"/, ""); gsub(/"$/, "")
      gsub(/^'\''/, ""); gsub(/'\''$/, "")
      print
      exit
    }
  ' "$file"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dir)               APP_DIR="${2:-}"; shift 2 ;;
    --user)              APP_USER="${2:-}"; shift 2 ;;
    --skip-mysql-check)  SKIP_MYSQL_CHECK=1; shift ;;
    -h|--help)
      sed -n '2,16p' "$0" | sed 's/^# \?//'
      exit 0
      ;;
    *) die "未知参数: $1" ;;
  esac
done

[[ "$(id -u)" -eq 0 ]] || die "请用 root 执行: sudo $0"
[[ "$(uname -s)" == "Linux" ]] || die "只能在 Linux 上安装"
[[ -x "${PKG_DIR}/${APP_NAME}" ]] || die "当前目录没有可执行文件 ${APP_NAME}，请先解压发布包"
[[ -f "${PKG_DIR}/config/config.yaml" ]] || die "发布包缺少 config/config.yaml（应由 pack.sh 从 config.prod.yaml 打入）"
command -v systemctl >/dev/null 2>&1 || die "需要 systemd"

APP_PORT="$(yaml_value app port "${PKG_DIR}/config/config.yaml")"
APP_PORT="${APP_PORT:-8090}"

id "$APP_USER" >/dev/null 2>&1 || useradd --system --home "$APP_DIR" --shell /usr/sbin/nologin "$APP_USER"

log "安装目录 ${APP_DIR}"
mkdir -p "$APP_DIR/config" "$APP_DIR/uploads"

install -m 755 "${PKG_DIR}/${APP_NAME}" "${APP_DIR}/${APP_NAME}"
install -m 640 "${PKG_DIR}/config/config.yaml" "${APP_DIR}/config/config.yaml"
chown -R "${APP_USER}:${APP_USER}" "$APP_DIR"
log "已写入配置 ${APP_DIR}/config/config.yaml"

if [[ "$SKIP_MYSQL_CHECK" != "1" ]] && command -v mysql >/dev/null 2>&1; then
  db_host="$(yaml_value database host "${APP_DIR}/config/config.yaml")"
  db_port="$(yaml_value database port "${APP_DIR}/config/config.yaml")"
  db_user="$(yaml_value database user "${APP_DIR}/config/config.yaml")"
  db_name="$(yaml_value database name "${APP_DIR}/config/config.yaml")"
  db_password="$(yaml_value database password "${APP_DIR}/config/config.yaml")"
  db_host="${db_host:-127.0.0.1}"
  db_port="${db_port:-3306}"
  if mysql --protocol=tcp -h "$db_host" -P "$db_port" -u "$db_user" -p"$db_password" -e "SELECT 1" >/dev/null 2>&1; then
    log "MySQL 连接正常 (${db_user}@${db_host}:${db_port})"
    mysql --protocol=tcp -h "$db_host" -P "$db_port" -u "$db_user" -p"$db_password" \
      -e "CREATE DATABASE IF NOT EXISTS \`${db_name}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" \
      >/dev/null 2>&1 || warn "当前账号可能没有建库权限，启动时应用会再尝试一次"
  else
    warn "现在连不上 MySQL。请确认服务已启动、账号密码正确。应用启动时仍会尝试建库建表。"
  fi
fi

cat > "/etc/systemd/system/${APP_NAME}.service" <<EOF
[Unit]
Description=YY Kitchen Logic API
After=network-online.target mysql.service mysqld.service mariadb.service
Wants=network-online.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_USER}
WorkingDirectory=${APP_DIR}
ExecStart=${APP_DIR}/${APP_NAME}
Restart=on-failure
RestartSec=3
LimitNOFILE=65535
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "${APP_NAME}.service"
systemctl restart "${APP_NAME}.service"

log "等待服务就绪（首次建表/seed 可能要几秒）"
ok=0
for i in $(seq 1 20); do
  if curl -fsS "http://127.0.0.1:${APP_PORT}/health" >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 1
done
if [[ "$ok" == "1" ]]; then
  log "健康检查通过  http://127.0.0.1:${APP_PORT}/health"
else
  warn "暂时没打通 /health，最近日志："
  journalctl -u "${APP_NAME}" -n 40 --no-pager || true
  echo
  die "服务已安装但启动异常。改开发机 config.prod.yaml 后重新打包安装，或直接改 ${APP_DIR}/config/config.yaml 再 systemctl restart ${APP_NAME}"
fi

log "安装完成"
log "  接口  http://服务器IP:${APP_PORT}"
log "  文档  http://服务器IP:${APP_PORT}/docs"
log "  配置  ${APP_DIR}/config/config.yaml"
log "  日志  journalctl -u ${APP_NAME} -f"
log "  重启  systemctl restart ${APP_NAME}"
warn "默认管理员 admin/admin，App 用户 13232251037/123456，上线后请改密。"
