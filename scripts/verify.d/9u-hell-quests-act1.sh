scenario_name="Hell quest rewards, Act 1 (OD2_AUTOQUEST=act1 at Hell: Akara's ring for Cain by difficulty)"
scenario_turbo=1  # the quest scripts need no wall clock
source scripts/verify.d/lib/hell.sh
scenario_env() { w36_quest_env 2 act1; }
scenario_check() { w36_quest_check 2 act1; }
