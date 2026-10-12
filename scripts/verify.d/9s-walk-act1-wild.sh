scenario_name="Levels walked, Act 1 Stony Field to Tamoe Highland (4 to 7, through Underground Passage 1), Tristram and the Moo Moo Farm"
source scripts/verify.d/lib/levelwalk.sh
scenario_warnings_ok=1
scenario_turbo=1
scenario_env() {
  lw_s=""; lw_town=1
  # Stony Field and the Dark Wood only meet through Underground Passage Level 1 (Levels.txt Vis of 4, 5 and 10)
  lw_into 4; lw_hop 10; lw_hop 5; lw_hop 6; lw_hop 7; lw_home
  lw_into 38; lw_home
  # the Moo Moo Farm portal is locked until Eve of Destruction (record slot 40) is done
  lw_s+=";say:completequest 5 6"
  lw_into 39; lw_home
  lw_env act1wild 1 "$lw_s"
}
scenario_check() { lw_check act1wild 4 10 5 6 7 38 39; }
