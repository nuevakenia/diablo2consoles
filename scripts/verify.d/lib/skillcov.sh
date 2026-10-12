# Shared by the w27 per-class skill coverage scenarios (86d ... 86j). Sourced by the scenario file; uses $log and $fail of verify.sh.
#
#   skillcov_report "<skill>" ...     prints, per skill, which kinds of log lines carry skill="<skill>" (for the run output)
#   skillcov_cast "<skill>" ...       FAIL unless the skill was cast to the end with an effect: a "CAST do skill=... ok=true" line
#                                     with at least one missile, a melee strike or an effect (the engine's own account of what the cast made)
#   skillcov_effect <kind> "<skill>" [arg]
#                                     FAIL unless the cast's outcome (damage, state, summon, missile created / hit...) is in the log (below)
skillcov_report() {
  local s
  for s in "$@"; do
    echo "  $s: $(grep -E "skill=\"$s\"" $log.txt | grep -vE "AUTOCAST|MANA paid" | grep -oE '\] [A-Z_]+ [a-z_]+' | sort | uniq -c | sed 's/ *\([0-9]*\) \] /\1x /' | tr '\n' ',' | cut -c1-200)"
  done
}
skillcov_cast() {
  local s
  for s in "$@"; do
    # a channelled skill (Arctic Blast) makes its stream on later ticks: CAST do shows no missile, a CHANNEL line follows
    grep -E "CAST do skill=\"$s\" " $log.txt | grep -E "ok=true" | grep -qE "missiles=[1-9]|melee=true|effects=[1-9]" \
      || { grep -qE "CAST do skill=\"$s\" .*ok=true" $log.txt && grep -qE "CHANNEL skill=\"$s\" " $log.txt; } \
      || { echo "FAIL: $s was not cast to the end with an effect (CAST do ok=true with missiles/melee/effects)"; fail=1; }
  done
}
# skillcov_effect <kind> "<skill>" [arg]   FAIL unless the cast had the outcome of <kind>:
#   dmg      a DAMAGE line of the skill with dmg >= 1 (its own strike or missile hurt a monster)
#   state    a STATE apply/aura/totem/hit/storm line of the skill, or a STATE area line that affected >= 1 unit (a state or
#            aura took effect on somebody)
#   summon   a SUMMON line of the skill with created >= 1
#   melee    a SKILL melee line of the skill that hit
#   area     a SKILL area line of the skill
#   chan     a CHANNEL line of the skill (a channelled stream; its own damage is a later line)
#   missile  (arg = missile name) the skill's missile was created: MISSILE create name=<arg>
#   hit      (arg = missile name) a MISSILE hit name=<arg> line: the missile reached a monster
#   raw      (arg = regex) any line matching the regex (for outcomes that carry no skill= field)
skillcov_effect() {
  local kind="$1" s="$2" arg="${3:-}" re
  case "$kind" in
    dmg)     re="DAMAGE skill=\"$s\" .*dmg=[1-9]" ;;
    state)   re="STATE (apply|aura|totem|hit|storm) skill=\"$s\" |STATE area skill=\"$s\" .*affected=[1-9]" ;;
    summon)  re="SUMMON skill=\"$s\" .*created=[1-9]" ;;
    melee)   re="SKILL melee skill=\"$s\" .*hit=true" ;;
    area)    re="SKILL area skill=\"$s\" " ;;
    chan)    re="CHANNEL skill=\"$s\" " ;;
    missile) re="MISSILE create name=$arg( |\$)" ;;
    hit)     re="MISSILE hit name=$arg( |\$)" ;;
    raw)     re="$arg" ;;
  esac
  grep -qE "$re" $log.txt || { echo "FAIL: $s: no '$kind' outcome ${arg:+($arg) }in the log"; fail=1; }
}
