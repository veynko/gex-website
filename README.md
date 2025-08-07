# GEX Traffic Filter Dashboard

Веб-дашборд для мониторинга и управления системой фильтрации трафика на базе NFQ и iptables.

## Возможности

- **Мониторинг системы**: CPU, RAM, дисковое пространство, сетевая статистика
- **Просмотр логов**: в реальном времени из файла `$APP/log.txt`
- **Статистика пакетов**: количество обработанных, пропущенных и заблокированных пакетов
- **JSON редактор конфигурации**: управление файлом `$APP/config.json` с подсветкой синтаксиса
- **JSON редактор правил**: создание и редактирование правил фильтрации в `$APP/rules/*.json`
- **Управление службами**: перезапуск web и nfq служб
- **Статический веб-интерфейс**: независимая frontend часть

## Требования

- Go 1.21+
- Linux (протестировано на Orange Pi с 1GB RAM)
- iptables
- systemd

## Быстрая установка

1. Соберите приложение для ARM64:
```bash
chmod +x build.sh
./build.sh linux arm64
```

2. Скопируйте файлы на целевое устройство и установите:
```bash
tar -xzf build/gex-dashboard-linux_arm64.tar.gz
cd linux_arm64
sudo ./install.sh
```

3. Dashboard будет доступен по адресу: http://localhost:8080/static/index.html

## Структура файлов

```
/opt/gex/
├── gex-dashboard      # Исполняемый файл Go
├── static/            # Веб-интерфейс (HTML/CSS/JS)
│   ├── index.html     # Главная страница
│   ├── logs.html      # Страница логов
│   ├── config.html    # Редактор конфигурации
│   ├── rules.html     # Управление правилами
│   ├── style.css      # Стили
│   └── app.js         # JavaScript
├── config.json        # Конфигурация
├── log.txt           # Логи
├── packet_stats.json # Статистика пакетов
├── monitor.sh        # Скрипт мониторинга
└── rules/            # Правила фильтрации
    ├── rule_1.json
    └── rule_2.json
```

## Архитектура

- **Backend**: Go приложение предоставляет REST API и WebSocket
- **Frontend**: Статические HTML/CSS/JS файлы
- **Мониторинг**: Bash скрипт собирает статистику каждые 30 секунд
- **Данные**: JSON файлы для конфигурации и правил

## Конфигурация

Пример `config.json`:
```json
{
    "interface": "eth0",
    "mode": "bridge",
    "logLevel": "info",
    "bufferSize": 4096,
    "maxConnections": 1000,
    "timeout": 30
}
```

## Пример правила фильтрации

```json
{
    "id": "rule_1",
    "name": "Блокировка SSH из внешней сети",
    "action": "block",
    "protocol": "tcp",
    "sourceIP": "0.0.0.0/0",
    "destIP": "192.168.1.0/24",
    "sourcePort": 0,
    "destPort": 22,
    "enabled": true,
    "description": "Блокирует SSH доступ из внешней сети"
}
```

## Управление службами

```bash
# Статус служб
systemctl status gex-dashboard
systemctl status gex-nfq
systemctl status gex-monitor.timer

# Перезапуск
systemctl restart gex-dashboard
systemctl restart gex-nfq
systemctl restart gex-monitor.timer

# Логи
journalctl -u gex-dashboard -f
journalctl -u gex-nfq -f
journalctl -u gex-monitor.timer -f
```

## Разработка

```bash
# Установка зависимостей
go mod download

# Запуск в режиме разработки
go run *.go

# Сборка для локальной архитектуры
go build -o gex-dashboard .

# Сборка для различных архитектур
./build.sh linux amd64    # x86_64
./build.sh linux arm64    # ARM64 (Orange Pi)
./build.sh linux arm      # ARM32
```

## API Endpoints

- `GET /api/stats` - системная статистика
- `GET /api/logs` - последние логи
- `GET /api/config` - текущая конфигурация
- `POST /api/config` - обновить конфигурацию
- `GET /api/rules` - список правил
- `POST /api/rules` - создать правило
- `PUT /api/rules/{id}` - обновить правило
- `DELETE /api/rules/{id}` - удалить правило
- `GET /api/packet-stats` - статистика пакетов
- `POST /api/restart/{service}` - перезапустить службу
- `WS /ws/stats` - WebSocket для real-time статистики

## Веб-интерфейс

- **Адаптивный дизайн** для различных устройств
- **Real-time обновления** через WebSocket с fallback на HTTP
- **JSON редакторы** с подсветкой синтаксиса для конфигурации и правил
- **Валидация JSON** перед сохранением
- **Автообновление логов** каждые 10 секунд

## Ограничения ресурсов для Orange Pi

В systemd сервисах настроены ограничения:
- Dashboard: 128MB RAM, 50% CPU
- NFQ: 64MB RAM, 25% CPU
- Monitor: минимальное использование ресурсов

## Мониторинг

Автоматический сбор статистики каждые 30 секунд через systemd timer:
- Статистика сетевых интерфейсов
- Счётчики iptables
- Статистика пакетов (пропущенные/заблокированные)

## Безопасность

- Сервис работает от имени пользователя `gex`
- Нет встроенной авторизации (добавьте при необходимости)
- По умолчанию доступ только с localhost
- JSON валидация всех входящих данных

## Лицензия

MIT License
