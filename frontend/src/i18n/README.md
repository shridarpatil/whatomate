# Internationalization (i18n)

This project uses [vue-i18n](https://vue-i18n.intlify.dev/) for internationalization and [Crowdin](https://crowdin.com/) for translation management.

## Adding New Languages

Languages are **auto-discovered** from the `locales/` folder. To add a new language:

1. Create a new JSON file: `locales/{language_code}.json` (e.g., `es.json` for Spanish)
2. Copy the structure from `en.json`
3. Translate all strings
4. The language will automatically appear in the language switcher

## Using Translations in Components

```vue
<script setup>
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <!-- Simple translation -->
  <p>{{ $t('common.save') }}</p>

  <!-- With interpolation -->
  <p>{{ $t('contacts.total', { count: 42 }) }}</p>

  <!-- In script -->
  <button @click="alert(t('common.success'))">Click</button>
</template>
```

## Translation Keys Structure

```
common.*      - Shared UI elements (buttons, labels)
auth.*        - Authentication related
nav.*         - Navigation menu items
chat.*        - Chat interface
contacts.*    - Contact management
settings.*    - Settings pages
users.*       - User management
analytics.*   - Analytics dashboard
templates.*   - WhatsApp templates
errors.*      - Error messages
validation.*  - Form validation messages
time.*        - Relative time strings
```

## For Translators (via Crowdin)

1. Go to [Crowdin Project URL]
2. Select your language
3. Translate strings using the web interface
4. Translations are automatically synced to this repo

## For Developers

### Adding New Strings

1. Add the string to `locales/en.json`
2. Use meaningful, hierarchical keys: `section.subsection.action`
3. Use interpolation for dynamic values: `"Hello, {name}!"`

### Changing Locale Programmatically

```typescript
import { setLocale, getLocale, SUPPORTED_LOCALES } from '@/i18n'

// Get current locale
const current = getLocale()

// Change locale
setLocale('es')

// List available locales
console.log(SUPPORTED_LOCALES)
```

## File Structure

```
src/i18n/
├── index.ts          # i18n configuration
├── README.md         # This file
└── locales/
    ├── en.json       # English (source)
    ├── es.json       # Spanish
    ├── fr.json       # French
    └── ...           # Other languages
```

## Crowdin Integration

### Setup (one-time)

1. Create a Crowdin project
2. Set environment variables:
   ```
   CROWDIN_PROJECT_ID=your_project_id
   CROWDIN_PERSONAL_TOKEN=your_token
   ```

### Sync Translations

```bash
# Upload source file
npx crowdin upload sources

# Download translations
npx crowdin download
```

### GitHub Integration

Crowdin can be configured to:
- Auto-sync when `en.json` changes
- Create PRs with new translations
- See: https://support.crowdin.com/github-integration/
