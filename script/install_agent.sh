#!/bin/sh

#========================================================
#   Santaizi Agent 一键安装脚本
#   默认从 hi2shark/santaizi-agent 下载，可通过 SANTAIZI_AGENT_REPO 覆盖
#========================================================

NZ_BASE_PATH="/opt/santaizi"
NZ_AGENT_PATH="${NZ_BASE_PATH}/agent"

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

SANTAIZI_AGENT_REPO="${SANTAIZI_AGENT_REPO:-hi2shark/santaizi-agent}"

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
        else
            err "ERROR: 当前非 root 且未安装 sudo，无法继续。"
            exit 1
        fi
    else
        "$@"
    fi
}

deps_check() {
    local deps="curl unzip"
    local missing=""
    for dep in $deps; do
        if ! command -v "$dep" >/dev/null 2>&1; then
            missing="${missing} ${dep}"
        fi
    done
    if [ -n "$missing" ]; then
        err "缺少依赖:${missing}，请先安装后再试。"
        exit 1
    fi
}

detect_os() {
    system=$(uname)
    case "$system" in
        *Linux*) echo "linux" ;;
        *Darwin*) echo "darwin" ;;
        *FreeBSD*) echo "freebsd" ;;
        *) echo "unknown" ;;
    esac
}

detect_arch() {
    mach=$(uname -m)
    case "$mach" in
        amd64|x86_64) echo "amd64" ;;
        i386|i686) echo "386" ;;
        aarch64|arm64) echo "arm64" ;;
        *arm*) echo "arm" ;;
        s390x) echo "s390x" ;;
        riscv64) echo "riscv64" ;;
        mips) echo "mips" ;;
        mipsel|mipsle) echo "mipsle" ;;
        *) echo "unknown" ;;
    esac
}

get_latest_version() {
    version=$(curl -fsSL -m 10 "https://api.github.com/repos/${SANTAIZI_AGENT_REPO}/releases/latest" | grep '"tag_name":' | head -n 1 | sed 's/.*"tag_name": "\(.*\)",.*/\1/')
    if [ -z "$version" ]; then
        err "获取 Agent 版本失败，请检查网络是否能访问 https://api.github.com/repos/${SANTAIZI_AGENT_REPO}/releases/latest"
        exit 1
    fi
    echo "$version"
}

install_agent() {
    deps_check

    os=$(detect_os)
    arch=$(detect_arch)
    if [ "$os" = "unknown" ] || [ "$arch" = "unknown" ]; then
        err "不支持的操作系统或架构: $(uname) / $(uname -m)"
        exit 1
    fi

    info "正在获取 Agent 最新版本..."
    version=$(get_latest_version)
    success "最新版本: ${version}"

    tmpfile="/tmp/santaizi-agent_${os}_${arch}.zip"
    url="https://github.com/${SANTAIZI_AGENT_REPO}/releases/download/${version}/santaizi-agent_${os}_${arch}.zip"

    info "正在下载 ${url} ..."
    if ! curl -fsSL -m 60 -o "$tmpfile" "$url"; then
        err "下载 Agent 失败，请检查网络连接。"
        exit 1
    fi

    info "正在安装到 ${NZ_AGENT_PATH} ..."
    sudo mkdir -p "$NZ_AGENT_PATH"
    sudo unzip -qo "$tmpfile" -d "$NZ_AGENT_PATH" || {
        err "解压 Agent 失败。"
        rm -f "$tmpfile"
        exit 1
    }
    rm -f "$tmpfile"
    sudo chmod +x "${NZ_AGENT_PATH}/santaizi-agent"
}

configure_agent() {
    if [ $# -lt 3 ]; then
        err "参数不足，用法: $0 install_agent <服务器地址> <端口> <密钥> [额外参数]"
        exit 1
    fi

    host=$1
    port=$2
    secret=$3
    shift 3

    info "正在配置并启动 Agent 服务..."
    sudo "${NZ_AGENT_PATH}/santaizi-agent" service uninstall >/dev/null 2>&1 || true
    if ! sudo "${NZ_AGENT_PATH}/santaizi-agent" service install -s "${host}:${port}" -p "${secret}" "$@"; then
        err "安装 Agent 服务失败。"
        exit 1
    fi
    success "Agent 安装完成。"
}

# 主入口
if [ "$1" = "install_agent" ]; then
    shift
fi

if [ $# -lt 3 ]; then
    echo "用法: $0 [install_agent] <服务器地址> <端口> <密钥> [额外参数]"
    echo "示例: $0 install_agent 1.2.3.4 5555 abcdef --tls --disable-auto-update"
    exit 1
fi

install_agent
configure_agent "$@"
