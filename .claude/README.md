# Claude Code Automation Setup Complete

## ✅ Установленные компоненты

### MCP Серверы
- ✅ **context7** — документация для Qt6, Gin, GORM, WireGuard, YooKassa
- ✅ **GitHub** — управление PR, issues, CI/CD

### Skills (`.claude/skills/`)
- ✅ **build-platform** — сборка для разных платформ (`/build-platform windows`)
- ✅ **translation-update** — обновление Qt переводов (`/translation-update`)

### Subagents (`.claude/agents/`)
- ✅ **security-reviewer** — аудит auth, платежей, VPN конфигов
- ✅ **qml-reviewer** — ревью QML на accessibility и UX

### Hooks (`.claude/settings.json`)
- ✅ **PreToolUse: block-env-edits** — защита .env файлов
- ✅ **PostToolUse: format-cpp** — автоформатирование C++

### Plugins
- ✅ **anthropic-agent-skills** — superpowers, feature-dev, code-review

---

## 🚀 Следующие шаги

### 1. Установите clang-format (для C++ форматирования)
```bash
# Windows
winget install LLVM.LLVM

# Linux
sudo apt install clang-format

# macOS
brew install clang-format
```

### 2. Авторизуйтесь в GitHub CLI (для GitHub MCP)
```bash
gh auth login
```

### 3. Перезагрузите плагины
```bash
/reload-plugins
```

---

## 📖 Использование

### Автоматические действия
- **C++ код** автоматически форматируется после Edit/Write
- **.env файлы** защищены от случайного редактирования
- **Subagents** вызываются при изменениях в соответствующих файлах

### Ручные команды
```bash
/build-platform windows    # Собрать для Windows
/build-platform android    # Собрать для Android
/translation-update        # Обновить переводы
```

### Работа с документацией
Просто спросите про любую библиотеку — context7 автоматически найдёт актуальную документацию:
- "Как использовать QML Loader?"
- "Gin middleware для JWT"
- "GORM preload syntax"

---

## 📝 Документация

Полная документация: `.claude/AUTOMATIONS.md`

**Дата установки**: 2026-04-28
