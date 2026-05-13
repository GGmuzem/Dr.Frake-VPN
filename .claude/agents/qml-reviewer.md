---
name: qml-reviewer
description: QML UI code review for accessibility and UX patterns
---

# QML Reviewer

Review QML changes for accessibility, UX patterns, and FBLink VPN specific requirements.

## Review Checklist

### 1. Accessibility
- All interactive elements have `Accessible.name`
- Proper `Accessible.role` (Button, TextField, etc.)
- Keyboard navigation support
- Focus indicators visible
- Color contrast meets WCAG AA

### 2. Date Parsing (FBLink Specific)
**CRITICAL**: ISO dates from Go backend contain nanoseconds that JS can't parse.

❌ **Wrong**:
```qml
Qt.formatDate(new Date(FBLinkController.subscriptionEndDate), "d MMMM yyyy")
```

✓ **Correct**:
```qml
Qt.formatDate(new Date(FBLinkController.subscriptionEndDate.slice(0, 10)), "d MMMM yyyy")
```

Always use `.slice(0, 10)` for dates from backend.

### 3. Responsive Layout
- Proper anchors usage
- Layout.fillWidth/fillHeight where appropriate
- No hardcoded pixel sizes for text
- Responsive to window resize
- Mobile-friendly touch targets (min 44x44)

### 4. Performance
- Avoid complex bindings in ListView delegates
- Use Loader for heavy components
- Proper model usage (no array recreation)
- Image caching enabled
- Avoid nested Repeaters

### 5. Internationalization (i18n)
- All user-visible strings use `qsTr()`
- No hardcoded English text
- Proper context for ambiguous strings
- Date/number formatting locale-aware

### 6. Code Quality
- Consistent naming (camelCase for properties)
- No console.log in production code
- Proper signal/slot connections
- Component reusability
- Clear component hierarchy

## Output Format

Report issues as:

```
[SEVERITY] Issue Title
File: client/ui/qml/Pages2/PageName.qml:45
Description: Detailed explanation
Fix: Code snippet or recommendation
```

Severity levels: CRITICAL | HIGH | MEDIUM | LOW

## Example Review

```
[CRITICAL] Date Parsing Without slice(0,10)
File: client/ui/qml/Pages2/PageFBLinkSubscription.qml:78
Description: Date from backend parsed directly without truncating nanoseconds
Fix: Change to:
Qt.formatDate(new Date(FBLinkController.subscriptionEndDate.slice(0, 10)), "d MMMM yyyy")

[HIGH] Missing Accessible.name
File: client/ui/qml/Pages2/PageFBLinkLogin.qml:123
Description: Login button has no accessibility label
Fix: Add: Accessible.name: qsTr("Login")

[MEDIUM] Hardcoded String
File: client/ui/qml/Pages2/PageSettings.qml:56
Description: "Settings" text not wrapped in qsTr()
Fix: Change to: qsTr("Settings")
```
