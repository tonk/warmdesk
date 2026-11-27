import { describe, it, expect, vi } from 'vitest'
import { useBackdropClose } from '../useBackdropClose'

// Simulate the events the browser sends: target is where the event happened,
// currentTarget is the backdrop the listener is on.
const backdrop = {}
const inside = {}
const ev = (target) => ({ target, currentTarget: backdrop })

describe('useBackdropClose', () => {
  it('closes when the button is pressed and released on the backdrop', () => {
    const onClose = vi.fn()
    const b = useBackdropClose(onClose)
    b.onMousedown(ev(backdrop))
    b.onClick(ev(backdrop))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('stays open when the press started inside the dialog (e.g. selecting text)', () => {
    const onClose = vi.fn()
    const b = useBackdropClose(onClose)
    b.onMousedown(ev(inside))
    // released on the backdrop: the browser fires click on the common ancestor
    b.onClick(ev(backdrop))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('stays open for a click inside the dialog', () => {
    const onClose = vi.fn()
    const b = useBackdropClose(onClose)
    b.onMousedown(ev(inside))
    b.onClick(ev(inside))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('does not carry a press over to the next click', () => {
    const onClose = vi.fn()
    const b = useBackdropClose(onClose)
    b.onMousedown(ev(backdrop))
    b.onClick(ev(inside))
    b.onClick(ev(backdrop)) // a click without its own press on the backdrop
    expect(onClose).not.toHaveBeenCalled()
  })
})
