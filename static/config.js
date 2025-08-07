function showMessage(message, type = 'success') {
    const container = document.getElementById('message-container');
    container.innerHTML = '<div class="message ' + type + '">' + message + '</div>';
    setTimeout(() => {
        container.innerHTML = '';
    }, 5000);
}

function loadConfig() {
    fetch('/api/config')
        .then(response => response.json())
        .then(data => {
            document.getElementById('config-content').value = JSON.stringify(data, null, 4);
        })
        .catch(error => {
            showMessage('Ошибка загрузки конфигурации: ' + error.message, 'error');
        });
}

document.addEventListener('DOMContentLoaded', function() {
    document.getElementById('config-form').addEventListener('submit', function(e) {
        e.preventDefault();
        
        const configContent = document.getElementById('config-content').value;
        
        try {
            // Проверяем валидность JSON
            JSON.parse(configContent);
            
            fetch('/api/config', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: configContent
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
    });

    // Загружаем конфигурацию при загрузке страницы
    loadConfig();
});
