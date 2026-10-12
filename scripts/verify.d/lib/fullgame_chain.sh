# The segments of one difficulty of the full-game chain, in order. fg_chain <from> <to> runs the segments whose index is in
# the range, each from the .d2s the previous one exported (fg_d2s). Sourced after fullgame.sh and fullgame_scripts.sh.
# Segment names are the keys of fg_chain_names; a segment that does not pass stops the chain (fg_chain_fail names it).
fg_chain_names=(a1 a2a a2b a2c a2d a3a a3b a4a a4b a5a a5b a5c a5d)

fg_chain_script() { # <segment>: the autoscript of the segment (needs fg_d2s for the seed dependent routes)
  case $1 in
    a1) fg_sc_a1 ;; a2a) fg_sc_a2a ;; a2b) fg_sc_a2b ;; a2c) fg_sc_a2c ;; a2d) fg_sc_a2d $(fg_tomb) ;;
    a3a) fg_sc_a3a $(fg_jungle) ;; a3b) fg_sc_a3b ;;
    a4a) fg_sc_a4a ;; a4b) fg_sc_a4b ;;
    a5a) fg_sc_a5a ;; a5b) fg_sc_a5b ;; a5c) fg_sc_a5c ;; a5d) fg_sc_a5d ;;
  esac
}

fg_chain() { # <from> <to>
  local i from=${fg_chain_names[(i)$1]} to=${fg_chain_names[(i)$2]} seg
  fg_chain_fail=""
  for ((i = from; i <= to; i++)); do
    seg=${fg_chain_names[$i]}
    # a failed segment is recorded and the chain goes on from the .d2s it exported (the next segments show what depends on it)
    fg_seg $seg "$(fg_chain_script $seg | tr -d '\n')" || { fg_chain_fail="$fg_chain_fail $seg"; echo "FG SEGMENT $seg: script steps failed"; }
    # the segment ended by itself (OD2_AUTOEXIT writes the .d2s); no .d2s means the game died
    [ -s "$fg_d2s" ] || { echo "FG SEGMENT $seg: no .d2s"; return 1; }
  done
}
