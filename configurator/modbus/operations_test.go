package modbus

import (
	"testing"

	"modbus-configurator/internal/testutil"
	"modbus-configurator/model"
)

// TestValidateStepParamsRanges проверяет границы валидации параметров шага (0..15, exposure 0..600, volume 0..6000).
func TestValidateStepParamsRanges(t *testing.T) {
	// 1. Валидный случай (шаг 0 — экспозиция образца, 0с, 0 мл*10)
	err := validateStepParams(0, 0, 0)
	testutil.AssertNil(t, err, "Валидный шаг 0 не должен возвращать ошибку")

	// 2. Валидный случай (номер 1, экспозиция 60с, объём 1500 мл*10)
	err = validateStepParams(1, 60, 1500)
	testutil.AssertNil(t, err, "Валидный шаг 1 не должен возвращать ошибку")

	// 3. Валидные граничные значения (шаг 15)
	err = validateStepParams(15, 600, 6000)
	testutil.AssertNil(t, err, "Максимальные валидные параметры 15, 600, 6000")

	// 4. Невалидные номера шагов
	err = validateStepParams(-1, 60, 1500)
	testutil.AssertNotNil(t, err, "Шаг -1 должен вызывать ошибку валидации")

	err = validateStepParams(16, 60, 1500)
	testutil.AssertNotNil(t, err, "Шаг 16 должен вызывать ошибку валидации")

	// 5. Невалидная экспозиция
	err = validateStepParams(1, 0, 1500)
	testutil.AssertNotNil(t, err, "Экспозиция 0 сек для шага 1 должна вызывать ошибку")

	err = validateStepParams(0, 601, 0)
	testutil.AssertNotNil(t, err, "Экспозиция > 600 сек должна вызывать ошибку")

	// 6. Невалидный объём налива
	err = validateStepParams(1, 60, 0)
	testutil.AssertNotNil(t, err, "Объём 0 для шага 1 должен вызывать ошибку")

	err = validateStepParams(1, 60, 6001)
	testutil.AssertNotNil(t, err, "Объём > 6000 должен вызывать ошибку")
}

// TestStainStepNamesLengthVerifies16Steps проверяет соответствие имён шагов протокола 16 шагам (0..15).
func TestStainStepNamesLengthVerifies16Steps(t *testing.T) {
	testutil.AssertEqual(t, len(StainStepNames), 16, "Массив StainStepNames должен содержать ровно 16 элементов")
	testutil.AssertEqual(t, StainStepNames[0], "Экспозиция образца", "Шаг 0: Экспозиция образца")
	testutil.AssertEqual(t, StainStepNames[1], "Спирт 96% (фиксация)", "Шаг 1: Спирт 96% (фиксация)")
	testutil.AssertEqual(t, StainStepNames[12], "Хлорка", "Шаг 12: Хлорка")
	testutil.AssertEqual(t, StainStepNames[15], "Вода", "Шаг 15: Вода")
}

// TestDefaultExposureAndVolumeArrays проверяет инициализацию массивов экспозиций и объёмов по умолчанию.
func TestDefaultExposureAndVolumeArrays(t *testing.T) {
	testutil.AssertEqual(t, len(DefaultExposureTimes), 16, "Массив экспозиций должен иметь длину 16")
	testutil.AssertEqual(t, len(DefaultFillVolumes), 16, "Массив объёмов должен иметь длину 16")

	for i := 0; i < 16; i++ {
		err := validateStepParams(i, DefaultExposureTimes[i], DefaultFillVolumes[i])
		testutil.AssertNil(t, err, "Стандартные параметры всех 16 шагов обязаны быть валидными")
	}
}

// TestValidateSelectorPositionRanges проверяет валидацию параметров позиций селектора.
func TestValidateSelectorPositionRanges(t *testing.T) {
	// Валидные
	err := validateSelectorPosition(1, 0, 0)
	testutil.AssertNil(t, err, "Селектор 1 отв 0 coord 0 валидно")

	err = validateSelectorPosition(2, 14, 17826)
	testutil.AssertNil(t, err, "Селектор 2 отв 14 coord 17826 валидно")

	// Невалидный селектор
	err = validateSelectorPosition(0, 1, 100)
	testutil.AssertNotNil(t, err, "Селектор 0 невалиден")

	err = validateSelectorPosition(3, 1, 100)
	testutil.AssertNotNil(t, err, "Селектор 3 невалиден")

	// Невалидное отверстие
	err = validateSelectorPosition(1, -1, 100)
	testutil.AssertNotNil(t, err, "Отверстие -1 невалидно")

	err = validateSelectorPosition(1, 15, 100)
	testutil.AssertNotNil(t, err, "Отверстие 15 невалидно")

	// Невалидная координата
	err = validateSelectorPosition(1, 1, -1)
	testutil.AssertNotNil(t, err, "Координата -1 невалидна")

	err = validateSelectorPosition(1, 1, 65536)
	testutil.AssertNotNil(t, err, "Координата 65536 невалидна")
}

// TestDefaultSelectorArrays проверяет длины и значения массивов селектора по умолчанию.
func TestDefaultSelectorArrays(t *testing.T) {
	testutil.AssertEqual(t, len(DefaultSelectorCoords), 15, "DefaultSelectorCoords должен иметь 15 элементов")
	testutil.AssertEqual(t, len(DefaultSelectorHoleNames), 15, "DefaultSelectorHoleNames должен иметь 15 элементов")

	testutil.AssertEqual(t, DefaultSelectorCoords[0], 0, "Координата отв 0 = 0")
	testutil.AssertEqual(t, DefaultSelectorCoords[2], 1200, "Координата отв 2 = 1200")
	testutil.AssertEqual(t, DefaultSelectorCoords[14], 17653, "Координата отв 14 = 17653")

	testutil.AssertEqual(t, DefaultSelectorHoleNames[2], "EA-50", "Имя отв 2: EA-50")
	testutil.AssertEqual(t, DefaultSelectorHoleNames[4], "Вода дистиллированная", "Имя отв 4: Вода дистиллированная")
}

// BenchmarkValidateStepParams замеряет скорость работы встроенного валидатора шагов.
func BenchmarkValidateStepParams(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := validateStepParams(5, 120, 1500)
		testutil.KeepError(err)
	}
}

// BenchmarkStepParamsStructAllocations замеряет память и время создания структуры StepParams.
func BenchmarkStepParamsStructAllocations(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sp := model.StepParams{
			ID:           5,
			Name:         "Hematoxylin",
			ExposureTime: 180,
			FillVolume:   1500,
		}
		testutil.KeepInterface(sp)
	}
}
