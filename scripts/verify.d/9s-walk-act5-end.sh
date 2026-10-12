scenario_name="Levels walked, Act 5 Worldstone Chamber, the Pandemonium areas (133 to 136) and the unused Hell rows (125 to 127)"
source scripts/verify.d/lib/levelwalk.sh
scenario_warnings_ok=1
scenario_turbo=1
scenario_env() {
  lw_s=""; lw_town=109
  # the Worldstone Chamber is entered through the red portal object that Baal's death opens in the Throne of
  # Destruction, not by a warp tile (the throne has only the way back), so the walk uses a portal like the others
  lw_into 131; lw_into 132; lw_home
  lw_into 133; lw_home
  lw_into 134; lw_home
  lw_into 135; lw_home
  lw_into 136; lw_home
  # 125 to 127 ("Hell 1 to 3") have no Vis/Warp link and no portal in the exe (unused rows, verify-level-graph.md
  # section 5): the debug portal is the only way in
  lw_into 125; lw_home
  lw_into 126; lw_home
  lw_into 127; lw_home
  lw_env act5end 5 "$lw_s"
}
scenario_check() { lw_check act5end 131 132 133 134 135 136 125 126 127; }
