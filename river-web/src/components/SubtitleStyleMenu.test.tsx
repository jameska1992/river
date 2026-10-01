import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { SubtitleStyleMenu } from './SubtitleStyleMenu'
import { DEFAULT_STATE } from '../hooks/useSubtitleStyle'

// The component takes a CSS-Modules object; an empty map is fine for tests
// since we assert on roles/labels, not on the (scoped) class names.
const styles: Record<string, string> = {}

function setup(overrides: Partial<React.ComponentProps<typeof SubtitleStyleMenu>> = {}) {
  const props = {
    open: true,
    fontScale: DEFAULT_STATE.fontScale,
    color: DEFAULT_STATE.color,
    bgOpacity: DEFAULT_STATE.bgOpacity,
    onToggle: vi.fn(),
    onSetFontScale: vi.fn(),
    onSetColor: vi.fn(),
    onSetBgOpacity: vi.fn(),
    onReset: vi.fn(),
    styles,
    ...overrides,
  }
  render(<SubtitleStyleMenu {...props} />)
  return props
}

describe('SubtitleStyleMenu', () => {
  it('shows only the toggle button when closed', () => {
    setup({ open: false })
    expect(screen.getByRole('button', { name: 'Subtitle appearance' })).toBeTruthy()
    expect(screen.queryByText('Text size')).toBeNull()
  })

  it('toggles open state via the trigger', async () => {
    const user = userEvent.setup()
    const props = setup({ open: false })
    await user.click(screen.getByRole('button', { name: 'Subtitle appearance' }))
    expect(props.onToggle).toHaveBeenCalledTimes(1)
  })

  it('applies a font-size preset', async () => {
    const user = userEvent.setup()
    const props = setup()
    await user.click(screen.getByRole('button', { name: 'Large' }))
    expect(props.onSetFontScale).toHaveBeenCalledWith(1.5)
  })

  it('marks the active colour as pressed and picks another', async () => {
    const user = userEvent.setup()
    const props = setup()
    expect(screen.getByRole('button', { name: 'White' }).getAttribute('aria-pressed')).toBe('true')
    await user.click(screen.getByRole('button', { name: 'Yellow' }))
    expect(props.onSetColor).toHaveBeenCalledWith('#ffeb3b')
  })

  it('reflects the current background opacity on the slider', () => {
    setup()
    const slider = screen.getByLabelText('Subtitle background opacity') as HTMLInputElement
    expect(slider.value).toBe(String(DEFAULT_STATE.bgOpacity))
  })

  it('fires reset', async () => {
    const user = userEvent.setup()
    const props = setup()
    await user.click(screen.getByRole('button', { name: /Reset to defaults/ }))
    expect(props.onReset).toHaveBeenCalledTimes(1)
  })
})
