package testutil

import (
	"fmt"
	"math/rand"
	"time"

	"modbus-configurator/model"
)

// RandomStepParams генерирует шаг со случайными валидными параметрами.
func RandomStepParams(id int) model.StepParams {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return model.StepParams{
		ID:           id,
		Name:         fmt.Sprintf("Step-%d", id),
		ExposureTime: r.Intn(599) + 1,
		FillVolume:   r.Intn(5999) + 1,
	}
}

// GenerateRandomByteSlice создаёт срез случайных байт заданной длины.
func GenerateRandomByteSlice(length int) []byte {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	r.Read(b)
	return b
}
