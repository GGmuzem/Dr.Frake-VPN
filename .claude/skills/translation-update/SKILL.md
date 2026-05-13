---
name: translation-update
description: Update Qt translation files after string changes
disable-model-invocation: true
---

# Translation Update

Updates Qt translation files using lupdate after QML/C++ string changes.

## Implementation

```bash
echo "Updating Qt translation files..."

cd client || exit 1

# Run lupdate to extract translatable strings
lupdate -recursive . -ts translations/amneziavpn_*.ts

if [ $? -eq 0 ]; then
  echo "✓ Translation files updated successfully"
  echo ""
  echo "Next steps:"
  echo "1. Open Qt Linguist: linguist translations/amneziavpn_ru.ts"
  echo "2. Translate new strings"
  echo "3. Release translations: lrelease translations/amneziavpn_*.ts"
else
  echo "✗ Failed to update translation files"
  exit 1
fi
```

## Usage

Run after adding or modifying user-visible strings in QML or C++ code:

```bash
/translation-update
```

## Prerequisites

- Qt Linguist tools installed (lupdate, lrelease)
- Run from project root directory
