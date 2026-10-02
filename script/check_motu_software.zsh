#!/usr/bin/env zsh
############################################################
#  check_motu_software.zsh
#  -  eval status of MOTU software 
#  - 2026-09-16
############################################################
typeset MOTU_PKG="motu";
typeset -a KNOWN_MOTU

# maintained list
KNOWN_MOTU=(
  "/Applications/MOTU Audio Tools.app"
  "/Applications/CueMix Pro.app"
)
#package inventory
inventory(){
  printf '\n-----\nINVENTORY\n-----\n'
  typeset -a motu_package_id
  motu_package_id=( ${(s: :)$(pkgutil --pkgs | grep -i $MOTU_PKG)} )
  echo "MOTU Packages $motu_package_id";

  for pkg in "${motu_package_id[@]}"; do
  printf '\n--------\nPACKAGE ID: %s\n---------\n' $pkg
  pkgutil --files $pkg  # what each package put where
done;
ls /Library/Audio/Plug-Ins/HAL | grep -i $MOTU_PKG
systemextensionsctl list | grep -i $MOTU_PKG
}
# check an app
check_motu() {
  printf '\n-----\nFunction: check_motu\n-----\n'
  local motu=$1 
  [[ -z $motu ]] && return
  printf 'Application: %s\n' $motu
  mdls -name kMDItemVersion -name kMDItemContentCreationDate $motu
  codesign -dv $motu 2>&1 | grep Identifier
}

#brew install switchaudio-osx
list_available_audio_sources() {
  printf '\n-----\nLIST AVAILABLE AUDIO SOURCES\n-----\n'
  SwitchAudioSource -a            # list device names
}  

check_default_audio() {
  printf '\n-----\nCURRENT DEFAULT AUDIO\n-----\n'
  print "output: $(SwitchAudioSource -c -t output)"
  print "input:  $(SwitchAudioSource -c -t input)"
}

check_10pre_state() {
  printf '\n-----\n10PRE STATE\n-----\n'
  system_profiler SPThunderboltDataType | grep -A6 'MOTU Thunderbolt' | grep -E 'Mode|Speed' \
    || print "10pre: NOT on Thunderbolt/USB4"
  launchctl list | grep -q com.motu.CueMixPro.reenumerator \
    && print "reenumerator: running" || print "reenumerator: NOT running"
  SwitchAudioSource -a | grep -qx 10pre \
    && print "CoreAudio: 10pre present" || print "CoreAudio: 10pre MISSING"
  log show --last 10m --predicate 'process == "coreaudiod"' 2>/dev/null \
    | grep -E "activating device.*10pa|_Deactivate.*10pa|Transport:.*'(usb |thun)'" | tail -8
}

set_audio_sources(){
  local audio_source="$1";
  valid_sources=( 10pre "Ultralite AVB" )
  if (($valid_sources[(I)$audio_source])); then
    SwitchAudioSource -t output -s $audio_source
    SwitchAudioSource -t input  -s $audio_source
  fi
}

check_thunderbolt() {
  printf '\n-----\nCHECK THUNDERBOLT\n-----\n'
  system_profiler SPThunderboltDataType
}

check_active() {
  printf '\n-----\nCHECK ACTIVE SOFTWARE\n-----\n'
  kmutil showloaded 2>/dev/null | grep -i $MOTU_PKG
  launchctl list | grep -i motu
  sudo launchctl list | grep -i motu
  pkgutil --file-info "/Applications/CueMix Pro.app"
}

check_drivers() {
  printf '\n-----\nLOADED / RUNNING\n-----\n'
  kmutil showloaded 2>/dev/null | grep -i motu
  launchctl list | grep -i motu
  ioreg -l -w0 | grep -i -A12 '10pre' | grep -iE 'bundle|UserServer' | sort -u
  system_profiler SPThunderboltDataType | grep -A3 'MOTU' | grep -E 'Device Name|Mode|Speed'
}

check_10pre_driver(){
  printf '\n-----\nCHECK 10 PRE DRIVER\n-----\n'
  ioreg -rl -c IOUserService | grep -iE '"IOUserServerName"|"CFBundleIdentifier"|10pre' | sort -u
  ioreg -l -w0 | grep -i -B3 -A12 '10pre' | grep -iE 'class|bundle|UserServer'
}
main(){
  check_thunderbolt;
  inventory 2>&1;
  for motu in ${KNOWN_MOTU[@]}; do 
    check_motu "$motu";
  done
  list_av_audio_sources;
  check_default_audio;
  check_active;
  check_drivers;
  check_10pre_state;
}

main "$@"

