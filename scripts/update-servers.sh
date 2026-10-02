#!/usr/bin/env bash
# Update the WarmDesk server on each host by running ./get_warmdesk from the
# warmdesk user's home directory. get_warmdesk needs root (systemctl), so it
# runs through sudo; ssh -t gives sudo a terminal for its password prompt.
#
# Usage: update-servers.sh [--force|-f] [host ...]
#   --force, -f   passed on to get_warmdesk (reinstall even if up to date)
#   host ...      hosts to update (default: toolbox warmtest stepper)
set -uo pipefail

readonly WD_USER="warmdesk"
DEFAULT_HOSTS=(toolbox warmtest stepper)

ARGS=()
HOSTS=()
for arg in "${@}"
do
    case "${arg}" in
        -f|--force) ARGS+=(--force) ;;
        -h|--help)
            sed -n '2,8p' "$0" | sed 's/^# \{0,1\}//'
            exit 0 ;;
        -*) echo "Unknown option: ${arg}" >&2; exit 1 ;;
        *)  HOSTS+=("${arg}") ;;
    esac
done
[[ ${#HOSTS[@]} -eq 0 ]] && HOSTS=("${DEFAULT_HOSTS[@]}")

FAILED=()
for host in "${HOSTS[@]}"
do
    echo "==> ${host}"
    if ! ssh -t "${host}" "sudo sh -c 'cd ~${WD_USER} && ./get_warmdesk ${ARGS[*]:-}'"
    then
        FAILED+=("${host}")
    fi
    echo
done

if [[ ${#FAILED[@]} -gt 0 ]]
then
    echo "Failed: ${FAILED[*]}" >&2
    exit 1
fi
echo "All hosts updated: ${HOSTS[*]}"
