#!/usr/bin/env zsh
############################################################
#  templates.zsh
#  - save / restore Logic project templates as brotli archives in the repo
#  - source it for the functions, or run: script/templates.zsh save|restore|list [name] [dest]
#  - 2026-10-01
############################################################
typeset -g LOGIC_TEMPLATE_DIR="$HOME/Music/Audio Music Apps/Project Templates"
typeset -g LOGIC_TEMPLATE_REPO="${${(%):-%x}:A:h:h}/templates"
typeset -g LOGIC_TEMPLATE_DEFAULT="T2026-07018-Bussed"

# logic_template_save [name] — archive a Logic template into the repo, verified by a round trip
logic_template_save() {
  local name=${1:-$LOGIC_TEMPLATE_DEFAULT}
  local src="$LOGIC_TEMPLATE_DIR/$name.logicx" out="$LOGIC_TEMPLATE_REPO/$name.logicx.tar.br"
  [[ -d $src ]] || { print -u2 "no template: $src"; return 1 }
  mkdir -p "$LOGIC_TEMPLATE_REPO"

  local check=$(mktemp -d)
  tar -cf - -C "$LOGIC_TEMPLATE_DIR" "$name.logicx" | brotli -q 11 -c > "$out.tmp" &&
    brotli -d -c "$out.tmp" | tar -xf - -C "$check" &&
    command diff -rq "$src" "$check/$name.logicx" >/dev/null || {
      print -u2 "archive did not round-trip; kept the previous $out"
      rm -rf "$check" "$out.tmp"; return 1
    }
  rm -rf "$check"
  mv "$out.tmp" "$out"
  print "saved $out ($(du -h "$out" | cut -f1), template saved $(stat -f '%Sm' -t '%Y-%m-%d %H:%M' "$src/Alternatives"/*/ProjectData(om[1])))"
}

# logic_template_restore [name] [dest] — unpack an archive; refuses to overwrite an existing template
logic_template_restore() {
  local name=${1:-$LOGIC_TEMPLATE_DEFAULT} dest=${2:-$LOGIC_TEMPLATE_DIR}
  local archive="$LOGIC_TEMPLATE_REPO/$name.logicx.tar.br"
  [[ -f $archive ]] || { print -u2 "no archive: $archive"; return 1 }
  [[ -e $dest/$name.logicx ]] && { print -u2 "$dest/$name.logicx exists; move it aside or pass another dest"; return 1 }
  mkdir -p "$dest"
  brotli -d -c "$archive" | tar -xf - -C "$dest" && print "restored $dest/$name.logicx"
}

# logic_template_list — templates in Logic vs archives in the repo
logic_template_list() {
  print "Logic ($LOGIC_TEMPLATE_DIR):"
  local t; for t in "$LOGIC_TEMPLATE_DIR"/*.logicx(N); do print "  ${t:t:r}"; done
  print "Repo ($LOGIC_TEMPLATE_REPO):"
  local a; for a in "$LOGIC_TEMPLATE_REPO"/*.tar.br(N); do print "  ${${a:t}%.logicx.tar.br}  $(du -h "$a" | cut -f1)"; done
}

if [[ $ZSH_EVAL_CONTEXT == toplevel ]]; then
  case $1 in
    save|restore|list) logic_template_$1 "${@:2}" ;;
    *) print -u2 "usage: ${0:t} save|restore|list [name] [dest]"; exit 2 ;;
  esac
fi
