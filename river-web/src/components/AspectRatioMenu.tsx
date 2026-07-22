import { RiAspectRatioLine, RiZoomInLine, RiZoomOutLine, RiResetRightLine } from 'react-icons/ri'
import type { FitMode } from '../hooks/useAspectRatio'

interface Props {
  open:        boolean
  fitMode:     FitMode
  zoom:        number
  minZoom:     number
  maxZoom:     number
  onToggle:    () => void
  onSetFit:    (m: FitMode) => void
  onZoomIn:    () => void
  onZoomOut:   () => void
  onReset:     () => void
  // CSS-Modules object from the parent page so this component picks up the
  // same submenu styling as the audio/subtitle popovers. Typed loosely as
  // Record<string, string> to match Vite's CSSModuleClasses import shape;
  // we look up keys by name and tolerate undefined for any that are missing.
  styles: Record<string, string>
}

const FIT_LABELS: Record<FitMode, string> = {
  contain: 'Fit (Original)',
  fill:    'Stretch',
  cover:   'Crop / Zoom Fill',
}

// AspectRatioMenu is a popover for video display options:
//   - Fit mode: Original (letterbox), Stretch (fill, distorts), Crop (cover,
//     trims edges to fill the screen).
//   - Zoom: multiplicative scale 0.5x..2.0x in 5% steps. Independent of
//     fit mode, so e.g. "Original + 110%" gives a slight zoom into a
//     letterboxed source.
//   - Reset returns to defaults.
export function AspectRatioMenu(p: Props) {
  const pct = Math.round(p.zoom * 100)
  const atMin = p.zoom <= p.minZoom + 0.001
  const atMax = p.zoom >= p.maxZoom - 0.001

  return (
    <div className={p.styles.subMenu}>
      <button
        className={`btn btn-icon ${p.styles.controlBtn}`}
        onClick={e => { e.stopPropagation(); p.onToggle() }}
        aria-label="Display options"
        title="Display options"
      >
        <RiAspectRatioLine size={20} />
      </button>
      {p.open && (
        <div className={p.styles.subMenuList} onClick={e => e.stopPropagation()} style={{ minWidth: 220 }}>
          {(Object.keys(FIT_LABELS) as FitMode[]).map(m => (
            <button
              key={m}
              className={`${p.styles.subMenuOption} ${m === p.fitMode ? p.styles.subMenuOptionActive : ''}`}
              onClick={() => p.onSetFit(m)}
            >
              {FIT_LABELS[m]}
            </button>
          ))}

          <div style={{
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            padding: '8px 12px', marginTop: 4,
            borderTop: '1px solid rgba(255,255,255,0.08)',
          }}>
            <span style={{ fontSize: 13, color: 'rgba(255,255,255,0.85)' }}>Zoom</span>
            <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <button
                className={`btn btn-icon ${p.styles.controlBtn}`}
                onClick={p.onZoomOut}
                disabled={atMin}
                aria-label="Zoom out"
                title="Zoom out"
              >
                <RiZoomOutLine size={16} />
              </button>
              <span style={{
                minWidth: 44, textAlign: 'center', fontSize: 13, fontVariantNumeric: 'tabular-nums',
                color: 'rgba(255,255,255,0.85)',
              }}>
                {pct}%
              </span>
              <button
                className={`btn btn-icon ${p.styles.controlBtn}`}
                onClick={p.onZoomIn}
                disabled={atMax}
                aria-label="Zoom in"
                title="Zoom in"
              >
                <RiZoomInLine size={16} />
              </button>
            </div>
          </div>

          <button className={p.styles.subMenuOption} onClick={p.onReset} style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <RiResetRightLine size={14} />
            Reset to defaults
          </button>
        </div>
      )}
    </div>
  )
}
