# Claude Code Automations

Этот документ описывает установленные автоматизации для проекта FBLink VPN.

## Установленные компоненты

### 🔌 MCP Серверы

#### context7
**Назначение**: Актуальная документация для библиотек и фреймворков
**Использование**: Автоматически доступен при работе с Qt6, Gin, GORM, WireGuard, YooKassa API

#### GitHub
**Назначение**: Управление PR, issues, CI/CD
**Использование**: Команды `gh` доступны через Claude

### 🎯 Skills

#### `/build-platform <platform>`
**Файл**: `.claude/skills/build-platform/SKILL.md`
**Назначение**: Сборка для конкретной платформы
**Использование**: 
```bash
/build-platform windows
/build-platform linux
/build-platform android
```

#### `/translation-update`
**Файл**: `.claude/skills/translation-update/SKILL.md`
**Назначение**: Обновление Qt переводов после изменения строк
**Использование**: Запустить после добавления/изменения qsTr() строк

### 🤖 Subagents

#### security-reviewer
**Файл**: `.claude/agents/security-reviewer.md`
**Назначение**: Аудит безопасности для auth, платежей, VPN конфигов
**Триггеры**: Изменения в `handlers/auth.go`, `handlers/payment.go`, `configurators/`

#### qml-reviewer
**Файл**: `.claude/agents/qml-reviewer.md`
**Назначение**: Ревью QML кода на accessibility, UX паттерны, i18n
**Триггеры**: Изменения в `client/ui/qml/`
**Критично**: Проверяет использование `.slice(0, 10)` для дат из Go backend

### ⚡ Hooks

#### PreToolUse: block-env-edits
**Файл**: `.claude/settings.json`
**Назначение**: Блокировка случайного редактирования .env файлов с секретами
**Триггер**: Попытка Edit/Write на файлы `.env`

#### PostToolUse: format-cpp
**Файл**: `.claude/settings.json`
**Назначение**: Автоформатирование C++ кода после редактирования
**Триггер**: Edit/Write на `.cpp`, `.h`, `.hpp` файлы
**Требует**: `clang-format` установлен в системе

### 📦 Plugins

#### anthropic-agent-skills
**Включает**: superpowers (brainstorming, writing-plans, executing-plans, systematic-debugging, test-driven-development, verification-before-completion), feature-dev, code-review

## Следующие шаги

### Для работы C++ форматирования
Установите clang-format:
```bash
# Windows (через LLVM)
winget install LLVM.LLVM

# Linux
sudo apt install clang-format

# macOS
brew install clang-format
```

### Для работы GitHub MCP
Авторизуйтесь в GitHub CLI:
```bash
gh auth login
```

### Активация изменений
Перезапустите Claude Code или выполните:
```bash
/reload-plugins
```

## Использование

### Автоматическое
- **C++ форматирование**: Происходит автоматически после каждого Edit/Write
- **.env защита**: Блокирует редактирование автоматически
- **Subagents**: Claude вызовет при изменениях в соответствующих файлах

### Ручное
- **Skills**: Вызывайте через `/skill-name`
- **MCP серверы**: Используются Claude автоматически при необходимости

## Приоритет внедрения

✅ **Критично (уже установлено)**:
1. context7 MCP — документация
2. security-reviewer — аудит безопасности
3. block-env-edits hook — защита секретов

✅ **Полезно (уже установлено)**:
4. build-platform skill — упрощение сборки
5. GitHub MCP — git workflow
6. qml-reviewer — QML качество
7. format-cpp hook — консистентность кода

---

**Дата установки**: 2026-04-28
**Версия**: 1.0
