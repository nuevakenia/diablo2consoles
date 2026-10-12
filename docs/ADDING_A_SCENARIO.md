# Adding a verify scenario

A scenario is one small zsh file in `scripts/verify.d/` that starts the real engine once, with a scripted hero, and then
greps the log for proof. This page is the short practical version; the reference is
[TESTING.md](TESTING.md#5-scriptsverifysh-and-scenarios) (file format, runner internals) and the autotest variables are in
[TESTING.md section 4](TESTING.md#4-the-in-game-harness-od2_auto).

You need a Mac with a GUI session, your own Diablo II 1.14b + LoD files and a real `.d2s` character. Nothing you write may
contain game data: a scenario holds environment variables and grep patterns only.

## 1. Make the behaviour observable

The check can only see the log. If the behaviour you care about prints nothing stable, add one log line in the game code
with a clear prefix and the values you want to assert (`MYTHING hit dmg=12 life=88`). Prefer values to words.

## 2. Write the file

Name it `NN-topic.sh`. `NN` (two hex-ish characters) orders the run; pick one after anything it depends on (for example
`9c-act1-reload` loads the save that `9b-act1-playthrough` exported). Copy a small existing scenario such as
`scripts/verify.d/10-menus-trade.sh` and change three pieces:

```zsh
scenario_name="my thing (one line a human can read)"
scenario_env() {                 # print shell lines that become part of the game's launch script
  echo 'export OD2_AUTOMONSTER="zombie1,2" OD2_AUTOCAST="Fire Bolt,4"'
}                                # $save, $tmp and $OD2_PORT are available
scenario_check() {               # read "$log.txt" (ANSI stripped); set fail=1 when something is wrong
  grep -qE "AUTOCAST summary .* kills=[1-9]" $log.txt || { echo "FAIL: no kill"; fail=1; }
}
# scenario_warnings_ok=1         # only if [ERROR]/[WARNING] lines are expected
```

Rules that keep scenarios useful:

* **Assert specific values** (`kills=[1-9]`, `mana_paid=2.50`), not only that a line exists.
* **Print the evidence** (a `grep ... | cut -c1-200` before the asserts) so a failure log explains itself.
* **Wait on log lines, not time**, in multi-process scenarios (`waitlog:` autoscript steps).
* Use the existing helpers in `scripts/verify.d/lib/` (`hero.sh` for a ready-made hero, `display.sh`, `levelwalk.sh`,
  `uiclicks.sh`) instead of re-inventing them; `OD2_HERO=barb` picks the generated Barbarian for playthroughs.
* Long playthroughs may use `OD2_TURBO`; keep clock-sensitive checks in a normal-speed scenario.
* Do not edit `scripts/verify.sh`: it picks up every file in `verify.d/` by name.

## 3. Run it alone

Use a private config directory so the run does not touch your real saves, and never start the game binary by hand:

```sh
mkdir -p .verify-config && cp "$HOME/Library/Application Support/OpenDiablo2/config.json" .verify-config/
OD2_VERIFY_CONFIG_DIR=$PWD/.verify-config OD2_MAX_GAMES=3 \
D2S_SAMPLE_BODY=/path/to/real.d2s D2S_SAMPLE_BODY_JSON=/path/to/expected.json D2_TABLES=/path/to/extracted/tables \
OD2_VERIFY_ONLY="NN-topic" scripts/verify.sh
```

`OD2_MAX_GAMES` caps how many games run at once on the machine (`scripts/gameslot.sh`); a scenario that waits for a slot
should be left to wait. A scenario that fails once is retried once automatically; if it is red twice, read the log (kept
under `/tmp/od2-verify.*` for failures) before changing anything.

## 4. Prove it can fail

Break the behaviour, or change the expected value, and confirm the scenario goes red. A check that cannot fail is not
evidence. Put the file back.

## 5. Document it

Add the scenario to the list in [TESTING.md](TESTING.md#51-scenario-file-format), list any new `OD2_*` variable there (and
in [macos-quickstart.md](macos-quickstart.md) if a player would set it), and update the matching row of
[STATUS_MATRIX.md](STATUS_MATRIX.md) so it names your scenario as the evidence. Then follow the gate in
[CONTRIBUTING.md](../CONTRIBUTING.md).
