#!/usr/bin/env bash
set -euo pipefail

project_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
script_path="${project_dir}/scripts/generate-config.sh"
test_dir=$(mktemp -d)
trap 'rm -rf -- "${test_dir}"' EXIT

english_env="${test_dir}/english.env"
english_config="${test_dir}/english-config.yml"
printf '%s\n' \
  'GEOFENCE_LANGUAGE=en' \
  'GEOFENCE_PROFILE=midcap' \
  'GEOFENCE_MIDCAP_MAX_PLAYERS=42' \
  'GEOFENCE_MIDCAP_ADVANCED_MAX_PLAYERS=88' \
  'GEOFENCE_LASTCAP_MAX_PLAYERS=77' \
  'GEOFENCE_PUNISH_AFTER_SECONDS=17' \
  'GEOFENCE_SECTOR_SIZE=19840.5' \
  'GEOFENCE_CENTER_OFFSET_X=150.25' \
  'GEOFENCE_CENTER_OFFSET_Y=-110.5' \
  'GEOFENCE_WARNING_MESSAGE=Custom warning text' \
  'GEOFENCE_PUNISH_MESSAGE=Custom punishment text' \
  'HLL_HOST=192.0.2.10' \
  'HLL_PORT=7779' \
  'HLL_PASSWORD=first-secret' \
  'HLL_HOST_2=192.0.2.11' \
  'HLL_PORT_2=7789' \
  'HLL_PASSWORD_2=second-secret' \
  >"${english_env}"

"${script_path}" "${english_env}" "${english_config}"

if [[ ! -f "${english_config}" ]]; then
  printf 'generator did not create config.yml\n' >&2
  exit 1
fi
for prefix in wdeva wdevb wdevc wdevd wdeve wdevf; do
  grep -Fq "${prefix}_warfare_day" "${english_config}"
done
grep -Fq 'player_count: 43' "${english_config}"

for forbidden in \
  'Host:' 'Port:' 'Password:' 'PunishAfterSeconds:' 'Grid:' 'Messages:' \
  '192.0.2.10' '192.0.2.11' 'first-secret' 'second-secret' \
  'Custom warning text' 'Custom punishment text'; do
  if grep -Fq "${forbidden}" "${english_config}"; then
    printf 'credential-free config contains %s\n' "${forbidden}" >&2
    exit 1
  fi
done
excluded_mode=$'see\x64ing'
if grep -Eiq "${excluded_mode}|Carentan|St[ .-]*Marie|Omaha|Utah|Foy|Kursk|Stalingrad|Remagen" "${english_config}"; then
  printf 'config contains excluded mode or Classic-HLL data\n' >&2
  exit 1
fi

mode=$(stat -c '%a' "${english_config}")
if [[ "${mode}" != "600" ]]; then
  printf 'config mode is %s, expected 600\n' "${mode}" >&2
  exit 1
fi

env \
  HLL_HOST=192.0.2.10 HLL_PORT=7779 HLL_PASSWORD=first-secret \
  HLL_HOST_2=192.0.2.11 HLL_PORT_2=7789 HLL_PASSWORD_2=second-secret \
  GEOFENCE_LANGUAGE=en GEOFENCE_PUNISH_AFTER_SECONDS=17 \
  GEOFENCE_SECTOR_SIZE=19840.5 GEOFENCE_CENTER_OFFSET_X=150.25 GEOFENCE_CENTER_OFFSET_Y=-110.5 \
  GEOFENCE_WARNING_MESSAGE='Custom warning text' GEOFENCE_PUNISH_MESSAGE='Custom punishment text' \
  go run "${project_dir}/cmd/configcheck" "${english_config}" >/dev/null

german_env="${test_dir}/german.env"
german_config="${test_dir}/german-config.yml"
printf '%s\n' \
  'GEOFENCE_LANGUAGE=de' \
  'GEOFENCE_PROFILE=lastcap' \
  'GEOFENCE_MIDCAP_MAX_PLAYERS=60' \
  'GEOFENCE_MIDCAP_ADVANCED_MAX_PLAYERS=99' \
  'GEOFENCE_LASTCAP_MAX_PLAYERS=77' \
  'HLL_HOST=192.0.2.10' \
  'HLL_PORT=7779' \
  'HLL_PASSWORD=german-secret' \
  >"${german_env}"
"${script_path}" "${german_env}" "${german_config}" >/dev/null
grep -Fq 'player_count: 78' "${german_config}"
if grep -Fq 'german-secret' "${german_config}"; then
  printf 'German config contains its RCON password\n' >&2
  exit 1
fi

assert_rejected() {
  local name=$1
  local expected=$2
  shift 2
  local fixture="${test_dir}/${name}.env"
  printf '%s\n' "$@" >"${fixture}"
  if "${script_path}" "${fixture}" "${test_dir}/${name}.yml" >"${test_dir}/${name}.log" 2>&1; then
    printf 'generator accepted invalid fixture %s\n' "${name}" >&2
    exit 1
  fi
  if ! grep -Fq "${expected}" "${test_dir}/${name}.log"; then
    printf 'generator error for %s did not contain %s\n' "${name}" "${expected}" >&2
    exit 1
  fi
}

assert_rejected invalid-language 'GEOFENCE_LANGUAGE must be en or de' \
  'GEOFENCE_LANGUAGE=fr' 'GEOFENCE_PROFILE=midcap' \
  'HLL_HOST=192.0.2.10' 'HLL_PORT=7779' 'HLL_PASSWORD=secret'

assert_rejected invalid-profile 'GEOFENCE_PROFILE must be midcap, midcap-advanced, or lastcap' \
  'GEOFENCE_LANGUAGE=en' 'GEOFENCE_PROFILE=full-map' \
  'HLL_HOST=192.0.2.10' 'HLL_PORT=7779' 'HLL_PASSWORD=secret'

assert_rejected invalid-midcap-limit 'GEOFENCE_MIDCAP_MAX_PLAYERS must be an integer between 0 and 100' \
  'GEOFENCE_LANGUAGE=en' 'GEOFENCE_PROFILE=midcap' 'GEOFENCE_MIDCAP_MAX_PLAYERS=101' \
  'HLL_HOST=192.0.2.10' 'HLL_PORT=7779' 'HLL_PASSWORD=secret'

assert_rejected overflowing-midcap-limit 'GEOFENCE_MIDCAP_MAX_PLAYERS must be an integer between 0 and 100' \
  'GEOFENCE_LANGUAGE=en' 'GEOFENCE_PROFILE=midcap' \
  'GEOFENCE_MIDCAP_MAX_PLAYERS=18446744073709551616000000000000000042' \
  'HLL_HOST=192.0.2.10' 'HLL_PORT=7779' 'HLL_PASSWORD=secret'

assert_rejected invalid-punish-delay 'GEOFENCE_PUNISH_AFTER_SECONDS must be an integer between 1 and 3600' \
  'GEOFENCE_LANGUAGE=en' 'GEOFENCE_PROFILE=midcap' 'GEOFENCE_PUNISH_AFTER_SECONDS=0' \
  'HLL_HOST=192.0.2.10' 'HLL_PORT=7779' 'HLL_PASSWORD=secret'

assert_rejected invalid-sector-size 'GEOFENCE_SECTOR_SIZE must be a finite number' \
  'GEOFENCE_LANGUAGE=en' 'GEOFENCE_PROFILE=midcap' 'GEOFENCE_SECTOR_SIZE=invalid' \
  'HLL_HOST=192.0.2.10' 'HLL_PORT=7779' 'HLL_PASSWORD=secret'

assert_rejected nonfinite-offset 'GEOFENCE_CENTER_OFFSET_X must be a finite number' \
  'GEOFENCE_LANGUAGE=en' 'GEOFENCE_PROFILE=midcap' 'GEOFENCE_CENTER_OFFSET_X=1e309' \
  'HLL_HOST=192.0.2.10' 'HLL_PORT=7779' 'HLL_PASSWORD=secret'

assert_rejected incomplete-server 'Incomplete server 2: HLL_PASSWORD_2 is missing' \
  'GEOFENCE_LANGUAGE=en' 'GEOFENCE_PROFILE=midcap' \
  'HLL_HOST=192.0.2.10' 'HLL_PORT=7779' 'HLL_PASSWORD=secret' \
  'HLL_HOST_2=192.0.2.11' 'HLL_PORT_2=7789'

printf 'generate-config test passed\n'
