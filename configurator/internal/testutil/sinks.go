package testutil

// Global variables for blackhole sinks
var (
	SinkUint16    uint16
	SinkSlice     []uint16
	SinkBytes     []byte
	SinkString    string
	SinkInterface any
	SinkErr       error
)

// KeepUint16 предотвращает оптимизацию сохранения переменной uint16.
func KeepUint16(v uint16) {
	SinkUint16 = v
}

// KeepSlice предотвращает оптимизацию сохранения среза uint16.
func KeepSlice(v []uint16) {
	SinkSlice = v
}

// KeepBytes предотвращает оптимизацию сохранения среза байтов.
func KeepBytes(v []byte) {
	SinkBytes = v
}

// KeepString предотвращает оптимизацию сохранения строки.
func KeepString(v string) {
	SinkString = v
}

// KeepInterface предотвращает оптимизацию сохранения любого интерфейса.
func KeepInterface(v any) {
	SinkInterface = v
}

// KeepError предотвращает оптимизацию сохранения ошибки.
func KeepError(err error) {
	SinkErr = err
}
