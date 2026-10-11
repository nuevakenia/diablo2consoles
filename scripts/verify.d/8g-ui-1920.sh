scenario_name="UI clicks at 1920x1080 (skill buttons, mini panel, character panel, inventory, skill tree, stash, belt)"
scenario_realtime=1
scenario_timeout=240
source scripts/verify.d/lib/uiclicks.sh
scenario_env() { ui_clicks_env 1920x1080; }
scenario_check() { ui_clicks_check; }
