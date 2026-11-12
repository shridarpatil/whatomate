// @vitest-environment happy-dom
import { describe, it, expect } from 'vitest'
import { createApp, type Component } from 'vue'
import * as icons from './icons'
import { iconData } from './icon-data'

function render(component: Component, props: Record<string, unknown> = {}) {
  const el = document.createElement('div')
  createApp(component, props).mount(el)
  return el.querySelector('svg') as SVGElement
}

describe('icons', () => {
  it('renders paths', () => {
    const svg = render(icons.Trash2)
    expect(svg).not.toBeNull()
    expect(svg.children.length).toBe(5)
    expect(svg.getAttribute('width')).toBe('16')
  })

  it('size prop', () => {
    const svg = render(icons.Trash2, { size: 24 })
    expect(svg.getAttribute('width')).toBe('24')
    expect(svg.getAttribute('height')).toBe('24')
  })

  it('data-icon is the canonical kebab name', () => {
    expect(render(icons.Trash2).getAttribute('data-icon')).toBe('trash-2')
    expect(render(icons.UserPlus).getAttribute('data-icon')).toBe('user-plus')
    expect(render(icons.Edit3).getAttribute('data-icon')).toBe('pen-line')
    expect(render(icons.CalendarIcon).getAttribute('data-icon')).toBe('calendar')
    expect(render(icons.Grid3X3).getAttribute('data-icon')).toBe('grid-3x3')
  })

  it('every icon has data', () => {
    const entries = Object.entries(icons)
    expect(entries.length).toBeGreaterThan(0)
    for (const [name, component] of entries) {
      const key = (component as { name?: string }).name ?? ''
      expect(iconData[key], `${name} -> ${key}`).toBeDefined()
      expect(iconData[key].length, name).toBeGreaterThan(0)
    }
  })
})
