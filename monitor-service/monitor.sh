#!/bin/bash

# Мониторинг сетевой активности для Orange Pi
# Этот скрипт собирает статистику пакетов для dashboard

STATS_FILE="/opt/gex/packet_stats.json"
LOG_FILE="/opt/gex/log.txt"

# Получаем статистику из iptables
get_iptables_stats() {
    local total_packets=0
    local total_bytes=0
    
    # Считаем пакеты из FORWARD chain
    while read -r line; do
        if [[ $line =~ NFQUEUE ]]; then
            packets=$(echo "$line" | awk '{print $1}')
            bytes=$(echo "$line" | awk '{print $2}')
            total_packets=$((total_packets + packets))
            total_bytes=$((total_bytes + bytes))
        fi
    done < <(iptables -L FORWARD -n -v 2>/dev/null || echo "")
    
    echo "$total_packets:$total_bytes"
}

# Получаем статистику сетевого интерфейса
get_interface_stats() {
    local interface=${1:-eth0}
    
    if [[ -f "/sys/class/net/$interface/statistics/rx_packets" ]]; then
        rx_packets=$(cat "/sys/class/net/$interface/statistics/rx_packets")
        tx_packets=$(cat "/sys/class/net/$interface/statistics/tx_packets")
        rx_bytes=$(cat "/sys/class/net/$interface/statistics/rx_bytes")
        tx_bytes=$(cat "/sys/class/net/$interface/statistics/tx_bytes")
        
        echo "$rx_packets:$tx_packets:$rx_bytes:$tx_bytes"
    else
        echo "0:0:0:0"
    fi
}

# Имитация статистики блокированных пакетов
# В реальной системе здесь должно быть чтение из NFQ логов
get_blocked_stats() {
    # Читаем логи и считаем заблокированные пакеты
    local blocked=0
    if [[ -f "$LOG_FILE" ]]; then
        blocked=$(grep -c "BLOCKED\|DROP" "$LOG_FILE" 2>/dev/null || echo "0")
    fi
    echo "$blocked"
}

# Основная функция
main() {
    # Получаем статистику
    iptables_stats=$(get_iptables_stats)
    interface_stats=$(get_interface_stats "eth0")
    blocked_count=$(get_blocked_stats)
    
    # Разбираем статистику
    IFS=':' read -r _iptables_packets _iptables_bytes <<< "$iptables_stats"
    IFS=':' read -r rx_packets tx_packets rx_bytes tx_bytes <<< "$interface_stats"
    
    total_packets=$((rx_packets + tx_packets))
    passed_packets=$((total_packets - blocked_count))
    
    # Создаём JSON с статистикой
    cat > "$STATS_FILE" << EOF
{
    "total": $total_packets,
    "passed": $passed_packets,
    "blocked": $blocked_count,
    "timestamp": $(date +%s),
    "interface": {
        "rx_packets": $rx_packets,
        "tx_packets": $tx_packets,
        "rx_bytes": $rx_bytes,
        "tx_bytes": $tx_bytes
    }
}
EOF

    # Логируем статистику
    echo "$(date): Total: $total_packets, Passed: $passed_packets, Blocked: $blocked_count" >> "$LOG_FILE"
}

# Запускаем если скрипт вызван напрямую
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
