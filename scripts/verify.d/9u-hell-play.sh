scenario_name="Hell play (a level 94 hero started at Hell through the difficulty screen: monster scaling against the real tables, resist penalty, automap difficulty line, Andariel killed, drop levels, act travel 1 to 5)"
scenario_timeout=600
scenario_turbo=1  # scripted autoplay, no wall-clock assertions
scenario_warnings_ok=1  # "KILL giving up" lines of fights are normal; real errors are caught by the AUTOSCRIPT result
source scripts/verify.d/lib/hell.sh
scenario_env() { w36_play_env 2; }
scenario_check() { w36_play_check 2; }
