#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
project_dir=$(cd -- "${script_dir}/.." && pwd)
source_env=${1:-${project_dir}/.env}
target=${2:-${project_dir}/config.yml}

if [[ ! -f "${source_env}" || ! -r "${source_env}" ]]; then
  printf 'Environment file is not readable: %s\n' "${source_env}" >&2
  exit 1
fi

trim() {
  local value=$1
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "${value}"
}

# Parse dotenv as data. Credential files are never sourced or evaluated.
declare -A dotenv=()
while IFS= read -r raw_line || [[ -n "${raw_line}" ]]; do
  line=${raw_line%$'\r'}
  [[ "${line}" =~ ^[[:space:]]*$ ]] && continue
  [[ "${line}" =~ ^[[:space:]]*# ]] && continue

  if [[ "${line}" =~ ^[[:space:]]*(export[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*=(.*)$ ]]; then
    key=${BASH_REMATCH[2]}
    value=$(trim "${BASH_REMATCH[3]}")
    if [[ ${#value} -ge 2 ]]; then
      first=${value:0:1}
      last=${value: -1}
      if [[ ( "${first}" == "'" && "${last}" == "'" ) || ( "${first}" == '"' && "${last}" == '"' ) ]]; then
        value=${value:1:${#value}-2}
      fi
    fi
    dotenv["${key}"]=${value}
  fi
done <"${source_env}"

language=$(printf '%s' "${dotenv[GEOFENCE_LANGUAGE]-en}" | tr '[:upper:]' '[:lower:]')
case "${language}" in
  en|de) ;;
  *)
    printf 'GEOFENCE_LANGUAGE must be en or de, got %q\n' "${language}" >&2
    exit 1
    ;;
esac

validate_integer() {
  local key=$1
  local value=$2
  local minimum=$3
  local maximum=$4
  if [[ ! "${value}" =~ ^[0-9]+$ ]]; then
    printf '%s must be an integer between %s and %s\n' "${key}" "${minimum}" "${maximum}" >&2
    exit 1
  fi
  local leading_zeros=${value%%[!0]*}
  local normalized=${value#"${leading_zeros}"}
  [[ -n "${normalized}" ]] || normalized=0
  if (( ${#normalized} > ${#maximum} )); then
    printf '%s must be an integer between %s and %s\n' "${key}" "${minimum}" "${maximum}" >&2
    exit 1
  fi
  local number=$((10#${normalized}))
  if (( number < minimum || number > maximum )); then
    printf '%s must be an integer between %s and %s\n' "${key}" "${minimum}" "${maximum}" >&2
    exit 1
  fi
  printf '%s' "${number}"
}

validate_number() {
  local key=$1
  local value=$2
  if [[ ! "${value}" =~ ^[+-]?([0-9]+([.][0-9]*)?|[.][0-9]+)([eE][+-]?[0-9]+)?$ ]]; then
    printf '%s must be a finite number\n' "${key}" >&2
    exit 1
  fi
  if ! awk -v value="${value}" 'BEGIN {
    number = value + 0
    maximum = 1.7976931348623157e308
    if (number > maximum || number < -maximum) exit 1

    # Reject non-zero values that underflow the runtime float parser to zero.
    mantissa = value
    sub(/[eE].*$/, "", mantissa)
    gsub(/[+.-]/, "", mantissa)
    gsub(/0/, "", mantissa)
    if (number == 0 && mantissa != "") exit 1
  }'; then
    printf '%s must be a finite number\n' "${key}" >&2
    exit 1
  fi
}

midcap_max=$(validate_integer "GEOFENCE_MIDCAP_MAX_PLAYERS" "${dotenv[GEOFENCE_MIDCAP_MAX_PLAYERS]-60}" 0 100)
advanced_max=$(validate_integer "GEOFENCE_MIDCAP_ADVANCED_MAX_PLAYERS" "${dotenv[GEOFENCE_MIDCAP_ADVANCED_MAX_PLAYERS]-99}" 0 100)
lastcap_max=$(validate_integer "GEOFENCE_LASTCAP_MAX_PLAYERS" "${dotenv[GEOFENCE_LASTCAP_MAX_PLAYERS]-90}" 0 100)
validate_integer "GEOFENCE_PUNISH_AFTER_SECONDS" "${dotenv[GEOFENCE_PUNISH_AFTER_SECONDS]-10}" 1 3600 >/dev/null

sector_size=${dotenv[GEOFENCE_SECTOR_SIZE]-20000}
validate_number "GEOFENCE_SECTOR_SIZE" "${sector_size}"
if ! awk -v value="${sector_size}" 'BEGIN { exit !(value > 0) }'; then
  printf 'GEOFENCE_SECTOR_SIZE must be a number greater than zero\n' >&2
  exit 1
fi
validate_number "GEOFENCE_CENTER_OFFSET_X" "${dotenv[GEOFENCE_CENTER_OFFSET_X]-0}"
validate_number "GEOFENCE_CENTER_OFFSET_Y" "${dotenv[GEOFENCE_CENTER_OFFSET_Y]-0}"

for message_key in GEOFENCE_WARNING_MESSAGE GEOFENCE_PUNISH_MESSAGE; do
  if [[ -v "dotenv[${message_key}]" && -z "${dotenv[${message_key}]}" ]]; then
    printf '%s must not be empty when set\n' "${message_key}" >&2
    exit 1
  fi
done

profile=$(printf '%s' "${dotenv[GEOFENCE_PROFILE]-midcap}" | tr '[:upper:]' '[:lower:]')
case "${profile}" in
  midcap)
    max_players=${midcap_max}
    ;;
  midcap-advanced)
    max_players=${advanced_max}
    ;;
  lastcap)
    max_players=${lastcap_max}
    ;;
  *)
    printf 'GEOFENCE_PROFILE must be midcap, midcap-advanced, or lastcap, got %q\n' "${profile}" >&2
    exit 1
    ;;
esac

require_server_value() {
  local server=$1
  local key=$2
  local value=${dotenv[${key}]-}
  if [[ -z "${value}" ]]; then
    printf 'Incomplete server %s: %s is missing\n' "${server}" "${key}" >&2
    exit 1
  fi
  if [[ "${value}" == *$'\n'* || "${value}" == *$'\r'* ]]; then
    printf '%s must not contain line breaks\n' "${key}" >&2
    exit 1
  fi
  printf '%s' "${value}"
}

# Validate every runtime server tuple even though credentials are never copied
# into YAML. Numeric suffixes must remain contiguous.
declare -A discovered_servers=([1]=1)
for key in "${!dotenv[@]}"; do
  if [[ "${key}" =~ ^HLL_(HOST|PORT|PASSWORD)_([0-9]+)$ ]]; then
    server=${BASH_REMATCH[2]}
    if (( server < 2 )); then
      printf 'Use unsuffixed HLL_HOST/HLL_PORT/HLL_PASSWORD for server 1\n' >&2
      exit 1
    fi
    discovered_servers["${server}"]=1
  fi
done
mapfile -t servers < <(printf '%s\n' "${!discovered_servers[@]}" | sort -n)

expected_server=1
for server in "${servers[@]}"; do
  if (( server != expected_server )); then
    printf 'Server numbering must be contiguous; expected server %s, found server %s\n' "${expected_server}" "${server}" >&2
    exit 1
  fi
  expected_server=$((expected_server + 1))
  suffix=""
  if (( server > 1 )); then
    suffix="_${server}"
  fi
  require_server_value "${server}" "HLL_HOST${suffix}" >/dev/null
  port=$(require_server_value "${server}" "HLL_PORT${suffix}")
  require_server_value "${server}" "HLL_PASSWORD${suffix}" >/dev/null
  if [[ ! "${port}" =~ ^[0-9]+$ || ${port} -lt 1 || ${port} -gt 65535 ]]; then
    printf 'HLL_PORT%s must be an integer between 1 and 65535\n' "${suffix}" >&2
    exit 1
  fi
done

target_dir=$(dirname -- "${target}")
mkdir -p -- "${target_dir}"
tmp=$(mktemp "${target}.tmp.XXXXXX")
{
  printf '%s\n' '# HLL: Vietnam geofence policy. Server credentials are loaded from environment variables.'
  printf '%s\n' 'Servers:'
  printf '%s\n' '  - AxisFence: []'
  printf '%s\n' '    AlliesFence: []'
} >"${tmp}"
chmod 0600 "${tmp}"

(
  cd -- "${project_dir}"
  go run ./cmd/applymap --map vietnam --variant "${profile}" --max-players "${max_players}" "${tmp}"
)
mv -f -- "${tmp}" "${target}"

printf 'Generated credential-free Vietnam geofence policy %s (profile=%s, max_players=%s, language=%s, runtime_servers=%s)\n' \
  "${target}" "${profile}" "${max_players}" "${language}" "${#servers[@]}"
