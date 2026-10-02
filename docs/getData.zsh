#!/usr/bin/env zsh
############################################################
#  getData.zsh
#  - pull the studio Google Sheet live as CSV, then convert to JSON
#  - sheet link lives in the keychain: ks show studio-data-url (any share/view link works)
#  - one-time auth (read-only Sheets access, separate from your normal gcloud login):
#      gcloud auth application-default login --scopes=https://www.googleapis.com/auth/spreadsheets.readonly,https://www.googleapis.com/auth/cloud-platform
#  - writes ./studio-sheet.csv and ./studio-data.json (does NOT touch ../data/studio-inputs.csv)
############################################################
emulate -L zsh
setopt err_return pipe_fail

main(){
  local raw id gid csv=studio-sheet.csv json=studio-data.json
  raw="$(ks show studio-data-url)"
  raw=${raw//\\/}                                   # links pasted shell-escaped: view\?gid\=0
  id=$(print -r -- "$raw" | sed -nE 's#.*/spreadsheets/d/([^/?#]+).*#\1#p')
  gid=$(print -r -- "$raw" | sed -nE 's#.*[?&#]gid=([0-9]+).*#\1#p')
  [[ -n $id ]] || { print -u2 "no spreadsheet id in the keychain link"; return 1 }

  curl -fsSL -o "$csv.tmp" \
    -H "Authorization: Bearer $(gcloud auth application-default print-access-token)" \
    "https://docs.google.com/spreadsheets/d/$id/export?format=csv&gid=${gid:-0}" || {
      print -u2 "download failed: run the one-time auth command in this script's header"
      rm -f "$csv.tmp"; return 1
    }
  head -1 "$csv.tmp" | grep -q 'Interface Input' || {
    print -u2 "that doesn't look like the studio sheet (no 'Interface Input' column); kept the old $csv"
    rm -f "$csv.tmp"; return 1
  }
  mv "$csv.tmp" "$csv"
  pnpm dlx csvtojson "$csv" > "$json"
  print "$csv: $(( $(wc -l < "$csv") - 1 )) rows; columns: $(head -1 "$csv")"
}
main "$@"
