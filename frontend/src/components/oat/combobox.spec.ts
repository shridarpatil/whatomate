import { describe, it, expect } from 'vitest'
import { filterOptions, type ComboOption } from './combobox'

const opts: ComboOption[] = [
  { value: '1', label: 'Alice' },
  { value: '2', label: 'Bob', hint: 'Natalie lead' },
  { value: '3', label: 'Carol' },
]

describe('filterOptions', () => {
  it('matches label and hint case-insensitively', () => {
    expect(filterOptions(opts, 'ali').map((o) => o.value)).toEqual(['1', '2'])
  })
  it('returns all for an empty query', () => {
    expect(filterOptions(opts, '')).toEqual(opts)
  })
  it('is case-insensitive', () => {
    expect(filterOptions(opts, 'ALI')).toEqual(filterOptions(opts, 'ali'))
  })
})
