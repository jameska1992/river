import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { DetailHero } from './Detail'

// The bottom fade is the only element with a linear-gradient background.
const fade = (c: HTMLElement) =>
  Array.from(c.querySelectorAll('div')).find(d => d.style.background.includes('linear-gradient'))

describe('DetailHero backdrop fade', () => {
  it('fades a landscape backdrop into the background', () => {
    const { container } = render(<DetailHero title="Dune" image="/backdrop.jpg" landscape />)
    expect(fade(container)).toBeTruthy()
  })

  it('does not fade the portrait poster', () => {
    const { container } = render(<DetailHero title="Dune" image="/poster.jpg" />)
    expect(fade(container)).toBeUndefined()
  })

  it('does not render a fade when there is no image', () => {
    const { container } = render(<DetailHero title="Dune" landscape />)
    expect(fade(container)).toBeUndefined()
  })
})
