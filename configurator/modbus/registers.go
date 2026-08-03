package modbus

// ProgressFunc — callback прогресса: current, total, label.
type ProgressFunc func(current, total int, label string)

// ─── Адреса регистров ─────────────────────────────────────────────────────────

const (
	RegReadyStatus       = 1   // R     готовность (0=готов, 1=занят)
	RegStatusError       = 5   // R     код ошибки
	RegCmdTarget         = 9   // W     код команды
	RegSelector1Hole     = 19  // R/W   позиция первого селектора (1..14)
	RegSelector2Hole     = 20  // R/W   позиция второго селектора (1..14)
	RegSelectorTarget    = 19  // R/W   выбор клапана (1 или 2) + home
	RegSelectorHole      = 20  // R/W   выбор номера отверстия (0...14)
	RegSelectorCoord     = 21  // R/W   координата отверстия
	RegStatusDrain       = 22  // R     статус слива
	RegCurrentStep       = 37  // W     номер шага / liquid_code
	RegExposureTime      = 38  // R/W   время экспозиции (сек)
	RegLiquidFillVolume  = 39  // R/W   объём налива (мл×10)
	RegReagentEmpty      = 40  // R     флаг окончания реагента
	RegReagentEmptyDelta = 41  // R/W   порог детекции пустого реагента
)

// ─── Коды команд ──────────────────────────────────────────────────────────────

const (
	CmdCalibrateValve    = 90
	CmdCalibrateSel1     = 100
	CmdCalibrateSel2     = 110
	CmdZeroRotor         = 120
	CmdDrainIntermediate = 130
	CmdLoadMaterial      = 140
	CmdSedimentation     = 160
	CmdStainCycle        = 180
	CmdWashSystem        = 200
	CmdEmergencyStop     = 111 // пишется в рег. 1
)

// ─── Коды ошибок ──────────────────────────────────────────────────────────────

const (
	ErrSystem          = 0x01
	ErrRotor           = 0x04
	ErrPump            = 0x05
	ErrMultiplexer     = 0x06
	ErrSensor          = 0x07
	ErrValve           = 0x08
	ErrReagentLow      = 0x09
	ErrSamplePlacement = 0x0A
	ErrGlassPlacement  = 0x0B
	ErrTimeout         = 0x0C
	ErrWasteFull       = 0x0D
	ErrRotorSync       = 0x0E
	ErrPower           = 0x0F
)

// ErrorNames — человекочитаемые имена ошибок.
var ErrorNames = map[uint16]string{
	ErrSystem:          "ERR_SYSTEM",
	ErrRotor:           "ERR_ROTOR",
	ErrPump:            "ERR_PUMP",
	ErrMultiplexer:     "ERR_MULTIPLEXER",
	ErrSensor:          "ERR_SENSOR",
	ErrValve:           "ERR_VALVE",
	ErrReagentLow:      "ERR_REAGENT_LOW",
	ErrSamplePlacement: "ERR_SAMPLE_PLACEMENT",
	ErrGlassPlacement:  "ERR_GLASS_PLACEMENT",
	ErrTimeout:         "ERR_TIMEOUT",
	ErrWasteFull:       "ERR_WASTE_FULL",
	ErrRotorSync:       "ERR_ROTOR_SYNC",
	ErrPower:           "ERR_POWER",
}

// ─── Шаги окраски (имена реагентов) ──────────────────────────────────────────

// StainStepNames — имена реагентов для 11 шагов Папаниколау.
var StainStepNames = []string{
	"Спирт 96% (фиксация)",
	"Гематоксилин Харриса",
	"Вода дистиллированная",
	"Вода дистиллированная",
	"Спирт 87%",
	"OG-6 (оранжевый G)",
	"Спирт 96%",
	"EA-50",
	"Спирт 87%",
	"Спирт 87%",
	"Спирт 96%",
}

// DefaultExposureTimes — времена экспозиции по умолчанию (сек).
var DefaultExposureTimes = []int{10, 120, 10, 50, 10, 10, 10, 120, 10, 10, 10}

// DefaultFillVolumes — объём налива по умолчанию (мл×10).
var DefaultFillVolumes = []int{50, 30, 80, 80, 60, 40, 60, 40, 60, 60, 60}

// DefaultSelectorCoords — начальные координаты отверстий второго селектора по умолчанию (0..14).
var DefaultSelectorCoords = []int{
	0, 600, 1200, 2571, 3942, 5313, 6684, 8056, 9427, 10798, 12169, 13541, 14912, 16283, 17653,
}

// DefaultSelector1Coords — начальные координаты отверстий первого селектора по умолчанию (0..14).
var DefaultSelector1Coords = []int{
	0, 700, 1400, 2771, 4142, 5513, 6884, 8256, 9627, 10998, 12369, 13741, 15112, 16483, 17853,
}

// DefaultSelectorHoleNames — наименования позиций по умолчанию (0..14).
var DefaultSelectorHoleNames = []string{
	"Исходное положение (Home)",
	"Воздух",
	"EA-50",
	"Воздух",
	"Вода дистиллированная",
	"Воздух",
	"Спирт 87%",
	"Воздух",
	"Спирт 96%",
	"Воздух",
	"OG-6 (оранжевый G)",
	"Воздух",
	"Гематоксилин Харриса",
	"Воздух",
	"Хлорка",
}
