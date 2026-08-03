# Public API Inventory — modbus-configurator

## 1. HTTP / REST API

| Метод | Путь | Назначение | Контракт / Ограничения |
|---|---|---|---|
| GET | `/healthz` | Health check сервера и Modbus | Возвращает JSON `{"ok":bool, "modbus":bool}` |
| GET | `/api/v1/status` | Статус подключения и системы | Возвращает текущее состояние `ConnectionState`, `ProgramState` |
| POST | `/api/v1/connect` | Подключение к COM-порту | Тело JSON: `Port`, `Baudrate`, `SlaveID` |
| POST | `/api/v1/disconnect` | Отключение от COM-порта | Отключает активный Modbus-клиент |
| GET | `/api/v1/ports` | Список доступных COM-портов | Возвращает массив имён COM-портов |
| GET | `/api/v1/settings/steps` | Список всех 11 шагов | Возвращает `[]StepParams` |
| GET | `/api/v1/settings/steps/{id}` | Получение шага по ID (1..11) | Возвращает `StepParams` |
| PUT | `/api/v1/settings/steps/{id}` | Обновление шага (1..11) | Валидация: exposure (1..600s), volume (1..6000) |
| POST | `/api/v1/settings/read-all` | Считывание всех шагов с контроллера | Опрашивает регистры 37, 38, 39, 40, 41 |
| POST | `/api/v1/settings/write-all` | Запись всех шагов на контроллер | Последовательно пишет в Modbus с эхо-проверкой |
| GET | `/api/v1/settings/detection` | Параметры детекции реагентов | Читает `reagent_empty` и `reagent_empty_delta` |
| PUT | `/api/v1/settings/detection` | Изменение порога детекции `delta` | Диапазон delta: 0..255 |
| POST | `/api/v1/programs/system-check` | Команда диагностики (90) | Запускает проверку систем контроллера |
| POST | `/api/v1/programs/load` | Команда загрузки стекла (91) | Выполняет загрузку кассеты |
| POST | `/api/v1/programs/sedimentation` | Осаждение (92) | Выполняет процесс осаждения |
| POST | `/api/v1/programs/stain/start` | Запуск программы окраски (100) | Запускает полный цикл 11 шагов |
| POST | `/api/v1/programs/stain/pause` | Пауза окраски (101) | Приостанавливает выполнение |
| POST | `/api/v1/programs/stain/resume` | Возобновление окраски (102) | Возобновляет окраску |
| POST | `/api/v1/programs/stain/stop` | Остановка окраски (103) | Отменяет программу |
| POST | `/api/v1/programs/wash/start` | Промывка (110) | Запускает промывочный цикл |
| POST | `/api/v1/programs/full/start` | Полный цикл (120) | Осаждение + Окраска |
| POST | `/api/v1/programs/emergency-stop` | Аварийный останов (200) | Немедленный останов движения |
| POST | `/api/v1/programs/reset-plc` | Сброс ошибки (99) | Сброс статуса ошибки контроллера |
| POST | `/api/v1/programs/stain/reagent-replaced` | Подтверждение замены реагента | Отправляет `true` в сигнальный канал `reagentResume` |
| POST | `/api/v1/programs/stain/reagent-cancel` | Отмена при отсутствии реагента | Отправляет `false` в сигнальный канал |
| GET | `/api/v1/testing/sensors` | Чтение сервисной карты датчиков | Возвращает `SensorSnapshot` |
| POST | `/api/v1/testing/selector` | Установка селектора (1..2, 1..14) | Переключает позицию клапана-селектора |
| POST | `/api/v1/testing/selector/calibrate` | Калибровка селектора | Выполняет поиск домашней позиции |
| GET | `/api/v1/diagnostics/log` | Считывание логов | Возвращает массив `LogEntry` |
| DELETE | `/api/v1/diagnostics/log` | Очистка логов | Очищает слайс логов |
| GET | `/api/v1/diagnostics/error` | Чтение последней ошибки | Возвращает информацию об ошибке |
| POST | `/api/v1/diagnostics/error/clear` | Сброс ошибки | Очищает статус |

## 2. WebSocket API

- Эндпоинт: `GET /ws/events?topics=status,progress,sensors,reagent,error,log,disconnect,connect`
- Поддерживаемые типы событий: `status`, `progress`, `sensors`, `reagent`, `error`, `log`, `connect`, `disconnect`.

## 3. Внутренний Go API (`modbus.Client`)

- `NewClient(cfg Config, retry RetryConfig) (*Client, error)`
- `(c *Client) ReadRegister(addr uint16) (uint16, error)`
- `(c *Client) ReadRegisters(addr, count uint16) ([]uint16, error)`
- `(c *Client) WriteRegister(addr, value uint16) error`
- `(c *Client) SafeWriteRegister(addr, value uint16) error`
- `(c *Client) WaitReady(timeout time.Duration) error`
- `(c *Client) SendCommand(ctx context.Context, cmd uint16, timeout time.Duration) error`
- `(c *Client) ReadStepParams(stepNum int) (model.StepParams, error)`
- `(c *Client) WriteStepParams(p model.StepParams) error`
- `(c *Client) ReadAllSettings(progress ProgressFunc) ([]model.StepParams, model.DetectionParams, error)`
- `(c *Client) WriteAllSettings(steps []model.StepParams, delta int, progress ProgressFunc) (int, error)`
- `(c *Client) SetSelector(selectorNum, hole int) error`
- `(c *Client) ReadSensorSnapshot() (model.SensorSnapshot, error)`
- `(c *Client) Close() error`
- `(c *Client) Recover() error`
