import type { SortOrder } from '../api'

export interface SortOption {
  // value is "<field>:<order>", e.g. "title:asc". Single string so a
  // native <select> can hold both axes — most users don't need the field
  // and direction as independent dropdowns and it doubles the chrome.
  value: string
  label: string
}

interface Props {
  options:  SortOption[]
  field:    string
  order:    SortOrder
  onChange: (field: string, order: SortOrder) => void
  className?: string
  selectClassName?: string
  labelClassName?: string
  labelText?: string
}

// SortControl is a native <select> for sort-by + direction. The parent
// owns the field/order state; this component only renders and emits
// changes. Caller is expected to persist if they want sticky behavior.
export function SortControl({
  options, field, order, onChange,
  className, selectClassName, labelClassName,
  labelText = 'Sort by',
}: Props) {
  const current = `${field}:${order}`
  return (
    <div className={className}>
      <span className={`label-sm ${labelClassName ?? ''}`}>{labelText}</span>
      <select
        className={selectClassName}
        value={current}
        onChange={e => {
          const [f, o] = e.target.value.split(':')
          onChange(f, (o === 'desc' ? 'desc' : 'asc'))
        }}
      >
        {options.map(o => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
    </div>
  )
}
