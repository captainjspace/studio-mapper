#!/usr/bin/env zsh
############################################################
#  nuc.zsh
#  - run the studio-map service on the NUC (rootless podman quadlet)
#  - source it for the functions, or run: script/nuc.zsh connect|status|logs|restart|stop|deploy|open
#  - 2026-10-02
############################################################
typeset -g NUC_HOST="amazing-kitty.landmania.internal"
typeset -g NUC_CONNECTION="amazing-kitty"
typeset -g NUC_URL="http://$NUC_HOST:8080"
typeset -g NUC_REPO="${${(%):-%x}:A:h:h}"

# nuc_connect — (re)create the podman connection by hostname, so podman --connection and Podman Desktop see the NUC
nuc_connect() {
  podman system connection rm "$NUC_CONNECTION" 2>/dev/null
  podman system connection rm remote-host 2>/dev/null
  podman system connection add "$NUC_CONNECTION" "ssh://joshua@$NUC_HOST/run/user/1000/podman/podman.sock" &&
    podman --connection "$NUC_CONNECTION" ps --format '{{.Names}}  {{.Status}}'
}

# nuc_status — unit state, container, and the health endpoint
nuc_status() {
  ssh "$NUC_HOST" 'systemctl --user --no-pager status studio-map | head -5; podman ps --filter name=studio-map --format "{{.Names}}  {{.Status}}  {{.Image}}"'
  print "healthz: $(curl -s -m5 "$NUC_URL/healthz" || print unreachable)"
}

# nuc_logs [-f] — the service journal
nuc_logs() { ssh -t "$NUC_HOST" "journalctl --user -u studio-map --no-pager -n 50 $*" }

nuc_restart() { ssh "$NUC_HOST" 'systemctl --user restart studio-map' && nuc_status }
nuc_stop()    { ssh "$NUC_HOST" 'systemctl --user stop studio-map' }

# nuc_deploy — rebuild the image and push image, config and sheets (same as make nuc-deploy)
nuc_deploy() { make -C "$NUC_REPO" nuc-deploy && nuc_status }

nuc_open() { open "$NUC_URL" }

if [[ $ZSH_EVAL_CONTEXT == toplevel ]]; then
  cmd=${1:-status}; shift 2>/dev/null
  (( $+functions[nuc_$cmd] )) || { print -u2 "usage: $0 connect|status|logs [-f]|restart|stop|deploy|open"; exit 2 }
  nuc_$cmd "$@"
fi
