let configEditor;

function showMessage(message, type = 'success') {
    const container = document.getElementById('message-container');
    container.innerHTML = '<div class="message ' + type + '">' + message + '</div>';
    setTimeout(() => {
        container.innerHTML = '';
    }, 5000);
}

function initCodeMirror() {
    configEditor = CodeMirror.fromTextArea(document.getElementById('config-content'), {
        mode: 'application/json',
        theme: 'material',
        lineNumbers: true,
        autoCloseBrackets: true,
        foldGutter: true,
        gutters: ["CodeMirror-linenumbers", "CodeMirror-foldgutter"],
        tabSize: 2,
        indentUnit: 2,
        lineWrapping: true
    });
    
    configEditor.setSize('100%', '400px');
}

function loadConfig() {
    fetch('/api/config')
        .then(response => {
            if (!response.ok) {
                throw new Error('HTTP ' + response.status);
            }
            return response.text();
        })
        .then(data => {
            try {
                const parsed = JSON.parse(data);
                configEditor.setValue(JSON.stringify(parsed, null, 2));
            } catch (e) {
                configEditor.setValue(data);
            }
        })
        .catch(error => {
            showMessage('Ошибка загрузки конфигурации: ' + error.message, 'error');
        });
}

function saveConfig() {
    const configContent = configEditor.getValue();
    
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
}

document.addEventListener('DOMContentLoaded', function() {
    initCodeMirror();
    
    document.getElementById('config-form').addEventListener('submit', function(e) {
        e.preventDefault();
        saveConfig();
    });

    setTimeout(() => {
        loadConfig();
    }, 100);
});
