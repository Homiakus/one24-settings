# ONEPAP.24 Modbus Configurator

Веб-конфигуратор контроллера ONEPAP.24 для настройки технологических шагов, координат селекторов, запуска сервисных программ и диагностики по Modbus RTU.

## Требования

- Windows 10/11;
- PowerShell 7 (`pwsh`);
- Go версии из `configurator/go.mod` для сборки из исходников;
- USB/UART-порт контроллера, по умолчанию `COM4`, 115200 baud, slave ID 1.

## Рекомендуемый запуск

Из корня репозитория:

```powershell
.\onepap.ps1 doctor
.\onepap.ps1 start
```

Менеджер при необходимости собирает EXE, запускает приложение в фоне, сохраняет PID и фактический URL в `.runtime/server.json`, открывает браузер и пишет журналы в `.runtime/`.

Если порт из `configurator.toml` занят или заблокирован Windows, сервер не завершается: он проверяет следующие 32 порта, затем запрашивает свободный порт у ОС и выводит фактический URL.

Для полностью автоматического выбора порта:

```powershell
.\onepap.ps1 start -Port 0
```

Для запрета автоматического fallback:

```powershell
.\onepap.ps1 start -Port 8083 -StrictPort
```

## Единый скрипт управления

| Команда | Назначение |
|---|---|
| `.\onepap.ps1 start` | собрать при необходимости, запустить и открыть UI |
| `.\onepap.ps1 stop` | остановить управляемый экземпляр |
| `.\onepap.ps1 restart` | перезапустить сервер |
| `.\onepap.ps1 status` | показать PID, URL, порт и HTTP-состояние |
| `.\onepap.ps1 open` | открыть текущий UI |
| `.\onepap.ps1 build` | собрать `artifacts/modbus-configurator.exe` |
| `.\onepap.ps1 test` | запустить Go-тесты |
| `.\onepap.ps1 check` | проверить форматирование, `go vet` и race tests |
| `.\onepap.ps1 doctor` | проверить Go, HTTP-порт, исключённые диапазоны Windows, COM-порты и runtime |
| `.\onepap.ps1 logs` | показать последние журналы запуска |
| `.\onepap.ps1 clean` | удалить сборки, runtime, журналы, профили и Go-кэши |
| `.\onepap.ps1 clean -Deep` | дополнительно удалить кэш Go-модулей |

Команды можно запускать и через `onepap.cmd`, например:

```cmd
onepap.cmd start
```

## Прямой запуск Go

```powershell
cd configurator
go run .
```

Параметры запуска:

```text
-config <path>       TOML-конфигурация
-host <address>      переопределение HTTP host
-port <number>       HTTP-порт; 0 — свободный системный порт
-strict-port         не использовать запасной порт
-runtime-file <path> записать PID, URL и фактический порт в JSON
```

Пример:

```powershell
go run . -port 0
```

## Конфигурация

Основной файл: `configurator/configurator.toml`.

```toml
[server]
host = "127.0.0.1"
port = 8080

[modbus]
port = "COM4"
baudrate = 115200
slave_id = 1
```

При недоступном Modbus-порту веб-интерфейс всё равно запускается. Подключение можно повторить из UI после выбора правильного COM-порта.

## Структура

```text
.
├── onepap.ps1                    # единое управление системой
├── onepap.cmd                    # CMD-обёртка для PowerShell 7
├── configurator/
│   ├── api/                      # REST API
│   ├── modbus/                   # Modbus RTU и операции контроллера
│   ├── model/                    # модели состояния и DTO
│   ├── ui/                       # встроенный веб-интерфейс
│   ├── ws/                       # WebSocket-события
│   ├── main.go                   # запуск сервера
│   └── server_runtime.go         # выбор порта и runtime-состояние
├── scripts/                      # расширенные аудит/benchmark/stress сценарии
├── docs/                         # UX-аудит и проектная документация
├── Протокол_Modbus.md            # карта регистров
└── ТЗ_Modbus_Configurator.md     # техническое задание
```

## Проверки

GitHub Actions выполняет:

- `gofmt` для изменённых Go-файлов;
- `go vet ./...`;
- `go test ./... -race -count=1`;
- синтаксическую и smoke-проверку `onepap.ps1` на Windows;
- сборку Windows EXE и публикацию артефакта.

## Безопасность оборудования

Команды движения, калибровки, запуска программ и записи Modbus-регистров необходимо проверять на стенде с доступным аварийным остановом. Не выполняйте сервисные операции при открытой рабочей зоне или неподтверждённых координатах селекторов.
