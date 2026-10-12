package d2quest

// Save points of the original server (Game.exe, callers of SERVER_SaveClientPlayer 0x52fda0), VERIFIED:
//   - every 8192 server frames while no client was kicked (GAME_UpdateServerClients), 327.68 s at 25 fps
//     (autosave.go has this one);
//   - when a client leaves the game (GAME_SaveAllPlayersOnClientLeave, GAME_LeaveServerGame), on game shutdown, and
//     when a client times out;
//   - when the player enters the dead mode (PLRMODE_ServerEnterDeadMode 0x57dd94);
//   - GAME_FlushSaveChunksForAllPlayers (0x52bce0) is called from the Eve of Destruction kill handler and the
//     Baal end code 0x58c568 (both Baal), and from 0x5b2943, an unnamed function in the Act IV quest code range:
//     that one is taken to be the Diablo kill (UNVERIFIED).

// KillForcesSave reports whether the death of a monster class makes the server save every player at once.
func KillForcesSave(class int) bool { return class == NPCBaalCrab || class == NPCDiablo }
