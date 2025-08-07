#!/bin/bash

# Установочный скрипт для GEX Dashboard
# Для запуска на Orange Pi с 1GB RAM

set -e

INSTALL_DIR="/opt/gex"
SERVICE_USER="gex"
DASHBOARD_PORT="8080"

echo "Установка GEX Dashboard..."

# Создаём пользователя для службы
if ! id "$SERVICE_USER" &>/dev/null; then
    echo "Создание пользователя $SERVICE_USER..."
    useradd -r -s /bin/false -d $INSTALL_DIR $SERVICE_USER
fi

# Создаём директории
echo "Создание директорий..."
mkdir -p $INSTALL_DIR/{rules,logs}
chown -R $SERVICE_USER:$SERVICE_USER $INSTALL_DIR

# Копируем бинарный файл
echo "Копирование исполнимого файла..."
cp gex-dashboard $INSTALL_DIR/
chmod +x $INSTALL_DIR/gex-dashboard
chown $SERVICE_USER:$SERVICE_USER $INSTALL_DIR/gex-dashboard

# Копируем статические файлы веб-интерфейса
echo "Копирование веб-интерфейса..."
cp -r static $INSTALL_DIR/
chown -R $SERVICE_USER:$SERVICE_USER $INSTALL_DIR/static

# Копируем скрипт мониторинга
echo "Копирование скрипта мониторинга..."
cp monitor.sh $INSTALL_DIR/
chmod +x $INSTALL_DIR/monitor.sh
chown $SERVICE_USER:$SERVICE_USER $INSTALL_DIR/monitor.sh

# Создаём systemd сервис и таймер для мониторинга
echo "Создание systemd сервиса и таймера мониторинга..."
cp gex-monitor.service /etc/systemd/system/
cp gex-monitor.timer /etc/systemd/system/

# Создаём конфигурационный файл по умолчанию
if [ ! -f "$INSTALL_DIR/config.json" ]; then
    echo "Создание конфигурационного файла..."
    cat > $INSTALL_DIR/config.json << EOF
{
    "interface": "eth0",
    "mode": "bridge",
    "logLevel": "info",
    "bufferSize": 4096,
    "maxConnections": 1000,
    "timeout": 30
}
EOF
    chown $SERVICE_USER:$SERVICE_USER $INSTALL_DIR/config.json
fi

# Создаём лог файл
touch $INSTALL_DIR/log.txt
chown $SERVICE_USER:$SERVICE_USER $INSTALL_DIR/log.txt

# Создаём systemd сервис для dashboard
echo "Создание systemd сервиса для dashboard..."
cat > /etc/systemd/system/gex-dashboard.service << EOF
[Unit]
Description=GEX Traffic Filter Dashboard
After=network.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/gex-dashboard
Restart=always
RestartSec=5
Environment=GIN_MODE=release

# Ограничения ресурсов для Orange Pi с 1GB RAM
MemoryMax=128M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
EOF

# Создаём фиктивный NFQ сервис (замените на реальный)
echo "Создание systemd сервиса для NFQ..."
cat > /etc/systemd/system/gex-nfq.service << EOF
[Unit]
Description=GEX NFQ Traffic Filter
After=network.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
WorkingDirectory=$INSTALL_DIR
ExecStart=/bin/bash -c 'while true; do echo "NFQ Service running..." >> $INSTALL_DIR/log.txt; sleep 60; done'
Restart=always
RestartSec=5

# Ограничения ресурсов
MemoryMax=64M
CPUQuota=25%

[Install]
WantedBy=multi-user.target
EOF

# Перезагружаем systemd и запускаем сервисы
echo "Перезагрузка systemd и запуск сервисов..."
systemctl daemon-reload
systemctl enable gex-dashboard
systemctl enable gex-nfq
systemctl enable gex-monitor.timer
systemctl start gex-dashboard
systemctl start gex-nfq
systemctl start gex-monitor.timer

# Настройка iptables для NFQ (пример)
echo "Настройка iptables..."
iptables -t mangle -I FORWARD -j NFQUEUE --queue-num 0 2>/dev/null || true
iptables -t mangle -I INPUT -j NFQUEUE --queue-num 0 2>/dev/null || true

echo "Установка завершена!"
echo "Dashboard доступен по адресу: http://localhost:$DASHBOARD_PORT"
echo ""
echo "Команды управления:"
echo "  systemctl status gex-dashboard    # Статус dashboard"
echo "  systemctl status gex-nfq         # Статус NFQ"
echo "  systemctl restart gex-dashboard  # Перезапуск dashboard"
echo "  systemctl restart gex-nfq        # Перезапуск NFQ"
echo "  journalctl -u gex-dashboard -f   # Логи dashboard"
echo "  journalctl -u gex-nfq -f         # Логи NFQ"
