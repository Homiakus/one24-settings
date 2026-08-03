package api

import (
	"bytes"
	"encoding/json"
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"modbus-configurator/internal/testutil"
	"modbus-configurator/model"
	"modbus-configurator/ws"
)

// emptyFS создаёт пустую встроенную файловую систему для тестов.
//
//go:embed server.go
var testUIFS embed.FS

// newTestServer возвращает инициализированный сервер без подключения к реальному Modbus.
func newTestServer(apiKey string) *Server {
	hub := ws.NewHub()
	srvCfg := &ServerConfig{
		RateLimitRPS: 100,
		APIKey:       apiKey,
		MaxWSClients: 10,
	}
	srv, _ := New(nil, hub, testUIFS, srvCfg)
	return srv
}

// TestHealthzEndpoint проверяет статус здоровья сервера без Modbus подключения.
func TestHealthzEndpoint(t *testing.T) {
	srv := newTestServer("")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp, err := client.Get(ts.URL + "/healthz")
	testutil.AssertNil(t, err, "Выполнение GET /healthz")
	defer resp.Body.Close()
	testutil.AssertEqual(t, resp.StatusCode, http.StatusOK, "Код ответа /healthz должен быть 200 OK")

	var body map[string]bool
	err = json.NewDecoder(resp.Body).Decode(&body)
	testutil.AssertNil(t, err, "Декодирование JSON ответа /healthz")
	testutil.AssertTrue(t, body["ok"], "ok должно быть true")
	testutil.AssertFalse(t, body["modbus"], "modbus должно быть false (не подключён)")
}

// TestGetStepsAndPutStep проверяет считывание 11 шагов и обновление конкретного шага через REST API.
func TestGetStepsAndPutStep(t *testing.T) {
	srv := newTestServer("")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	// 1. GET /api/v1/settings/steps
	resp, err := client.Get(ts.URL + "/api/v1/settings/steps")
	testutil.AssertNil(t, err, "GET /api/v1/settings/steps")
	testutil.AssertEqual(t, resp.StatusCode, http.StatusOK, "Статус 200 OK")

	var apiResp model.APIResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResp)
	_ = resp.Body.Close()
	testutil.AssertNil(t, err, "Декодирование ответа API")
	testutil.AssertTrue(t, apiResp.OK, "APIResponse.OK должен быть true")

	dataMap, ok := apiResp.Data.(map[string]any)
	testutil.AssertTrue(t, ok, "Data должна быть объектом map[string]any")

	stepsBytes, _ := json.Marshal(dataMap["steps"])
	var steps []model.StepParams
	_ = json.Unmarshal(stepsBytes, &steps)
	testutil.AssertEqual(t, len(steps), 11, "Должна возвращаться таблица из 11 шагов")

	// 2. PUT /api/v1/settings/steps/1
	updatePayload := model.StepParams{
		ID:           1,
		Name:         "70% Ethanol Modified",
		ExposureTime: 90,
		FillVolume:   1800,
	}
	payloadBytes, _ := json.Marshal(updatePayload)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/settings/steps/1", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	putResp, err := client.Do(req)
	testutil.AssertNil(t, err, "PUT /api/v1/settings/steps/1")
	if putResp != nil {
		_ = putResp.Body.Close()
		testutil.AssertEqual(t, putResp.StatusCode, http.StatusOK, "Статус обновления шага 200 OK")
	}

	// 3. GET /api/v1/settings/steps/1 — проверяем сохранение в памяти
	getOneResp, err := client.Get(ts.URL + "/api/v1/settings/steps/1")
	testutil.AssertNil(t, err, "GET /api/v1/settings/steps/1")
	var getOneApiResp model.APIResponse
	_ = json.NewDecoder(getOneResp.Body).Decode(&getOneApiResp)
	_ = getOneResp.Body.Close()
	step1Bytes, _ := json.Marshal(getOneApiResp.Data)
	var step1 model.StepParams
	_ = json.Unmarshal(step1Bytes, &step1)

	testutil.AssertEqual(t, step1.ExposureTime, 90, "Экспозиция обновлена до 90")
	testutil.AssertEqual(t, step1.FillVolume, 1800, "Объём обновлён до 1800")
}

// TestGetValvesAndPutValve проверяет получение таблицы клапанов и обновление координаты через PUT.
func TestGetValvesAndPutValve(t *testing.T) {
	srv := newTestServer("")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	// 1. GET /api/v1/settings/valves
	resp, err := client.Get(ts.URL + "/api/v1/settings/valves")
	testutil.AssertNil(t, err, "GET /api/v1/settings/valves")
	testutil.AssertEqual(t, resp.StatusCode, http.StatusOK, "Статус 200 OK")

	var apiResp model.APIResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResp)
	_ = resp.Body.Close()
	testutil.AssertNil(t, err, "Декодирование ответа API")
	testutil.AssertTrue(t, apiResp.OK, "APIResponse.OK должен быть true")

	dataMap, ok := apiResp.Data.(map[string]any)
	testutil.AssertTrue(t, ok, "Data должна быть map[string]any")

	sel1Bytes, _ := json.Marshal(dataMap["selector1"])
	var sel1 []model.SelectorPosition
	_ = json.Unmarshal(sel1Bytes, &sel1)
	testutil.AssertEqual(t, len(sel1), 15, "Селектор 1 имеет 15 позиций (0..14)")
	testutil.AssertEqual(t, sel1[2].Coord, 1371, "Начальная координата отв 2 = 1371")

	// 2. PUT /api/v1/settings/valves/1/2
	updatePayload := map[string]int{"coord": 1500}
	payloadBytes, _ := json.Marshal(updatePayload)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/settings/valves/1/2", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	putResp, err := client.Do(req)
	testutil.AssertNil(t, err, "PUT /api/v1/settings/valves/1/2")
	if putResp != nil {
		_ = putResp.Body.Close()
		testutil.AssertEqual(t, putResp.StatusCode, http.StatusOK, "Статус 200 OK")
	}

	// 3. Проверяем сохранение
	getValvesResp, err := client.Get(ts.URL + "/api/v1/settings/valves")
	testutil.AssertNil(t, err, "GET /api/v1/settings/valves")
	var getValvesApiResp model.APIResponse
	_ = json.NewDecoder(getValvesResp.Body).Decode(&getValvesApiResp)
	_ = getValvesResp.Body.Close()
	dataMap2 := getValvesApiResp.Data.(map[string]any)
	sel1Bytes2, _ := json.Marshal(dataMap2["selector1"])
	var sel1After []model.SelectorPosition
	_ = json.Unmarshal(sel1Bytes2, &sel1After)

	testutil.AssertEqual(t, sel1After[2].Coord, 1500, "Координата отв 2 обновилась до 1500")
	testutil.AssertTrue(t, sel1After[2].Dirty, "Позиция отв 2 должна быть dirty")
}

// TestAPIKeyAuthMiddleware проверяет проверку заголовка X-API-Key при включенной авторизации.
func TestAPIKeyAuthMiddleware(t *testing.T) {
	srv := newTestServer("secret-api-token")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	// 1. Без ключа -> 401 Unauthorized
	resp, err := client.Get(ts.URL + "/api/v1/status")
	testutil.AssertNil(t, err, "GET без API ключа")
	if resp != nil {
		_ = resp.Body.Close()
		testutil.AssertEqual(t, resp.StatusCode, http.StatusUnauthorized, "Неверный ключ должен возвращать 401")
	}

	// 2. С правильным ключом -> 200 OK
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/status", nil)
	req.Header.Set("X-API-Key", "secret-api-token")
	okResp, err := client.Do(req)
	testutil.AssertNil(t, err, "GET с валидным API ключом")
	if okResp != nil {
		_ = okResp.Body.Close()
		testutil.AssertEqual(t, okResp.StatusCode, http.StatusOK, "Валидный ключ должен возвращать 200 OK")
	}
}

// TestCORSMiddlewareHeaders проверяет отдачу заголовков CORS при префлайт OPTIONS запросе.
func TestCORSMiddlewareHeaders(t *testing.T) {
	srv := newTestServer("")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	req, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/v1/status", nil)
	req.Header.Set("Origin", "http://localhost")
	resp, err := client.Do(req)
	testutil.AssertNil(t, err, "OPTIONS запрос")
	if resp != nil {
		_ = resp.Body.Close()
		testutil.AssertEqual(t, resp.StatusCode, http.StatusOK, "Preflight OPTIONS отдает 200 OK")
		testutil.AssertEqual(t, resp.Header.Get("Access-Control-Allow-Origin"), "http://localhost", "Проверка CORS origin")
	}
}

// BenchmarkAPIStatusEndpoint замеряет производительность выполнения запроса GET /api/v1/status.
func BenchmarkAPIStatusEndpoint(b *testing.B) {
	srv := newTestServer("")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := ts.Client()
	url := ts.URL + "/api/v1/status"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err != nil {
			b.Fatalf("Ошибка HTTP запроса в бенчмарке: %v", err)
		}
		_ = resp.Body.Close()
		testutil.KeepUint16(uint16(resp.StatusCode))
	}
}
