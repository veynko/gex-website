let currentFiles = [];
let currentEditingFileName = null;
let isNewFile = false;
let codeMirrorEditor = null;

function loadRuleFiles() {
    fetch('/api/rules/files')
        .then(response => response.json())
        .then(data => {
            currentFiles = data.files || [];
            renderFilesList();
        })
        .catch(error => {
            console.error('Ошибка загрузки файлов:', error);
            document.getElementById('rules-list').innerHTML = '<p style="color: red;">Ошибка загрузки файлов правил</p>';
        });
}

function renderFilesList() {
    const listDiv = document.getElementById('rules-list');
    
    if (currentFiles.length === 0) {
        listDiv.innerHTML = '<p>Нет файлов правил</p>';
        return;
    }

    listDiv.innerHTML = currentFiles.map(filename => `
        <div class="rule-file">
            <span class="rule-file-name">${filename}.json</span>
            <div>
                <button class="btn btn-warning" onclick="editFile('${filename}')">Редактировать</button>
                <button class="btn btn-danger" onclick="deleteFile('${filename}')">Удалить</button>
            </div>
        </div>
    `).join('');
}

function createNewFile() {
    isNewFile = true;
    currentEditingFileName = null;
    
    document.getElementById('editor-title').textContent = 'Создание нового файла';
    document.getElementById('rule-filename').value = '';
    
    const baseTemplate = {
        "name": "Новое правило",
        "action": "allow", 
        "protocol": "tcp",
        "enabled": true,
        "description": ""
    };
    
    document.getElementById('filename-group').style.display = 'block';
    document.getElementById('json-editor-modal').style.display = 'block';
    
    setTimeout(() => {
        initCodeMirror(JSON.stringify(baseTemplate, null, 2));
    }, 100);
}

function editFile(filename) {
    isNewFile = false;
    currentEditingFileName = filename;
    
    document.getElementById('editor-title').textContent = 'Редактирование: ' + filename + '.json';
    document.getElementById('rule-filename').value = filename;
    document.getElementById('filename-group').style.display = 'none';
    
    document.getElementById('json-editor-modal').style.display = 'block';
    
    fetch('/api/rules/raw/' + filename)
        .then(response => response.text())
        .then(content => {
            setTimeout(() => {
                initCodeMirror(content);
            }, 100);
        })
        .catch(error => {
            alert('Ошибка загрузки файла: ' + error.message);
        });
}

function deleteFile(filename) {
    if (!confirm(`Вы уверены, что хотите удалить файл "${filename}.json"?`)) {
        return;
    }
    
    fetch('/api/rules/raw/' + filename, {
        method: 'DELETE'
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            loadRuleFiles();
        } else {
            alert('Ошибка удаления: ' + (data.error || 'Неизвестная ошибка'));
        }
    })
    .catch(error => {
        alert('Ошибка удаления: ' + error.message);
    });
}

function closeJsonEditor() {
    document.getElementById('json-editor-modal').style.display = 'none';
    currentEditingFileName = null;
    isNewFile = false;
    
    if (codeMirrorEditor) {
        codeMirrorEditor.toTextArea();
        codeMirrorEditor = null;
    }
}

function initCodeMirror(initialValue) {
    if (codeMirrorEditor) {
        codeMirrorEditor.toTextArea();
        codeMirrorEditor = null;
    }
    
    const textarea = document.getElementById('rule-json-editor');
    textarea.value = initialValue || '{}';
    
    codeMirrorEditor = CodeMirror.fromTextArea(textarea, {
        mode: { name: "javascript", json: true },
        theme: "material",
        lineNumbers: true,
        autoCloseBrackets: true,
        matchBrackets: true,
        indentUnit: 2,
        tabSize: 2,
        foldGutter: true,
        gutters: ["CodeMirror-linenumbers", "CodeMirror-foldgutter"],
        extraKeys: {
            "Ctrl-Q": function(cm) { cm.foldCode(cm.getCursor()); },
            "Ctrl-Space": "autocomplete"
        }
    });
    
    try {
        const parsed = JSON.parse(initialValue || '{}');
        const formatted = JSON.stringify(parsed, null, 2);
        codeMirrorEditor.setValue(formatted);
    } catch (e) {
        codeMirrorEditor.setValue(initialValue || '{}');
    }
}

function saveCurrentRule() {
    const jsonText = codeMirrorEditor ? codeMirrorEditor.getValue() : document.getElementById('rule-json-editor').value;
    
    let isValidJson = true;
    try {
        if (jsonText.trim() !== '') {
            JSON.parse(jsonText);
        }
    } catch (error) {
        isValidJson = false;
        if (!confirm('Содержимое не является валидным JSON:\n' + error.message + '\n\nВы хотите сохранить файл как есть?')) {
            return;
        }
    }
    
    let filename;
    if (isNewFile) {
        filename = document.getElementById('rule-filename').value.trim();
        if (!filename) {
            alert('Введите имя файла');
            return;
        }
        filename = filename.replace(/\.json$/, '');
    } else {
        filename = currentEditingFileName;
    }
    
    const url = '/api/rules/raw/' + filename;
    const method = isNewFile ? 'POST' : 'PUT';
    
    fetch(url, {
        method: method,
        headers: {
            'Content-Type': 'text/plain'
        },
        body: jsonText
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            closeJsonEditor();
            loadRuleFiles();
            if (!isValidJson) {
                alert('Файл сохранен, но содержит невалидный JSON');
            }
        } else {
            alert('Ошибка сохранения: ' + (data.error || 'Неизвестная ошибка'));
        }
    })
    .catch(error => {
        alert('Ошибка сохранения: ' + error.message);
    });
}

function createNewRule() {
    createNewFile();
}

document.addEventListener('DOMContentLoaded', function() {
    window.onclick = function(event) {
        const modal = document.getElementById('json-editor-modal');
        if (event.target == modal) {
            closeJsonEditor();
        }
    };

    loadRuleFiles();
});
