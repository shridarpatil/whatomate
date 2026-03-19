export interface Column<_T = unknown> {
  key: string
  label: string
  width?: string
  align?: 'left' | 'center' | 'right'
  sortable?: boolean
  sortKey?: string // Custom sort key if different from display key
}
