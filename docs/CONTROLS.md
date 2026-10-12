# Controls

Defaults for the macOS build. Every key can be changed in Esc -> Options -> Configure Controls; changes are saved in
`config.json` (`KeyBindings`, by key name, e.g. `"ToggleInventoryPanel": ["I", "B"]`) and come back on the next start.
Esc cannot be rebound. A saved key that another action already holds moves to the saved action.

## Mouse and trackpad (one button is enough)

| Input | Action |
| --- | --- |
| Left click on the ground | Walk there, whatever the left skill is |
| Hold the left button (see 8f-hold-walk) | Keep walking toward the cursor |
| Left click on a monster | Use the left skill (Attack walks up and swings) |
| Shift + left click | Use the left skill standing still at the clicked spot (Fire Wall, Blizzard, ...); on a monster, attack without moving |
| Right click, two-finger click, Control + click or Command + click | Use the right skill |
| Hold Alt / Option | Show the names of the items lying on the ground (releases when you let go) |
| Left click on an item | Walk to and pick it up |

On macOS Control + click is a right click, so Control (the default "hold run" key) does not run while you click. Press `R`
to toggle run / walk instead.

## Keyboard

| Key | Action |
| --- | --- |
| I (or B) | Inventory |
| C (or A) | Character |
| T | Skill tree |
| L | Quest log (Q drinks the next potion in this port) |
| P | Party |
| M | Message log |
| H | Help overlay |
| Tab | Automap |
| 1 - 4 | Belt potions |
| F1 - F8 | Skill hotkeys |
| R | Toggle run / walk |
| W | Swap weapons |
| X | Town portal |
| Esc | Game menu, or close the open window |
| Cmd + Enter | Full screen |
| Cmd + Q | Quit |

The full list is in Configure Controls.

## Keyboard only

* Main menu pages (main, multiplayer, TCP/IP): Up / Down or Tab / Shift+Tab move the highlighted button, Enter or Space
  presses it, Esc goes back. The highlight is the pressed look of the button.
* NPC list (Talk / Trade / Gamble / ...): Up / Down or Tab / Shift+Tab move the highlighted row (wraps around), Enter or
  Space chooses it, Esc picks Cancel.
* Game menu and its pages: Up / Down, Enter, Esc.
* Moving items without the mouse (inventory, stash, cube, vendor window): with one of them open, an arrow key shows a
  yellow cell cursor; arrows move it, Enter or Space is a left click on that cell (pick the item up, drop or swap the
  held one, buy from a vendor), Ctrl + Enter auto-places, PageUp / PageDown (or Home / End) hop to the previous / next
  open panel. Enter is the chat key again once all panels are closed.
* Mouse wheel: previous / next skill (the default binding); it steps the right skill, Shift + wheel the left one.
  Skills are visited in skill id order (which button the original changes is not verified).

## Gamepad

See [gamepad.md](gamepad.md) for the layout and remapping. Menu play-through:

* While the game menu or an NPC list is open, the D-pad presses the arrow keys (so it does not drink potions) and A presses
  Enter. Moving a stick hands A back to the cursor (click at the cursor).
* Panels (inventory, character, ...) use the right stick as the cursor and A as the left click, B as the right click.
* Town: left stick walks, Start opens the menu, X / Y open inventory / character.

## Check it

`scripts/verify.d/8e-mac-input.sh` (clicks, Control + click, Shift, keys, rebinding saved to `config.json`),
`8f-hold-walk.sh`, `9e-gamepad.sh`. Unit tests: `d2game/d2player/input_mapping_test.go`, `key_swap_persist_test.go`,
`npc_menu_keys_test.go`, `d2core/d2input/d2gamepad/gamepad_test.go`.
