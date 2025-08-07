const API_BASE = '';

function showMessage(message, type = 'success') {
    const container = document.getElementById('message-container');
    if (container) {
        container.innerHTML = `<div class="message ${type}">${message}</div>`;
        setTimeout(() => {
            container.innerHTML = '';
        }, 5000);
    } else {
        alert(message);
    }
}

let ws;
function initWebSocket() {
    if (document.getElementById('stats-grid')) {
        try {
            ws = new WebSocket(`ws://${window.location.host}/ws/stats`);
            
            ws.onmessage = function(event) {
                const data = JSON.parse(event.data);
                updateStats(data);
            };
            
            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
                setInterval(updateStatsHttp, 5000);
            };
            
            ws.onclose = function() {
                console.log('WebSocket closed, using HTTP polling');
                setInterval(updateStatsHttp, 5000);
            };
        } catch (e) {
            console.error('WebSocket not supported, using HTTP polling');
            setInterval(updateStatsHttp, 5000);
        }
    }
}

function updateStatsHttp() {
    fetch('/api/stats')
        .then(response => response.json())
        .then(data => updateStats(data))
        .catch(error => console.error('Stats error:', error));
}

function updateStats(stats) {
    const elements = {
        'cpu-value': `${stats.cpu.toFixed(1)}%`,
        'ram-value': `${stats.ram.toFixed(1)}%`,
        'disk-value': `${stats.disk.toFixed(1)}%`,
        'network-value': `${((stats.speed.download + stats.speed.upload) / 1024 / 1024).toFixed(2)} MB/s`
    };
    
    Object.entries(elements).forEach(([id, value]) => {
        const element = document.getElementById(id);
        if (element) element.textContent = value;
    });
    
    const progressBars = {
        'cpu-progress': stats.cpu,
        'ram-progress': stats.ram,
        'disk-progress': stats.disk
    };
    
    Object.entries(progressBars).forEach(([id, width]) => {
        const element = document.getElementById(id);
        if (element) element.style.width = `${width}%`;
    });
}

function updatePacketStats() {
    fetch('/api/packet-stats')
        .then(response => response.json())
        .then(data => {
            const elements = {
                'packets-total': data.total,
                'packets-passed': data.passed,
                'packets-blocked': data.blocked
            };
            
            Object.entries(elements).forEach(([id, value]) => {
                const element = document.getElementById(id);
                if (element) element.textContent = value;
            });
        })
        .catch(error => console.error('Packet stats error:', error));
}

function restartService(service) {
    if (confirm(`Вы уверены, что хотите перезапустить службу ${service}?`)) {
        fetch(`/api/restart/${service}`, {method: 'POST'})
            .then(response => response.json())
            .then(data => {
                alert(data.message || 'Служба перезапущена');
            })
            .catch(error => {
                alert('Ошибка: ' + error.message);
            });
    }
}

function refreshLogs() {
    fetch('/api/logs')
        .then(response => response.text())
        .then(data => {
            const container = document.getElementById('log-container');
            if (container) {
                container.textContent = data || 'Логи пусты';
                container.scrollTop = container.scrollHeight;
            }
        })
        .catch(error => {
            const container = document.getElementById('log-container');
            if (container) {
                container.textContent = 'Ошибка загрузки логов: ' + error.message;
            }
        });
}

function loadConfig() {
    fetch('/api/config')
        .then(response => response.json())
        .then(data => {
            const editor = document.getElementById('config-editor');
            if (editor) {
                editor.value = JSON.stringify(data, null, 4);
            }
        })
        .catch(error => {
            showMessage('Ошибка загрузки конфигурации: ' + error.message, 'error');
        });
}

function saveConfig() {
    const editor = document.getElementById('config-editor');
    if (!editor) return;
    
    try {
        const config = JSON.parse(editor.value);
        
        fetch('/api/config', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(config)
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                showMessage('Конфигурация успешно сохранена');
            } else {
                showMessage(data.error || 'Ошибка сохранения', 'error');
            }
        })
        .catch(error => {
            showMessage('Ошибка сохранения: ' + error.message, 'error');
        });
    } catch (e) {
        showMessage('Некорректный JSON: ' + e.message, 'error');
    }
}

function loadRules() {
    fetch('/api/rules')
        .then(response => response.json())
        .then(data => {
            const tbody = document.getElementById('rules-tbody');
            if (!tbody) return;
            
            if (data.length === 0) {
                tbody.innerHTML = '<tr><td colspan="7">Нет правил</td></tr>';
                return;
            }
            
            tbody.innerHTML = data.map(rule => `
                <tr>
                    <td>${rule.name}</td>
                    <td><span style="color: ${rule.action === 'allow' ? '#27ae60' : '#e74c3c'}">${rule.action === 'allow' ? 'Разрешить' : 'Блокировать'}</span></td>
                    <td>${rule.protocol.toUpperCase()}</td>
                    <td>${rule.sourceIP || 'Любой'}${rule.sourcePort ? ':' + rule.sourcePort : ''}</td>
                    <td>${rule.destIP || 'Любой'}${rule.destPort ? ':' + rule.destPort : ''}</td>
                    <td><span class="${rule.enabled ? 'status-enabled' : 'status-disabled'}">${rule.enabled ? 'Включено' : 'Отключено'}</span></td>
                    <td>
                        <button class="btn btn-primary" onclick="editRule('${rule.id}')">Изменить</button>
                        <button class="btn btn-restart" onclick="deleteRule('${rule.id}')">Удалить</button>
                    </td>
                </tr>
            `).join('');
        })
        .catch(error => {
            console.error('Ошибка загрузки правил:', error);
        });
}

function editRule(ruleId) {
    fetch(`/api/rules/${ruleId}`)
        .then(response => response.json())
        .then(rule => {
            const editor = document.getElementById('rule-editor');
            if (editor) {
                editor.value = JSON.stringify(rule, null, 4);
                editor.dataset.ruleId = ruleId;
            }
        })
        .catch(error => {
            showMessage('Ошибка загрузки правила: ' + error.message, 'error');
        });
}

function newRule() {
    const defaultRule = {
        id: 'rule_' + Date.now(),
        name: 'Новое правило',
        action: 'allow',
        protocol: 'tcp',
        sourceIP: '0.0.0.0/0',
        destIP: '',
        sourcePort: 0,
        destPort: 0,
        enabled: true,
        description: ''
    };
    
    const editor = document.getElementById('rule-editor');
    if (editor) {
        editor.value = JSON.stringify(defaultRule, null, 4);
        editor.dataset.ruleId = '';
    }
}

function saveRule() {
    const editor = document.getElementById('rule-editor');
    if (!editor) return;
    
    try {
        const rule = JSON.parse(editor.value);
        const ruleId = editor.dataset.ruleId;
        
        const url = ruleId ? `/api/rules/${ruleId}` : '/api/rules';
        const method = ruleId ? 'PUT' : 'POST';
        
        fetch(url, {
            method: method,
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(rule)
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                showMessage(ruleId ? 'Правило обновлено' : 'Правило создано');
                loadRules();
            } else {
                showMessage(data.error || 'Ошибка сохранения', 'error');
            }
        })
        .catch(error => {
            showMessage('Ошибка сохранения: ' + error.message, 'error');
        });
    } catch (e) {
        showMessage('Некорректный JSON: ' + e.message, 'error');
    }
}

function deleteRule(ruleId) {
    if (confirm('Вы уверены, что хотите удалить это правило?')) {
        fetch(`/api/rules/${ruleId}`, {
            method: 'DELETE'
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                showMessage('Правило удалено');
                loadRules();
            } else {
                showMessage(data.error || 'Ошибка удаления', 'error');
            }
        })
        .catch(error => {
            showMessage('Ошибка удаления: ' + error.message, 'error');
        });
    }
}

document.addEventListener('DOMContentLoaded', function() {
    const currentPage = window.location.pathname.split('/').pop() || 'index.html';
    
    switch (currentPage) {
        case 'index.html':
        case '':
            initWebSocket();
            updatePacketStats();
            setInterval(updatePacketStats, 5000);
            break;
            
        case 'logs.html':
            refreshLogs();
            setInterval(refreshLogs, 10000);
            break;
            
        case 'config.html':
            loadConfig();
            break;
            
        case 'rules.html':
            loadRules();
            newRule();
            break;
    }
});
