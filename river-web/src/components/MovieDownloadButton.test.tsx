import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { MovieDownloadButton } from './MovieDownloadButton'

const TRANSCODED = '/api/movies/m1/download?token=t'
const ORIGINAL = '/api/movies/m1/download?token=t&variant=source'

describe('MovieDownloadButton', () => {
  it('renders nothing when neither variant is available', () => {
    const { container } = render(<MovieDownloadButton />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders a single download link when only the transcoded file exists', () => {
    render(<MovieDownloadButton transcodedUrl={TRANSCODED} />)
    const link = screen.getByRole('link', { name: 'Download movie' })
    expect(link).toHaveAttribute('href', TRANSCODED)
    // no dropdown trigger
    expect(screen.queryByRole('button', { name: 'Download options' })).toBeNull()
  })

  it('renders a single original-download link when only the source exists', () => {
    render(<MovieDownloadButton originalUrl={ORIGINAL} />)
    const link = screen.getByRole('link', { name: 'Download original movie file' })
    expect(link).toHaveAttribute('href', ORIGINAL)
  })

  it('renders a dropdown with both options when both variants exist', async () => {
    const user = userEvent.setup()
    render(<MovieDownloadButton transcodedUrl={TRANSCODED} originalUrl={ORIGINAL} />)

    // Collapsed: a trigger button, no links yet.
    const trigger = screen.getByRole('button', { name: 'Download options' })
    expect(screen.queryByRole('menuitem')).toBeNull()

    await user.click(trigger)

    const menu = screen.getByRole('menu', { name: 'Download options' })
    const transcoded = within(menu).getByRole('menuitem', { name: /^Download$/ })
    const original = within(menu).getByRole('menuitem', { name: /Download original/ })
    expect(transcoded).toHaveAttribute('href', TRANSCODED)
    expect(original).toHaveAttribute('href', ORIGINAL)
  })

  it('closes the menu after choosing an option', async () => {
    const user = userEvent.setup()
    render(<MovieDownloadButton transcodedUrl={TRANSCODED} originalUrl={ORIGINAL} />)

    await user.click(screen.getByRole('button', { name: 'Download options' }))
    await user.click(screen.getByRole('menuitem', { name: /^Download$/ }))
    expect(screen.queryByRole('menu')).toBeNull()
  })
})
