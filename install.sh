#!/bin/bash

set -e

INSTALL_DIR="/opt/gex"
SERVICE_USER="gex"
DASHBOARD_PORT="8080"

echo "Установка GEX Dashboard..."

if ! id "$SERVICE_USER" &>/dev/null; then
    echo "Создание пользователя $SERVICE_USER..."
    useradd -r -s /bin/false -d $INSTALL_DIR $SERVICE_USER
fi

echo "Создание директорий..."
mkdir -p $INSTALL_DIR/rules
chown -R $SERVICE_USER:$SERVICE_USER $INSTALL_DIR

echo "Копирование исполнимого файла..."
cp gex-dashboard $INSTALL_DIR/
chmod +x $INSTALL_DIR/gex-dashboard
chown $SERVICE_USER:$SERVICE_USER $INSTALL_DIR/gex-dashboard

echo "Копирование веб-интерфейса..."
cp -r static $INSTALL_DIR/
chown -R $SERVICE_USER:$SERVICE_USER $INSTALL_DIR/static

if [ ! -f "$INSTALL_DIR/config.json" ]; then
    echo "Создание конфигурационного файла..."
    cat > $INSTALL_DIR/config.json << EOF
{
    "interface": "lan0",    
    "nfq_log_file": "/root/nfq/log.txt",
    "nfq_config_file": "/root/nfq/config.json",
    "nfq_rules_dir": "/root/nfq/rules",
    "net_stats_file": "/tmp/nfq/nfq.json",
    "sys_stats_file": "/tmp/nfq/sys.json",
    "log_level": "info",
    "listen_port": "$DASHBOARD_PORT"
}
EOF
    chown $SERVICE_USER:$SERVICE_USER $INSTALL_DIR/config.json
fi


echo "Создание правил sudoers для перезапуска служб..."
cat > /etc/sudoers.d/gex-services << EOF
# Разрешить пользователю gex перезапускать службы GEX
$SERVICE_USER ALL=(ALL) NOPASSWD: /bin/systemctl restart gex-web
$SERVICE_USER ALL=(ALL) NOPASSWD: /bin/systemctl restart gex-nfq
EOF

# Установка правильных прав доступа для файла sudoers
chmod 0440 /etc/sudoers.d/gex-services

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

echo "Перезагрузка systemd и запуск сервисов..."
systemctl daemon-reload
systemctl enable gex-dashboard
systemctl start gex-dashboard

echo "Установка завершена!"
echo "Dashboard доступен по адресу: http://localhost:$DASHBOARD_PORT"
