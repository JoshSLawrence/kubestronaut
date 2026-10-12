#!/usr/bin/env bash
# Helper library sourced by scenario setup, solution and check scripts via
# "$KS_LIB". It only talks to the cluster through kubectl and to nodes through
# the kubestronaut binary, so scripts stay independent of the backend.
#
# The file is sourced, so it must not change shell options of the caller.

ks_log() {
  printf '[ks] %s\n' "$*" >&2
}

# ks_node <alias> <cmd...>: run a command on a node (controlplane, node01...).
ks_node() {
  local alias=$1
  shift
  "$KS_BIN" ssh "$alias" -- "$@"
}

# ks_node_name <alias>: print the real node name for an alias.
ks_node_name() {
  local var
  var="KS_NODE_$(printf '%s' "$1" | tr '[:lower:]' '[:upper:]')"
  if [ -z "${!var:-}" ]; then
    ks_log "unknown node alias '$1' (no \$$var): use controlplane or node01, node02..."
    return 1
  fi
  printf '%s\n' "${!var}"
}

# ks_wait_node <alias> Ready|NotReady <seconds>: poll until the node reaches
# the wanted state.
ks_wait_node() {
  local alias=$1 want=$2 secs=$3 name status deadline
  name=$(ks_node_name "$alias") || return 1
  deadline=$((SECONDS + secs))
  while [ "$SECONDS" -lt "$deadline" ]; do
    status=$(kubectl get node "$name" \
      -o 'jsonpath={.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || true)
    case "$want" in
      Ready) [ "$status" = "True" ] && return 0 ;;
      NotReady) [ "$status" != "True" ] && return 0 ;;
      *)
        ks_log "ks_wait_node: state must be Ready or NotReady, got '$want'"
        return 1
        ;;
    esac
    sleep 2
  done
  ks_log "node $name did not become $want within ${secs}s"
  return 1
}

# ks_copy_to <alias> <local-dir> <node-dir>: copy the contents of a local
# directory onto a node, creating the destination. It streams a tar archive
# through the node, so nothing is needed on it but mkdir and tar.
ks_copy_to() {
  local alias=$1 src=$2 dest=$3
  if [ ! -d "$src" ]; then
    ks_log "ks_copy_to: '$src' is not a directory"
    return 1
  fi
  ks_node "$alias" mkdir -p "$dest"
  # macOS tar adds AppleDouble "._" entries for extended attributes unless
  # both of these are set, and "kubectl apply -f dir/" chokes on them. On the
  # node, take ownership and permissions from the node's defaults instead of
  # the host's uid and mode; --no-overwrite-dir stops GNU tar from applying the
  # "./" entry's mode (often 0700) to the destination directory.
  COPYFILE_DISABLE=1 tar --no-xattrs -C "$src" -cf - . |
    ks_node "$alias" tar --no-same-owner --no-same-permissions --no-overwrite-dir -xf - -C "$dest"
}

# _ks_sha256: print the sha256 of stdin. macOS has no sha256sum.
_ks_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | cut -d' ' -f1
  else
    shasum -a 256 | cut -d' ' -f1
  fi
}

# _ks_answer_file <key>: print the path of an answer file, or fail on a key
# that could escape the answers directory.
_ks_answer_file() {
  # ASCII only: in other locales a range like a-z can match more characters.
  local LC_ALL=C
  case "$1" in
    '' | *[!A-Za-z0-9._-]* | .*)
      ks_log "answer key must be letters, digits, '.', '_' or '-', got '$1'"
      return 1
      ;;
  esac
  printf '%s\n' "$KS_SCENARIO_DIR/.answers/$1"
}

# ks_answer_put <key> <value>: remember the sha256 of a value that only setup
# knew (a random token, a uid). The learner can read the hash, but cannot
# reverse it. Checks compare with ks_answer_check.
ks_answer_put() {
  local file
  file=$(_ks_answer_file "$1") || return 1
  (
    umask 077
    mkdir -p "$KS_SCENARIO_DIR/.answers"
    printf '%s' "$2" | _ks_sha256 >"$file"
  )
}

# ks_answer_check <key> <value>: succeed when the value hashes to the stored
# answer. Callers write their own message; only a missing answer is logged.
ks_answer_check() {
  local file want got
  file=$(_ks_answer_file "$1") || return 1
  if [ ! -f "$file" ]; then
    ks_log "no recorded answer for '$1': setup did not run ks_answer_put"
    return 1
  fi
  want=$(cat "$file")
  got=$(printf '%s' "$2" | _ks_sha256)
  [ "$want" = "$got" ]
}
