package d2records

// ObjectDetails stores all of the ObjectDetailRecords
type ObjectDetails map[int]*ObjectDetailRecord

// ObjectDetailRecord represents the settings for one type of object from objects.txt
type ObjectDetailRecord struct {
	Index         int    // Line number in file, this is the actual index used for objects
	FrameCount    [8]int // how many frames does this mode have, 0 = skip
	FrameDelta    [8]int // what rate is the animation played at (256 = 100% speed)
	LightDiameter [8]int

	StartFrame [8]int

	OrderFlag   [8]int //  0 = object, 1 = floor, 2 = wall
	Parm        [8]int // unknown
	Name        string
	Description string

	// Don't use, get token from objtypes
	token string // refers to what graphics this object uses

	// Don't use, index by line number
	id              int //nolint:golint,stylecheck // unused, indexed by line number instead
	SpawnMax        int // unused?
	TrapProbability int // unused

	SizeX int
	SizeY int

	XOffset int // in pixels offset
	YOffset int

	SubClass int // subclass of object:
	// 1 = shrine
	// 2 = obelisk
	// 4 = portal
	// 8 = container
	// 16 = arcane sanctuary gateway
	// 32 = well
	// 64 = waypoint
	// 128 = secret jails door

	OperateRange int // distance object can be used from, might be unused

	Act int // what acts this object can appear in (15 = all three)

	Damage int // amount of damage done by this (used depending on operatefn)

	Left   int // unknown, clickable bounding box?
	Top    int
	Width  int
	Height int

	OperateFn  int // what function is called when the player clicks on the object
	PopulateFn int // what function is used to spawn this object?
	InitFn     int // what function is run when the object is initialized?

	// 'To ...' or 'trap door' when highlighting, not sure which is T/F
	AutoMap int // controls how this object appears on the map
	// 0 = it doesn't, rest of modes need to be analyzed

	CycleAnimation   [8]bool // probably whether animation loops
	Selectable       [8]bool // is this mode selectable
	HasAnimationMode [8]bool // 'Mode' in source, true if this mode is used
	IsAttackable     bool    // do we kick it when interacting
	IsDoor           bool
	Draw             bool // if false, object isn't drawn (shadow is still drawn and player can still select though)
	Restore          bool // if true, object is stored in memory and will be retained if you leave and re-enter the area
	Lockable         bool
	Sync             bool // unknown
	Flicker          bool // light flickers if true
	Overlay          bool // unknown
	RestoreVirgins   bool // if true, only restores unused objects (see Restore)

}
