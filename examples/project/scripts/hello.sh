#!/usr/bin/env bash
# Bash "app" demo — runs inside ansible-ui's live PTY (colour + TTY preserved).
# Any run extra-vars are exported as environment variables (e.g. GREETING).
set -euo pipefail

bold=$'\e[1m'; green=$'\e[32m'; cyan=$'\e[36m'; reset=$'\e[0m'

echo "${bold}${cyan}=== ansible-ui · bash app demo ===${reset}"
echo "host    : $(hostname)"
echo "date    : $(date)"
echo "user    : $(whoami)"
echo "greeting: ${GREETING:-<none>}"

for i in 1 2 3; do
  echo "${green}▸ step $i ok${reset}"
  sleep 0.3
done

echo "${bold}done.${reset}"
