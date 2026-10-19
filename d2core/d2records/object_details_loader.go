package d2records

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
)

//nolint:funlen // Makes no sense to split
func objectDetailsLoader(r *RecordManager, d *d2txt.DataDictionary) error {
	records := make(ObjectDetails)

	i := 0

	for d.Next() {
		record := &ObjectDetailRecord{
			Index:       i,
			Name:        d.String("Name"),
			Description: d.String("description - not loaded"),
			id:          d.Number("Id"),
			token:       d.String("Token"),

			SpawnMax: d.Number("SpawnMax"),
			Selectable: [8]bool{
				d.Number("Selectable0") == 1,
				d.Number("Selectable1") == 1,
				d.Number("Selectable2") == 1,
				d.Number("Selectable3") == 1,
				d.Number("Selectable4") == 1,
				d.Number("Selectable5") == 1,
				d.Number("Selectable6") == 1,
				d.Number("Selectable7") == 1,
			},
			TrapProbability: d.Number("TrapProb"),

			SizeX: d.Number("SizeX"),
			SizeY: d.Number("SizeY"),

			FrameCount: [8]int{
				d.Number("FrameCnt0"),
				d.Number("FrameCnt1"),
				d.Number("FrameCnt2"),
				d.Number("FrameCnt3"),
				d.Number("FrameCnt4"),
				d.Number("FrameCnt5"),
				d.Number("FrameCnt6"),
				d.Number("FrameCnt7"),
			},
			FrameDelta: [8]int{
				d.Number("FrameDelta0"),
				d.Number("FrameDelta1"),
				d.Number("FrameDelta2"),
				d.Number("FrameDelta3"),
				d.Number("FrameDelta4"),
				d.Number("FrameDelta5"),
				d.Number("FrameDelta6"),
				d.Number("FrameDelta7"),
			},
			CycleAnimation: [8]bool{
				d.Number("CycleAnim0") == 1,
				d.Number("CycleAnim1") == 1,
				d.Number("CycleAnim2") == 1,
				d.Number("CycleAnim3") == 1,
				d.Number("CycleAnim4") == 1,
				d.Number("CycleAnim5") == 1,
				d.Number("CycleAnim6") == 1,
				d.Number("CycleAnim7") == 1,
			},
			LightDiameter: [8]int{
				d.Number("Lit0"),
				d.Number("Lit1"),
				d.Number("Lit2"),
				d.Number("Lit3"),
				d.Number("Lit4"),
				d.Number("Lit5"),
				d.Number("Lit6"),
				d.Number("Lit7"),
			},
			IsAttackable: d.Number("IsAttackable0") == 1,
			StartFrame: [8]int{
				d.Number("Start0"),
				d.Number("Start1"),
				d.Number("Start2"),
				d.Number("Start3"),
				d.Number("Start4"),
				d.Number("Start5"),
				d.Number("Start6"),
				d.Number("Start7"),
			},

			IsDoor: d.Number("IsDoor") == 1,

			OrderFlag: [8]int{
				d.Number("OrderFlag0"),
				d.Number("OrderFlag1"),
				d.Number("OrderFlag2"),
				d.Number("OrderFlag3"),
				d.Number("OrderFlag4"),
				d.Number("OrderFlag5"),
				d.Number("OrderFlag6"),
				d.Number("OrderFlag7"),
			},
			HasAnimationMode: [8]bool{
				d.Number("Mode0") == 1,
				d.Number("Mode1") == 1,
				d.Number("Mode2") == 1,
				d.Number("Mode3") == 1,
				d.Number("Mode4") == 1,
				d.Number("Mode5") == 1,
				d.Number("Mode6") == 1,
				d.Number("Mode7") == 1,
			},

			XOffset: d.Number("Yoffset"),
			YOffset: d.Number("Xoffset"),
			Draw:    d.Number("Draw") == 1,

			SubClass: d.Number("SubClass"),

			OperateRange: d.Number("OperateRange"),
			Restore:      uint8(d.Number("Restore")) == 1,

			Parm: [8]int{
				d.Number("Parm0"),
				d.Number("Parm1"),
				d.Number("Parm2"),
				d.Number("Parm3"),
				d.Number("Parm4"),
				d.Number("Parm5"),
				d.Number("Parm6"),
				d.Number("Parm7"),
			},
			Act:      d.Number("Act"),
			Lockable: uint8(d.Number("Lockable")) == 1,
			Sync:     uint8(d.Number("Sync")) == 1,
			Flicker:  uint8(d.Number("Flicker")) == 1,
			Damage:   d.Number("Damage"),
			Overlay:  uint8(d.Number("Overlay")) == 1,

			Left:   d.Number("Left"),
			Top:    d.Number("Top"),
			Width:  d.Number("Width"),
			Height: d.Number("Height"),

			OperateFn:  d.Number("OperateFn"),
			PopulateFn: d.Number("PopulateFn"),
			InitFn:     d.Number("InitFn"),

			RestoreVirgins: uint8(d.Number("RestoreVirgins")) == 1,

			AutoMap: d.Number("AutoMap"),
		}

		records[i] = record
		i++
	}

	if d.Err != nil {
		return d.Err
	}

	r.Debugf("Loaded %d ObjectDetail records", len(records))

	r.Object.Details = records

	return nil
}
