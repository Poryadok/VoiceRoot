#!/usr/bin/env bash
# Normalize only the CR byte introduced by Windows CRLF output translation.
voice_manifest_lines=()
voice_normalize_manifest_lines() {
  local line=''
  voice_manifest_lines=()
  if [[ $# -gt 0 ]]; then
    while IFS= read -r line; do
      if [[ "${line}" == *$'\r' ]]; then line="${line%$'\r'}"; fi
      [[ "${line}" != *$'\r'* ]] || return 1
      voice_manifest_lines+=("${line}")
    done < "$1"
    if [[ -n "${line}" ]]; then
      [[ "${line}" != *$'\r'* ]] || return 1
      voice_manifest_lines+=("${line}")
    fi
  else
    while IFS= read -r line; do
      if [[ "${line}" == *$'\r' ]]; then line="${line%$'\r'}"; fi
      [[ "${line}" != *$'\r'* ]] || return 1
      voice_manifest_lines+=("${line}")
    done
    if [[ -n "${line}" ]]; then
      [[ "${line}" != *$'\r'* ]] || return 1
      voice_manifest_lines+=("${line}")
    fi
  fi
}
