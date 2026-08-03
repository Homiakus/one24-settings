package testutil

import (
	"modbus-configurator/model"
)

// ValidStepParamsFixture создаёт 11 стандартных валидных шагов Папаниколау.
func ValidStepParamsFixture() []model.StepParams {
	names := []string{
		"70% Ethanol", "Hematoxylin", "Rinse Water", "70% Ethanol",
		"OG-6 Stain", "95% Ethanol", "EA-50 Stain", "95% Ethanol",
		"100% Ethanol 1", "100% Ethanol 2", "Xylene",
	}
	exposures := []int{60, 180, 30, 45, 120, 45, 150, 45, 60, 60, 120}
	volumes := []int{1500, 1500, 2000, 1500, 1500, 1500, 1500, 1500, 1500, 1500, 1500}

	steps := make([]model.StepParams, 11)
	for i := 0; i < 11; i++ {
		steps[i] = model.StepParams{
			ID:           i + 1,
			Name:         names[i],
			ExposureTime: exposures[i],
			FillVolume:   volumes[i],
		}
	}
	return steps
}

// InvalidStepParamsFixture возвращает набор с некорректными границами для негативных тестов.
func InvalidStepParamsFixture() []model.StepParams {
	return []model.StepParams{
		{ID: 0, Name: "Invalid ID Low", ExposureTime: 60, FillVolume: 1000},
		{ID: 12, Name: "Invalid ID High", ExposureTime: 60, FillVolume: 1000},
		{ID: 1, Name: "Invalid Exposure Low", ExposureTime: 0, FillVolume: 1000},
		{ID: 1, Name: "Invalid Exposure High", ExposureTime: 601, FillVolume: 1000},
		{ID: 1, Name: "Invalid Volume Low", ExposureTime: 60, FillVolume: 0},
		{ID: 1, Name: "Invalid Volume High", ExposureTime: 60, FillVolume: 6001},
	}
}

// DefaultDetectionFixture создаёт фикстуру параметров детекции реагента.
func DefaultDetectionFixture() model.DetectionParams {
	return model.DetectionParams{
		ReagentEmpty:      false,
		ReagentEmptyDelta: 50,
	}
}
