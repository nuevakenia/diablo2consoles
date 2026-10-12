package d2audio

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2animspeed"

// FormMonSoundsID is the MonSounds.txt row of the monster a Druid form is drawn
// as (monstats Wolf 430 and Bear 431 name the rows werewolf and werebear). A
// shapeshifted hero is the monster class for the animations (VERIFIED earlier,
// UNIT_GetShapeshiftAdjustedMode 0x646620). VERIFIED in wave 37: the sound
// code asks the same adjusted type and class (UNITSND_GetUnitTypeForSound 0x4c6e40,
// UNITSND_GetUnitIdForSound 0x4c6de0), so a shapeshifted hero is a monster of class
// 430/431 for sounds and uses its MonSounds row (werewolf / werebear, which name the
// Sounds.txt rows druidwere*_attack, _bite, _hit, _neutral). The form change itself
// plays the states.txt onsound / offsound (druid_morph, a real file; STATE_ClientAddState
// 0x4d68c0 and CLIENT_RemovePlayerState 0x4d69d0), not the *_morph rows (none.wav).
func FormMonSoundsID(token string) string {
	switch token {
	case d2animspeed.TokenWerewolf:
		return "werewolf"
	case d2animspeed.TokenWerebear:
		return "werebear"
	}

	return ""
}

// Sounds.txt handles of the sounds objects and the town portal make. The
// object rows are VERIFIED in wave 37 (ObjectSounds); the portal cast and enter
// rows are the exe's hero sound ids 7 and 8 (UNITSND_PlayUnitSoundById 0x4c8470
// plays rows 0x8b6 and 0x8b7 at the unit), when the server sends them is UNVERIFIED.
const (
	HandleWaypointOpen    = "object_waypoint_open"
	HandleTownPortalOpen  = "object_townportal"
	HandleTownPortalCast  = "player_townportal_cast"
	HandleTownPortalEnter = "player_townportal_enter"
)
