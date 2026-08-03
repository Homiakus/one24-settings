package ws

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"modbus-configurator/internal/testutil"
	"modbus-configurator/model"
)

// TestHubClientLimits проверяет ограничение максимального количества WebSocket подключений (maxClients).
func TestHubClientLimits(t *testing.T) {
	// Инициализируем хаб и задаём жесткий лимит 2 клиента
	hub := NewHub()
	hub.SetMaxClients(2)

	// Создаём тестовый HTTP-сервер для обработки WebSocket handshake
	ts := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Клиент 1: подсоединяемся успешно
	c1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	testutil.AssertNil(t, err, "Первое подключение должно пройти успешно")
	defer c1.Close()

	// Клиент 2: подсоединяемся успешно
	c2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	testutil.AssertNil(t, err, "Второе подключение должно пройти успешно")
	defer c2.Close()

	// Клиент 3: должно произойти отлонение из-за достижения maxClients (503 Service Unavailable)
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	testutil.AssertNotNil(t, err, "Третье подключение должно вернуть ошибку")
	if resp != nil {
		testutil.AssertEqual(t, resp.StatusCode, http.StatusServiceUnavailable, "Ожидается статус 503 при превышении лимита клиентов")
	}
}

// TestHubBroadcastAndTopics проверяет фильтрацию вещания по топикам подписки.
func TestHubBroadcastAndTopics(t *testing.T) {
	hub := NewHub()
	ts := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Подключаемся с фильтром по топику "sensors"
	u, _ := url.Parse(wsURL)
	q := u.Query()
	q.Set("topics", "sensors")
	u.RawQuery = q.Encode()

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	testutil.AssertNil(t, err, "Подключение с подпиской на топик sensors")
	defer conn.Close()

	// Отправляем событие топика "log", которое не должно дойти
	hub.Broadcast(model.WSEvent{Type: "log", Data: "Unwanted event"})

	// Отправляем целевое событие "sensors"
	hub.Broadcast(model.WSEvent{Type: "sensors", Data: "Sensor update payload"})

	// Устанавливаем таймаут на чтение для исключения зависания теста
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var received model.WSEvent
	err = conn.ReadJSON(&received)

	testutil.AssertNil(t, err, "Чтение пришедшего события sensors")
	testutil.AssertEqual(t, received.Type, "sensors", "Тип доставленного события должен совпадать")
	testutil.AssertEqual(t, received.Data.(string), "Sensor update payload", "Содержимое сообщения должно совпадать")
}

// TestHubConcurrentBroadcasts проверяет потокобезопасность метода Broadcast при высокой параллельной нагрузке.
func TestHubConcurrentBroadcasts(t *testing.T) {
	hub := NewHub()
	ts := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Создаём 5 подключений клиентов
	var clients []*websocket.Conn
	for i := 0; i < 5; i++ {
		c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		testutil.AssertNil(t, err, "Подключение параллельного клиента")
		clients = append(clients, c)
		defer c.Close()
	}

	// Запускаем 20 горутин, одновременно вещающих события
	var wg sync.WaitGroup
	workers := 20
	messagesPerWorker := 50

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for m := 0; m < messagesPerWorker; m++ {
				hub.Broadcast(model.WSEvent{
					Type: "status",
					Data: "Parallel message",
				})
			}
		}(w)
	}

	wg.Wait()
}

// BenchmarkHubBroadcast замеряет производительность рассылки сообщений через WebSocket Hub.
func BenchmarkHubBroadcast(b *testing.B) {
	hub := NewHub()
	ts := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		b.Fatalf("Ошибка подключения WebSocket: %v", err)
	}
	defer conn.Close()

	// Горутина-поглотитель отправляемых данных
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	event := model.WSEvent{
		Type: "status",
		Data: "Benchmark status payload text",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hub.Broadcast(event)
	}
}
