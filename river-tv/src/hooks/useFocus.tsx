import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'

/*
 * Spatial focus manager for D-pad navigation.
 *
 * Every <Focusable> registers itself on mount. The provider attaches a
 * single global keydown listener: arrow keys pick the nearest registered
 * element in the chosen direction (by bounding-rect distance, weighted to
 * prefer items roughly in line with the current focus), Enter fires
 * onSelect, and Escape/Back fires onBack if provided.
 *
 * "Focused" is tracked via a `.focused` className we add/remove — using
 * native :focus would force every element to be a button/anchor, and we
 * want any div/card to participate.
 */

interface Entry {
  id: string
  el: HTMLElement
  onSelect?: () => void
  // When true, this entry claims focus on registration even if another
  // entry is already focused. Used by pages that load content async and
  // want the first content item focused once it arrives.
  autoFocus?: boolean
  // When true, this entry will NOT be picked as the initial focus on
  // page mount when nothing is focused yet. Used by sidebar items so the
  // sidebar doesn't briefly grab + release focus (and visually expand +
  // collapse) before async content arrives and claims focus.
  skipInitialFocus?: boolean
  // Called whenever this entry gains or loses focus. Used by containers
  // (sidebar, hero) that need to react to focus entering/leaving them.
  onFocusChange?: (focused: boolean) => void
  // Group lock: when focused on an entry with a group, up/down/left can
  // only move to other entries with the same group. Right is the
  // explicit escape that can cross out of the group. Used by the
  // sidebar so the page can't grab focus accidentally when the sidebar
  // is expanded and its hit-boxes overlap the grid.
  group?: string
  // Tag this entry as a candidate for directional overrides from other
  // entries. Cheap label, no behaviour on its own — meaningful only
  // when something else lists this tag in its `overrides`.
  tag?: string
  // When true, this entry is invisible to the spatial-nav search and
  // can only be reached via a directional `override` or `focusByTag`.
  // Used by Sort/Pager/Sidebar so a focused card can't drift into them
  // by accident. Tagged-but-not-hidden entries (e.g. the player's
  // play/pause button) stay normally navigable.
  spatialHidden?: boolean
  // Pre-empt spatial navigation: when this entry is focused and the
  // user presses the given direction, jump to the first entry whose
  // `tag` matches the listed value (instead of running the geometric
  // search). Used so cards in the first/last row can reach the Sort
  // button / pager directly, and so off-row cards can't reach them.
  overrides?: { up?: string; down?: string; left?: string; right?: string }
  // Run a callback instead of moving focus. Used by widgets like the
  // player's progress bar where Left/Right should scrub the playhead
  // rather than navigate away to another control. Checked before
  // `overrides` and before spatial search.
  customDirections?: {
    up?: () => void
    down?: () => void
    left?: () => void
    right?: () => void
  }
}

interface FocusCtx {
  register: (entry: Entry) => () => void
  focus: (id: string) => void
  focusByTag: (tag: string) => boolean
  // Suspend the provider's keydown handling while an overlay (e.g. a
  // popup with its own focus scope) is open. The provider keeps its
  // focused element styled so focus visually "stays put" underneath the
  // overlay, but key events are ignored.
  setPaused: (paused: boolean) => void
}

const Ctx = createContext<FocusCtx | null>(null)

export function useFocusContext() {
  return useContext(Ctx)
}

export function FocusProvider({
  onBack,
  backFocusesTag,
  children,
}: {
  onBack?: () => void
  // When Back is pressed and no `onBack` is provided, focus the first
  // registered entry with this tag. Lets pages wire "Back → sidebar"
  // without having to reach into the provider's internals.
  backFocusesTag?: string
  children: ReactNode
}) {
  const entries = useRef(new Map<string, Entry>())
  const focusedId = useRef<string | null>(null)
  const pausedRef = useRef(false)

  const apply = useCallback((id: string | null) => {
    const prev = focusedId.current
    if (prev && prev !== id) {
      const prevEntry = entries.current.get(prev)
      prevEntry?.el.classList.remove('focused')
      prevEntry?.onFocusChange?.(false)
    }
    focusedId.current = id
    if (!id) return
    const entry = entries.current.get(id)
    if (!entry) return
    entry.el.classList.add('focused')
    entry.el.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' })
    entry.onFocusChange?.(true)
  }, [])

  const register = useCallback((entry: Entry) => {
    entries.current.set(entry.id, entry)
    // Claim focus if (a) nothing is focused yet and this entry isn't
    // opting out, or (b) this entry asked to be the autofocus target
    // (e.g. first card on a browse page).
    if ((focusedId.current === null && !entry.skipInitialFocus) || entry.autoFocus) {
      apply(entry.id)
    }
    return () => {
      entries.current.delete(entry.id)
      // Strip the className unconditionally — in StrictMode the element
      // can outlive its effect (cleanup runs, then mount runs again on
      // the same DOM node) and a stale `.focused` would survive on every
      // unregistered-then-rebound element.
      entry.el.classList.remove('focused')
      if (focusedId.current === entry.id) {
        focusedId.current = null
        entry.onFocusChange?.(false)
        // Pick any remaining entry as fallback.
        const first = entries.current.keys().next().value
        if (first) apply(first)
      }
    }
  }, [apply])

  const focus = useCallback((id: string) => apply(id), [apply])

  const focusByTag = useCallback((tag: string) => {
    for (const e of entries.current.values()) {
      if (e.tag === tag) { apply(e.id); return true }
    }
    return false
  }, [apply])

  // Find the best candidate in the given direction relative to current.
  //
  // Two-pass: first prefer items whose cross-axis range overlaps the
  // source's (i.e. "in the same column" for vertical, "same row" for
  // horizontal). Among those, distance is just the primary axis — the
  // overlap already proved they're aligned. Only fall back to weighted
  // off-axis scoring when no in-line candidate exists. Without this,
  // pressing Up from a far-corner item (e.g. Sign Out in the sidebar)
  // would pick a much closer item in the page grid over a perfectly
  // aligned item in the same column further away.
  const move = useCallback((dir: 'up' | 'down' | 'left' | 'right') => {
    const cur = focusedId.current
    if (!cur) return
    const curEntry = entries.current.get(cur)
    if (!curEntry) return

    // Custom action handler wins first — useful for widgets like the
    // player progress bar that consume Left/Right as scrub input rather
    // than focus navigation.
    const customAction = curEntry.customDirections?.[dir]
    if (customAction) {
      customAction()
      return
    }

    // Directional override wins over spatial search. Used to express
    // "from row-0 cards, Up goes to the Sort button" without that
    // button being reachable from off-row cards by accident.
    const overrideTag = curEntry.overrides?.[dir]
    if (overrideTag) {
      for (const e of entries.current.values()) {
        if (e.tag === overrideTag) { apply(e.id); return }
      }
    }

    const from = curEntry.el.getBoundingClientRect()
    const fx = from.left + from.width / 2
    const fy = from.top + from.height / 2
    const isVertical = dir === 'up' || dir === 'down'

    // Group lock: up/down/left from a grouped entry are restricted to
    // entries with the same group. Right is the explicit escape.
    const lockGroup = curEntry.group && dir !== 'right' ? curEntry.group : null

    let bestInline: { id: string; score: number } | null = null
    let bestOffline: { id: string; score: number } | null = null

    for (const e of entries.current.values()) {
      if (e.id === cur) continue
      if (lockGroup && e.group !== lockGroup) continue
      // Spatially-hidden entries are only reachable via directional
      // overrides or from a peer that shares the same tag. This keeps
      // Sort/Pager off-limits to spatial nav from cards in middle rows
      // while still letting the two pager buttons reach each other.
      if (e.spatialHidden && e.tag !== curEntry.tag) continue
      const r = e.el.getBoundingClientRect()
      const cx = r.left + r.width / 2
      const cy = r.top + r.height / 2
      const dx = cx - fx
      const dy = cy - fy

      switch (dir) {
        case 'up':    if (dy >= -1) continue; break
        case 'down':  if (dy <=  1) continue; break
        case 'left':  if (dx >= -1) continue; break
        case 'right': if (dx <=  1) continue; break
      }

      const primary = isVertical ? Math.abs(dy) : Math.abs(dx)
      const cross   = isVertical ? Math.abs(dx) : Math.abs(dy)
      const overlap = isVertical
        ? Math.max(0, Math.min(from.right, r.right) - Math.max(from.left, r.left))
        : Math.max(0, Math.min(from.bottom, r.bottom) - Math.max(from.top, r.top))

      if (overlap > 0) {
        if (!bestInline || primary < bestInline.score) bestInline = { id: e.id, score: primary }
      } else {
        const score = primary + cross * 2
        if (!bestOffline || score < bestOffline.score) bestOffline = { id: e.id, score }
      }
    }

    const best = bestInline ?? bestOffline
    if (best) apply(best.id)
  }, [apply])

  const setPaused = useCallback((p: boolean) => { pausedRef.current = p }, [])

  useEffect(() => {
    const onKey = (ev: KeyboardEvent) => {
      if (pausedRef.current) return

      // Yield to native form controls while the user is typing. The
      // FocusProvider listens at window level, so otherwise its
      // bookkeeping runs on every keystroke (even when nothing in the
      // switch below matches), which can interfere with the WebView's
      // IME path and stop characters from reaching the input. We
      // still let Escape / GoBack through so the field can blur and
      // popups / pages can react to the back gesture.
      const active = document.activeElement as HTMLElement | null
      const typing = !!active && (
        active.tagName === 'INPUT' ||
        active.tagName === 'TEXTAREA' ||
        active.isContentEditable
      )
      if (typing && ev.key !== 'Escape' && ev.key !== 'GoBack') return

      switch (ev.key) {
        case 'ArrowUp':    ev.preventDefault(); move('up');    break
        case 'ArrowDown':  ev.preventDefault(); move('down');  break
        case 'ArrowLeft':  ev.preventDefault(); move('left');  break
        case 'ArrowRight': ev.preventDefault(); move('right'); break
        case 'Enter': {
          ev.preventDefault()
          const cur = focusedId.current
          if (!cur) return
          entries.current.get(cur)?.onSelect?.()
          break
        }
        case 'Escape':
        case 'Backspace':
        case 'GoBack':
          if (onBack) { ev.preventDefault(); onBack() }
          else if (backFocusesTag && focusByTag(backFocusesTag)) ev.preventDefault()
          break
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [move, onBack, backFocusesTag, focusByTag])

  const value = useMemo<FocusCtx>(
    () => ({ register, focus, focusByTag, setPaused }),
    [register, focus, focusByTag, setPaused],
  )
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

let nextId = 0
function genId() { return `f${++nextId}` }

export function useFocusable<T extends HTMLElement = HTMLElement>(
  onSelect?: () => void,
  options?: {
    autoFocus?: boolean
    skipInitialFocus?: boolean
    group?: string
    tag?: string
    spatialHidden?: boolean
    overrides?: { up?: string; down?: string; left?: string; right?: string }
    customDirections?: {
      up?: () => void
      down?: () => void
      left?: () => void
      right?: () => void
    }
    onFocusChange?: (focused: boolean) => void
  },
) {
  const ref = useRef<T | null>(null)
  const [id] = useState(genId)
  const ctx = useContext(Ctx)
  const autoFocus = options?.autoFocus
  const skipInitialFocus = options?.skipInitialFocus
  const group = options?.group
  const tag = options?.tag
  const spatialHidden = options?.spatialHidden
  // Stash inline-object options via refs so callers can pass them
  // without triggering re-registration each render.
  const overridesRef = useRef(options?.overrides)
  overridesRef.current = options?.overrides
  const customDirectionsRef = useRef(options?.customDirections)
  customDirectionsRef.current = options?.customDirections

  // Stash handlers in refs so re-renders that pass new function
  // identities don't force unregister/register churn. Without this, a
  // parent that creates `onSelect={() => …}` inline would cause its
  // child to deregister on every render — and if that child was the
  // currently focused element, the unregister cascade would shift focus
  // away before the next register call finished.
  const onSelectRef = useRef(onSelect)
  onSelectRef.current = onSelect
  const onFocusChangeRef = useRef(options?.onFocusChange)
  onFocusChangeRef.current = options?.onFocusChange

  useEffect(() => {
    if (!ctx || !ref.current) return
    return ctx.register({
      id,
      el: ref.current,
      onSelect: () => onSelectRef.current?.(),
      autoFocus,
      skipInitialFocus,
      group,
      tag,
      spatialHidden,
      // Always read through refs so inline-object identity changes
      // don't force re-registration.
      get overrides() { return overridesRef.current },
      get customDirections() { return customDirectionsRef.current },
      onFocusChange: focused => onFocusChangeRef.current?.(focused),
    })
  }, [ctx, id, autoFocus, skipInitialFocus, group, tag, spatialHidden])

  return ref
}
