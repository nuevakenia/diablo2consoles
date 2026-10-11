scenario_name="menu flow at 1920x1080 (main menu, character select, hero creation, escape menu, options: screenshot size, menu content centred 4:3)"
scenario_realtime=1
scenario_timeout=240
source scripts/verify.d/lib/menushots.sh
scenario_env() { menu_env 1920 1080; }
scenario_check() { menu_check 1920 1080; }
