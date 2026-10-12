scenario_name="Levels walked, Act 3 Spider Cave 1 and the dungeon levels 1B / 2B and their treasure rooms (84, 87, 89, 90, 91)"
source scripts/verify.d/lib/levelwalk.sh
scenario_warnings_ok=1
scenario_turbo=1
scenario_env() {
  lw_s=""; lw_town=75
  lw_into 76; lw_hop 84; lw_home
  lw_into 86; lw_hop 87; lw_hop 90; lw_home
  lw_into 88; lw_hop 89; lw_hop 91; lw_home
  lw_env act3dungeons 3 "$lw_s"
}
scenario_check() { lw_check act3dungeons 84 87 89 90 91; }
