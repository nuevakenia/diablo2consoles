scenario_name="Nightmare quest rewards, Act 5 (OD2_AUTOQUEST=act5 at Nightmare: Malah's potion once per game, Anya's item tier, Rite experience, Baal's gold)"
scenario_turbo=1  # the quest scripts need no wall clock
source scripts/verify.d/lib/hell.sh
scenario_env() { w36_quest_env 1 act5; }
scenario_check() { w36_quest_check 1 act5; }
