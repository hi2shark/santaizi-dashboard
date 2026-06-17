#!/bin/sh

#========================================================
# Install script wrapper
# Reads the actual install script URL from NEZHA_SCRIPT_URL
# Defaults to this repo's agent-only install script
#========================================================

shell_url="${NEZHA_SCRIPT_URL:-https://raw.githubusercontent.com/hi2shark/nezha-next/master/script/install_agent_en.sh}"

if command -v wget >/dev/null 2>&1; then
    wget -O nezha_v0.sh "$shell_url"
elif command -v curl >/dev/null 2>&1; then
    curl -fsSL -o nezha_v0.sh "$shell_url"
else
    echo "Error: wget or curl not found, please install one of them first"
    exit 1
fi

chmod +x nezha_v0.sh

# run new script with original parameters
exec ./nezha_v0.sh "$@"
