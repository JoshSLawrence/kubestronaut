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
