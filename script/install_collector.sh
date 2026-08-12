#!/bin/sh

#========================================================
#   Santaizi 从端（Collector）一键安装脚本
#   非交互；供 curl | bash -s -- 使用
#========================================================

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

GHCR_IMAGE="ghcr.io/hi2shark/santaizi-dashboard"
PRIMARY_ENDPOINT=""
REGISTRATION_TOKEN=""
GRPC_PORT="5556"
PRIMARY_TLS="false"
PRIMARY_INSECURE_TLS="false"
WORK_DIR="/opt/santaizi/collector"

err() {
    printf "${red}%s${plain}\n" "$*" >&2
}

info() {
    printf "${yellow}%s${plain}\n" "$*"
}

success() {
    printf "${green}%s${plain}\n" "$*"
}

sudo() {
    myEUID=$(id -ru)
    if [ "$myEUID" -ne 0 ]; then
        if command -v sudo > /dev/null 2>&1; then
            command sudo "$@"
        elif command -v doas > /dev/null 2>&1; then
            command doas "$@"
        else
            err "ERROR: 当前非 root，且未安装 sudo/doas，无法继续。"
            exit 1
        fi
    else
        "$@"
    fi
}

yaml_escape() {
    printf "%s" "$1" | sed "s/'/''/g"
}

usage() {
    cat <<EOF
Usage: $0 --primary-endpoint host:port --token <registration_token> [options]

Options:
  --primary-endpoint   Primary gRPC 地址（必填，如 primary.example.com:5555）
  --token              从端注册 Token（必填）
  --grpc-port          本机监听 gRPC 端口（默认 5556）
  --primary-tls        连接 Primary 时启用 TLS
  --primary-insecure-tls  跳过 Primary 证书校验（仅受控测试）
  --dir                安装目录（默认 /opt/santaizi/collector）
  -h, --help           显示帮助
EOF
}

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --primary-endpoint)
                PRIMARY_ENDPOINT=$2
                shift 2
                ;;
            --token)
                REGISTRATION_TOKEN=$2
                shift 2
                ;;
            --grpc-port)
                GRPC_PORT=$2
                shift 2
                ;;
            --primary-tls)
                PRIMARY_TLS="true"
                shift
                ;;
            --primary-insecure-tls)
                PRIMARY_INSECURE_TLS="true"
                shift
                ;;
            --dir)
                WORK_DIR=$2
                shift 2
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                err "未知参数: $1"
                usage
                exit 1
                ;;
        esac
    done
}

validate_port() {
    case "$1" in
        ''|*[!0-9]*) return 1 ;;
    esac
    [ "$1" -ge 1 ] 2>/dev/null && [ "$1" -le 65535 ] 2>/dev/null
}

detect_linux_distro() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        printf "%s\n" "$ID"
    else
        printf "unknown\n"
    fi
}

install_docker() {
    info "正在安装 Docker..."
    distro=$(detect_linux_distro)
    case "$distro" in
        debian|ubuntu|linuxmint|raspbian)
            sudo apt-get update
            sudo apt-get install -y ca-certificates curl gnupg
            sudo install -m 0755 -d /etc/apt/keyrings
            curl -fsSL https://download.docker.com/linux/debian/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg || \
            curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
            . /etc/os-release
            printf "deb [arch=%s signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/%s %s stable\n" "$(dpkg --print-architecture)" "$ID" "$VERSION_CODENAME" | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
            sudo apt-get update
            sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            ;;
        centos|rhel|almalinux|rocky)
            sudo yum install -y yum-utils
            sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
            sudo yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            sudo systemctl start docker
            sudo systemctl enable docker
            ;;
        fedora)
            sudo dnf -y install dnf-plugins-core
            sudo dnf config-manager --add-repo https://download.docker.com/linux/fedora/docker-ce.repo
            sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            sudo systemctl start docker
            sudo systemctl enable docker
            ;;
        alpine)
            sudo apk add --no-cache docker docker-cli-compose
            sudo rc-update add docker default
            sudo service docker start
            ;;
        *)
            err "暂不支持的 Linux 发行版: $distro，请手动安装 Docker。"
            exit 1
            ;;
    esac
    success "Docker 安装完成。"
}

install_docker_compose_plugin() {
    info "正在安装 Docker Compose 插件..."
    sudo mkdir -p /usr/local/lib/docker/cli-plugins
    compose_version=$(curl -fsSL -m 10 https://api.github.com/repos/docker/compose/releases/latest | grep '"tag_name":' | head -n 1 | sed 's/.*"tag_name": "\(.*\)",.*/\1/')
    if [ -z "$compose_version" ]; then
        err "获取 Docker Compose 版本失败，请检查网络。"
        exit 1
    fi
    arch=$(uname -m)
    case "$arch" in
        x86_64) arch="x86_64" ;;
        aarch64|arm64) arch="aarch64" ;;
        armv7l) arch="armv7" ;;
        *) err "不支持的架构: $arch" ; exit 1 ;;
    esac
    sudo curl -fsSL -o /usr/local/lib/docker/cli-plugins/docker-compose "https://github.com/docker/compose/releases/download/${compose_version}/docker-compose-linux-${arch}"
    sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
    success "Docker Compose 插件安装完成。"
}

check_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        info "未检测到 Docker，尝试自动安装..."
        install_docker
    fi

    if ! docker compose version >/dev/null 2>&1 && ! command -v docker-compose >/dev/null 2>&1; then
        info "未检测到 Docker Compose，尝试自动安装插件..."
        install_docker_compose_plugin
    fi

    if ! command -v docker >/dev/null 2>&1; then
        err "Docker 不可用，请先手动安装。"
        exit 1
    fi
    if ! docker compose version >/dev/null 2>&1 && ! command -v docker-compose >/dev/null 2>&1; then
        err "Docker Compose 不可用，请先手动安装。"
        exit 1
    fi
}

run_compose() {
    if docker compose version >/dev/null 2>&1; then
        sudo docker compose "$@"
    else
        sudo docker-compose "$@"
    fi
}

write_compose() {
    mkdir -p "$1"
    cat > "$1/docker-compose.yml" <<EOF
services:
  santaizi-collector:
    image: ${GHCR_IMAGE}:latest
    container_name: santaizi-collector
    restart: unless-stopped
    ports:
      - "${GRPC_PORT}:${GRPC_PORT}"
    volumes:
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
      - ./data:/var/lib/santaizi-dashboard
      - ./config/dashboard.yaml:/etc/santaizi/dashboard.yaml:ro
    environment:
      - TZ=Asia/Shanghai
EOF
}

write_config() {
    mkdir -p "$1/data" "$1/config"
    endpoint=$(yaml_escape "$PRIMARY_ENDPOINT")
    token=$(yaml_escape "$REGISTRATION_TOKEN")
    cat > "$1/config/dashboard.yaml" <<EOF
mode: collector
debug: false
language: zh-CN
grpcport: ${GRPC_PORT}
telemetry:
  data_dir: /var/lib/santaizi-dashboard
collector:
  primary_endpoint: '${endpoint}'
  primary_tls: ${PRIMARY_TLS}
  primary_insecure_tls: ${PRIMARY_INSECURE_TLS}
  registration_token: '${token}'
  database_path: /var/lib/santaizi-dashboard/collector.db
EOF
    chmod 600 "$1/config/dashboard.yaml"
}

main() {
    parse_args "$@"

    info "欢迎使用 Santaizi 从端一键安装脚本"

    if [ "$(uname -s)" != "Linux" ]; then
        err "本脚本目前仅支持 Linux 系统。"
        exit 1
    fi

    if [ -z "$PRIMARY_ENDPOINT" ] || [ -z "$REGISTRATION_TOKEN" ]; then
        err "必须提供 --primary-endpoint 与 --token。"
        usage
        exit 1
    fi

    if ! validate_port "$GRPC_PORT"; then
        err "grpc-port 必须是 1-65535 之间的数字。"
        exit 1
    fi

    check_docker

    if ! mkdir -p "$WORK_DIR"; then
        err "创建工作目录失败: $WORK_DIR"
        exit 1
    fi
    cd "$WORK_DIR" || {
        err "无法进入工作目录: $WORK_DIR"
        exit 1
    }

    info "正在生成从端配置..."
    write_compose "$WORK_DIR"
    write_config "$WORK_DIR"

    info "正在拉取镜像并启动从端..."
    if ! run_compose up -d; then
        err "启动从端失败，请检查 Docker 与网络。"
        exit 1
    fi

    success "Santaizi 从端安装完成。"
    success "工作目录: ${WORK_DIR}"
    success "gRPC 端口: ${GRPC_PORT}"
    success "配置文件: ${WORK_DIR}/config/dashboard.yaml"
}

main "$@"
