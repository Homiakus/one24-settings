# ONEPAP.24 — Modbus Configurator & Protocol Specification

> **Единственный источник истины** для Modbus RTU протокола и веб-конфигуратора управляющего контроллера автоматизированного цитологического процессора (24 стекла, окраска по Папаниколау).

---

## 📌 Описание проекта

Данный репозиторий содержит спецификацию протокола обмена Modbus RTU, а также полноценный веб-конфигуратор на **Go (Backend REST + WebSocket)** и **Vanilla HTML5/JS/CSS (Embedded Single-Page Application)** для настройки, калибровки и диагностики управляющего контроллера процессора Medtechnica / ESP32.

### 🌟 Ключевые возможности

- ⚙️ **Параметры окраски (11 шагов Папаниколау):** Настройка времени экспозиции (сек) и объёма налива реагентов (мл×10), установка дельты детекции пустого реагента.
- 🔀 **Настройка переключающих клапанов (Селекторы 1 и 2):** Отдельная страница для считывания и сохранения координат 15 позиций (0..14) для каждого клапана из Modbus-регистров 19, 20, 21.
- ▶ **Управление программами:** Запуск цикла окраски, промывки системы (4 шага), загрузки образцов, осаждения, проверки системы, аварийный останов и сброс PLC.
- ◉ **Диагностика и датчики:** Мониторинг температур MCU/NTC, веса HX711, состояния концевиков и датчиков уровня жидкостей в реальном времени по WebSocket.
- 🧪 **Progressive Quality & Performance Pipeline:** Автоматизированный 9-этапный конвейер тестирования (Unit, Integration, Race Detector, Vet, Benchmarks, Escape Analysis, Profiling, Stress Test).

---

## 🛠 Технический стек

- **Backend:** Go 1.22 (`net/http`, `gorilla/websocket`, `goburrow/modbus`)
- **Frontend:** Vanilla JS / HTML5 / CSS3 (Enterprise Dark Mode Design System)
- **Конфигурация:** TOML (`configurator.toml`)
- **Интерфейс связи:** Modbus RTU через UART/USB (115200 baud, 8N1, slave_id=1)

---

## 📁 Структура репозитория

```text
.
├── configurator/                  # Веб-конфигуратор (Go Backend + UI)
│   ├── api/                       # REST API контроллеры и middleware
│   ├── modbus/                    # Клиент Modbus RTU и высокоуровневые операции
│   ├── model/                     # Структуры данных и типы DTO
│   ├── ui/                        # Веб-интерфейс (index.html, app.js, style.css)
│   ├── ws/                        # WebSocket Hub рассылки телеметрии
│   ├── config.go                  # Загрузка TOML-конфигурации
│   └── main.go                    # Точка входа сервера
├── scripts/                       # Progressive Quality & Performance Pipeline
│   ├── pipeline.ps1               # Единый конвейер качества для PowerShell
│   └── pipeline.sh                # Единый конвейер качества для Bash
├── audit/                         # Отчёты аудита и профилирования
├── Протокол_Modbus.md             # Спецификация карты регистров Modbus
└── ТЗ_Modbus_Configurator.md      # Техническое задание проекта
```

---

## 📋 Карта Modbus-регистров (основные)

| Регистр | Название | R/W | Описание |
|---|---|---|---|
| `1` | `ready_status` | R | Статус готовности (0 = готов, 1 = занят) |
| `5` | `status_error` | R | Код ошибки контроллера |
| `9` | `cmd_target` | W | Код отправляемой команды (90–200) |
| `19` | `selector1_hole` / `selector_target` | R/W | Номер селектора (1 или 2) / позиция селектора 1 |
| `20` | `selector2_hole` / `selector_hole` | R/W | Номер отверстия (0...14) / позиция селектора 2 |
| `21` | `selector_coord` | R/W | Координата выбранного отверстия селектора |
| `37` | `current_step` | W | Номер активного шага цикла окраски (1..11) |
| `38` | `exposure_time` | R/W | Время экспозиции (сек) |
| `39` | `liquid_fill_volume` | R/W | Объём налива (мл×10) |
| `40` | `reagent_empty` | R | Флаг окончания реагента (0/1) |
| `41` | `reagent_empty_delta` | R/W | Порог детекции окончания реагента |

---

## 🚀 Быстрый запуск

### 1. Запуск конфигуратора

```bash
cd configurator
go run .
```

После запуска откройте браузер по адресу: **`http://localhost:8080`**

### 2. Сборка исполняемого файла

```bash
cd configurator
go build -o modbus-configurator.exe .
```

---

## 🧪 Запуск конвейера тестирования и аудита

В репозиторий встроен 9-этапный автоматизированный конвейер качества (Unit tests, Race detector, Benchmarks, Escape analysis, Profiling, Stress testing):

```powershell
# Полный запуск всех этапов конвейера
.\scripts\pipeline.ps1 -Mode all

# Запуск только аудита и проверок гонок (Race detector)
.\scripts\pipeline.ps1 -Mode audit

# Запуск только стресс-тестирования параллельности
.\scripts\pipeline.ps1 -Mode stress -StressCount 50
```

Для Unix/Linux систем:
```bash
./scripts/pipeline.sh all
```

---

## 📄 Лицензия

Проект предназначен для использования в составе программно-аппаратного комплекса **ONEPAP.24**. Все права защищены.
