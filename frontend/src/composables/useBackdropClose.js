// useBackdropClose — close a dialog when its backdrop is clicked, but only when
// the mouse button was also pressed on the backdrop.
//
// A plain `@click.self` on the backdrop is not enough: pressing the button
// inside the dialog (for example while selecting text in an input) and
// releasing it outside makes the browser fire `click` on the nearest common
// ancestor — the backdrop itself — so the dialog closes and the input is lost.
//
// Usage:
//   const backdrop = useBackdropClose(() => emit('close'))
//   <div class="modal-backdrop" @mousedown="backdrop.onMousedown" @click="backdrop.onClick">
export function useBackdropClose(onClose) {
  let pressedOnBackdrop = false
  return {
    onMousedown(event) {
      pressedOnBackdrop = event.target === event.currentTarget
    },
    onClick(event) {
      const close = pressedOnBackdrop && event.target === event.currentTarget
      pressedOnBackdrop = false
      if (close) onClose(event)
    },
  }
}
