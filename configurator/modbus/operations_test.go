package modbus

import (
	"testing"

	"modbus-configurator/internal/testutil"
	"modbus-configurator/model"
)

// TestValidateStepParamsRanges проверяет границы валидации параметров шага (1..11, exposure 1..600, volume 1..6000).
func TestValidateStepParamsRanges(t *testing.T) {
	// 1. Валидный случай (номер 1, экспозиция 60с, объём 1500 мл*10)
	err := validateStepParams(1, 60, 1500)
	testutil.AssertNil(t, err, "Валидный шаг 1 не должен возвращать ошибку")

	// 2. Валидные граничные значения
	err = validateStepParams(11, 600, 6000)
	testutil.AssertNil(t, err, "Максимальные валидные параметры 11, 600, 6000")

	err = validateStepParams(1, 1, 1)
	testutil.AssertNil(t, err, "Минимальные валидные параметры 1, 1, 1")

	// 3. Невалидные номера шагов
	err = validateStepParams(0, 60, 1500)
	testutil.AssertNotNil(t, err, "Шаг 0 должен вызывать ошибку валидации")

	err = validateStepParams(12, 60, 1500)
	testutil.AssertNotNil(t, err, "Шаг 12 должен вызывать ошибку валидации")

	// 4. Невалидная экспозиция
	err = validateStepParams(1, 0, 1500)
	testutil.AssertNotNil(t, err, "Экспозиция 0 сек должна вызывать ошибку")

	err = validateStepParams(1, 601, 1500)
	testutil.AssertNotNil(t, err, "Экспозиция > 600 сек должна вызывать ошибку")

	// 5. Невалидный объём налива
	err = validateStepParams(1, 60, 0)
	testutil.AssertNotNil(t, err, "Объём 0 должен вызывать ошибку")

	err = validateStepParams(1, 60, 6001)
	testutil.AssertNotNil(t, err, "Объём > 6000 должен вызывать ошибку")
}

// TestStainStepNamesLengthVerifies11Steps проверяет соответствие имён шагов протокола 11 шагам Папаниколау.
func TestStainStepNamesLengthVerifies11Steps(t *testing.T) {
	testutil.AssertEqual(t, len(StainStepNames), 11, "Массив StainStepNames должен содержать ровно 11 элементов")
	testutil.AssertEqual(t, StainStepNames[0], "Спирт 96% (фиксация)", "Шаг 1: Спирт 96% (фиксация)")
	testutil.AssertEqual(t, StainStepNames[1], "Гематоксилин Харриса", "Шаг 2: Гематоксилин Харриса")
	testutil.AssertEqual(t, StainStepNames[10], "Спирт 96%", "Шаг 11: Спирт 96%")
}

// TestDefaultExposureAndVolumeArrays проверяет инициализацию массивов экспозиций и объёмов по умолчанию.
func TestDefaultExposureAndVolumeArrays(t *testing.T) {
	testutil.AssertEqual(t, len(DefaultExposureTimes), 11, "Массив экспозиций должен иметь длину 11")
	testutil.AssertEqual(t, len(DefaultFillVolumes), 11, "Массив объёмов должен иметь длину 11")

	for i := 0; i < 11; i++ {
		err := validateStepParams(i+1, DefaultExposureTimes[i], DefaultFillVolumes[i])
		testutil.AssertNil(t, err, "Стандартные параметры всех 11 шагов обязаны быть валидными")
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
