let currentRules = [];

function loadRules() {
    fetch('/api/rules')
        .then(response => response.json())
        .then(data => {
            currentRules = data;
            if (currentRules === null) {
                currentRules = [];
            }
            renderRulesTable();
        })
        .catch(error => {
            console.error('Ошибка загрузки правил:', error);
        });
}

function renderRulesTable() {
    const tbody = document.getElementById('rules-tbody');
    
    if (currentRules.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7">Нет правил</td></tr>';
        return;
    }

    tbody.innerHTML = currentRules.map(rule => `
        <tr>
            <td>${rule.name}</td>
            <td><span style="color: ${rule.action === 'allow' ? '#27ae60' : '#e74c3c'}">${rule.action === 'allow' ? 'Разрешить' : 'Блокировать'}</span></td>
            <td>${rule.protocol.toUpperCase()}</td>
            <td>${rule.sourceIP || 'Любой'}${rule.sourcePort ? ':' + rule.sourcePort : ''}</td>
            <td>${rule.destIP || 'Любой'}${rule.destPort ? ':' + rule.destPort : ''}</td>
            <td><span class="${rule.enabled ? 'status-enabled' : 'status-disabled'}">${rule.enabled ? 'Включено' : 'Отключено'}</span></td>
            <td>
                <button class="btn btn-warning" onclick="editRule('${rule.id}')">Изменить</button>
                <button class="btn btn-danger" onclick="deleteRule('${rule.id}')">Удалить</button>
            </td>
        </tr>
    `).join('');
}

function openModal(ruleId = null) {
    const modal = document.getElementById('rule-modal');
    const form = document.getElementById('rule-form');
    const title = document.getElementById('modal-title');
    
    form.reset();
    document.getElementById('rule-enabled').checked = true;
    
    if (ruleId) {
        const rule = currentRules.find(r => r.id === ruleId);
        if (rule) {
            title.textContent = 'Редактировать правило';
            document.getElementById('rule-id').value = rule.id;
            document.getElementById('rule-name').value = rule.name;
            document.getElementById('rule-action').value = rule.action;
            document.getElementById('rule-protocol').value = rule.protocol;
            document.getElementById('rule-source-ip').value = rule.sourceIP || '';
            document.getElementById('rule-dest-ip').value = rule.destIP || '';
            document.getElementById('rule-source-port').value = rule.sourcePort || '';
            document.getElementById('rule-dest-port').value = rule.destPort || '';
            document.getElementById('rule-enabled').checked = rule.enabled;
            document.getElementById('rule-description').value = rule.description || '';
        }
    } else {
        title.textContent = 'Новое правило';
        document.getElementById('rule-id').value = '';
    }
    
    modal.style.display = 'block';
}

function closeModal() {
    document.getElementById('rule-modal').style.display = 'none';
}

function editRule(ruleId) {
    openModal(ruleId);
}

function deleteRule(ruleId) {
    if (confirm('Вы уверены, что хотите удалить это правило?')) {
        fetch('/api/rules/' + ruleId, {
            method: 'DELETE'
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                loadRules();
            } else {
                alert('Ошибка удаления: ' + (data.error || 'Неизвестная ошибка'));
            }
        })
        .catch(error => {
            alert('Ошибка удаления: ' + error.message);
        });
    }
}

document.addEventListener('DOMContentLoaded', function() {
    document.getElementById('rule-form').addEventListener('submit', function(e) {
        e.preventDefault();
        
        const ruleId = document.getElementById('rule-id').value;
        const ruleData = {
            id: ruleId || 'rule_' + Date.now(),
            name: document.getElementById('rule-name').value,
            action: document.getElementById('rule-action').value,
            protocol: document.getElementById('rule-protocol').value,
            sourceIP: document.getElementById('rule-source-ip').value,
            destIP: document.getElementById('rule-dest-ip').value,
            sourcePort: parseInt(document.getElementById('rule-source-port').value) || 0,
            destPort: parseInt(document.getElementById('rule-dest-port').value) || 0,
            enabled: document.getElementById('rule-enabled').checked,
            description: document.getElementById('rule-description').value
        };
        
        const url = ruleId ? '/api/rules/' + ruleId : '/api/rules';
        const method = ruleId ? 'PUT' : 'POST';
        
        fetch(url, {
            method: method,
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(ruleData)
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                closeModal();
                loadRules();
            } else {
                alert('Ошибка сохранения: ' + (data.error || 'Неизвестная ошибка'));
            }
        })
        .catch(error => {
            alert('Ошибка сохранения: ' + error.message);
        });
    });

    // Закрытие модального окна по клику вне его
    window.onclick = function(event) {
        const modal = document.getElementById('rule-modal');
        if (event.target == modal) {
            closeModal();
        }
    };

    // Загружаем правила при загрузке страницы
    loadRules();
});
