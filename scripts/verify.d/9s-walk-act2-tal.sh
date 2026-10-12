scenario_name="Levels walked, Act 2 Tal Rasha's seven tombs (Tomb Tal 1 to 7 from the Canyon of the Magi)"
source scripts/verify.d/lib/levelwalk.sh
scenario_warnings_ok=1
scenario_turbo=1
scenario_env() {
  lw_s=""; lw_town=40
  lw_into 46
  lw_hop 66; lw_hop 46; lw_hop 67; lw_hop 46; lw_hop 68; lw_hop 46; lw_hop 69; lw_home
  lw_into 46
  lw_hop 70; lw_hop 46; lw_hop 71; lw_hop 46; lw_hop 72; lw_home
  lw_env act2tal 2 "$lw_s"
}
scenario_check() { lw_check act2tal 46 66 67 68 69 70 71 72; }
