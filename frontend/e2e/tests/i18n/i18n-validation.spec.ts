import { test, expect } from '@playwright/test'
import * as fs from 'fs'
import * as path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const LOCALES_DIR = path.resolve(__dirname, '../../../src/i18n/locales')

/**
 * Recursively collect all string values from a nested object,
 * returning their full dot-notation key paths.
 */
function collectStrings(obj: Record<string, unknown>, prefix = ''): Array<{ key: string; value: string }> {
  const entries: Array<{ key: string; value: string }> = []

  for (const [k, v] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'string') {
      entries.push({ key: fullKey, value: v })
    } else if (v && typeof v === 'object' && !Array.isArray(v)) {
      entries.push(...collectStrings(v as Record<string, unknown>, fullKey))
    }
  }

  return entries
}

/**
 * Check if a string contains unescaped double curly braces.
 *
 * vue-i18n uses {placeholder} for interpolation. Double curly braces {{ }}
 * cause a "Not allowed nest placeholder" compilation error in production.
 *
 * The escaped form uses vue-i18n's literal syntax: {'{{'}  and  {'}}'}
 * This function strips all literal blocks {' ... '} first, then checks
 * for remaining {{ or }} patterns.
 */
function findUnescapedDoubleCurlies(value: string): boolean {
  const stripped = value.replace(/\{'[^']*'\}/g, '')
  return stripped.includes('{{') || stripped.includes('}}')
}

test.describe('i18n Translation Validation', () => {
  test('locale files should not contain unescaped double curly braces', () => {
    const files = fs.readdirSync(LOCALES_DIR).filter(f => f.endsWith('.json'))
    expect(files.length).toBeGreaterThan(0)

    const violations: string[] = []

    for (const file of files) {
      const filePath = path.join(LOCALES_DIR, file)
      const content = JSON.parse(fs.readFileSync(filePath, 'utf-8'))
      const strings = collectStrings(content)

      for (const { key, value } of strings) {
        if (findUnescapedDoubleCurlies(value)) {
          violations.push(
            `${file} → "${key}": contains unescaped {{ or }}. ` +
            `Use {'{{'}  and {'}}'}  to escape. Value: "${value}"`
          )
        }
      }
    }

    expect(violations, `Found ${violations.length} unescaped double curly brace(s) in i18n files:\n${violations.join('\n')}`).toHaveLength(0)
  })

  test('locale files should be valid JSON', () => {
    const files = fs.readdirSync(LOCALES_DIR).filter(f => f.endsWith('.json'))

    for (const file of files) {
      const filePath = path.join(LOCALES_DIR, file)
      const raw = fs.readFileSync(filePath, 'utf-8')

      expect(() => JSON.parse(raw), `${file} is not valid JSON`).not.toThrow()
    }
  })

  test('locale files should not have empty translation values', () => {
    const files = fs.readdirSync(LOCALES_DIR).filter(f => f.endsWith('.json'))
    const empties: string[] = []

    for (const file of files) {
      const filePath = path.join(LOCALES_DIR, file)
      const content = JSON.parse(fs.readFileSync(filePath, 'utf-8'))
      const strings = collectStrings(content)

      for (const { key, value } of strings) {
        if (value.trim() === '') {
          empties.push(`${file} → "${key}" is empty`)
        }
      }
    }

    expect(empties, `Found ${empties.length} empty translation(s):\n${empties.join('\n')}`).toHaveLength(0)
  })
})
