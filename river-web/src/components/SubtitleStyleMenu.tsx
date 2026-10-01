import { RiFontSize, RiResetRightLine } from 'react-icons/ri'
import { FONT_PRESETS, SUBTITLE_COLORS } from '../hooks/useSubtitleStyle'

interface Props {
  open: boolean
  fontScale: number
  color: string
  bgOpacity: number
  onToggle: () => void
  onSetFontScale: (s: number) => void
  onSetColor: (c: string) => void
  onSetBgOpacity: (o: number) => void
  onReset: () => void
  // CSS-Modules object from the parent page so this popover picks up the same
  // submenu styling as the audio/subtitle/offset popovers. Typed loosely to
  // match Vite's CSSModuleClasses import shape (mirrors AspectRatioMenu).
  styles: Record<string, string>
}

// SubtitleStyleMenu is the player popover for caption *appearance* — text
// size, colour and background opacity — distinct from the existing subtitle
// track picker and timing-offset popovers. It drives useSubtitleStyle, whose
// values the watch page applies to the .subtitleOverlay via CSS custom
// properties. Shared by MovieWatchPage and EpisodeWatchPage.
export function SubtitleStyleMenu(p: Props) {
  return (
    <div className={p.styles.subMenu}>
      <button
        className={`btn btn-icon ${p.styles.controlBtn}`}
        onClick={e => { e.stopPropagation(); p.onToggle() }}
        aria-label="Subtitle appearance"
        title="Subtitle appearance"
      >
        <RiFontSize size={20} />
      </button>
      {p.open && (
        <div
          className={p.styles.subMenuList}
          onClick={e => e.stopPropagation()}
          style={{ minWidth: 230, padding: 'var(--space-2)' }}
        >
          <div className="label-sm" style={{ textAlign: 'center', marginBottom: 6 }}>Text size</div>
          <div style={{ display: 'flex', gap: 4, justifyContent: 'center', marginBottom: 10 }}>
            {FONT_PRESETS.map(preset => (
              <button
                key={preset.label}
                className={`btn ${Math.abs(p.fontScale - preset.scale) < 0.001 ? 'btn-primary' : ''}`}
                onClick={() => p.onSetFontScale(preset.scale)}
              >
                {preset.label}
              </button>
            ))}
          </div>

          <div className="label-sm" style={{ textAlign: 'center', marginBottom: 6 }}>Colour</div>
          <div style={{ display: 'flex', gap: 10, justifyContent: 'center', marginBottom: 10 }}>
            {SUBTITLE_COLORS.map(c => (
              <button
                key={c.value}
                aria-label={c.label}
                aria-pressed={p.color === c.value}
                title={c.label}
                onClick={() => p.onSetColor(c.value)}
                style={{
                  width: 26,
                  height: 26,
                  borderRadius: '50%',
                  background: c.value,
                  padding: 0,
                  cursor: 'pointer',
                  border: p.color === c.value
                    ? '2px solid var(--accent, #fff)'
                    : '2px solid rgba(255, 255, 255, 0.3)',
                }}
              />
            ))}
          </div>

          <div style={{
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            padding: '0 4px', marginBottom: 10, gap: 8,
          }}>
            <span style={{ fontSize: 13, color: 'rgba(255, 255, 255, 0.85)' }}>Background</span>
            <input
              type="range" min={0} max={1} step={0.05} value={p.bgOpacity}
              aria-label="Subtitle background opacity"
              onChange={e => p.onSetBgOpacity(Number(e.target.value))}
            />
          </div>

          <button
            className={p.styles.subMenuOption}
            onClick={p.onReset}
            style={{ display: 'flex', alignItems: 'center', gap: 6, width: '100%' }}
          >
            <RiResetRightLine size={14} />
            Reset to defaults
          </button>
        </div>
      )}
    </div>
  )
}
