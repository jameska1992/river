import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { activeCueText, parseVTT, parseVTTTime } from './vtt'
import { SubtitleOverlay } from '../react/SubtitleOverlay'

const VTT = `WEBVTT

1
00:00:01.000 --> 00:00:03.500 align:middle
<i>Hello</i> there

2
00:01:00.000 --> 00:01:02.000
Second line
continues

3
00:02:00.000 --> 00:02:01.000
m 0 0 l 10 10
`

describe('vtt', () => {
  it('parses timestamps', () => {
    expect(parseVTTTime('00:01:02.500')).toBe(62.5)
    expect(parseVTTTime('01:02.5')).toBe(62.5)
  })

  it('parses cues, strips tags, skips drawing cues', () => {
    expect(parseVTT(VTT)).toEqual([
      { start: 1, end: 3.5, text: 'Hello there' },
      { start: 60, end: 62, text: 'Second line\ncontinues' },
    ])
  })

  it('finds the active cue with an offset', () => {
    const cues = parseVTT(VTT)
    expect(activeCueText(cues, 2)).toBe('Hello there')
    expect(activeCueText(cues, 5)).toBe('')
    expect(activeCueText(cues, 5, 3)).toBe('Hello there')
  })
})

describe('<SubtitleOverlay>', () => {
  afterEach(() => { vi.unstubAllGlobals() })

  it('fetches the VTT and shows the cue for the position', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(VTT)))
    const { rerender } = render(<SubtitleOverlay url="/api/subtitles/s/stream" position={2} />)
    expect(await screen.findByText('Hello there')).toBeInTheDocument()
    rerender(<SubtitleOverlay url="/api/subtitles/s/stream" position={10} />)
    expect(screen.queryByText('Hello there')).not.toBeInTheDocument()
  })

  it('renders nothing when off or when the fetch fails', async () => {
    const fetchFn = vi.fn(async () => { throw new Error('x') })
    vi.stubGlobal('fetch', fetchFn)
    const { container, rerender } = render(<SubtitleOverlay url={null} position={2} />)
    expect(container).toBeEmptyDOMElement()
    rerender(<SubtitleOverlay url="/bad" position={2} />)
    await waitFor(() => expect(fetchFn).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })
})
