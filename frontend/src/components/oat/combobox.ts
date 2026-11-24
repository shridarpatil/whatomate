export interface ComboOption {
  value: string
  label: string
  hint?: string
}

export function filterOptions<T extends ComboOption>(options: T[], query: string): T[] {
  const q = query.trim().toLowerCase()
  if (!q) return options
  return options.filter(
    (o) => o.label.toLowerCase().includes(q) || (o.hint?.toLowerCase().includes(q) ?? false),
  )
}
