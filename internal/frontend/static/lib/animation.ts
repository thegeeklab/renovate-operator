export function animateDetailsElement(
  el: HTMLDetailsElement,
  open: boolean,
  onComplete?: () => void
): void {
  if (open) {
    el.open = true
    onComplete?.()

    return
  }

  el.classList.add("animating-closed")

  const raw = getComputedStyle(el).getPropertyValue("--details-animation-duration").trim()

  let durationMs = 300

  if (raw.endsWith("ms")) {
    durationMs = Number.parseInt(raw.slice(0, -2), 10) || 300
  }

  setTimeout(() => {
    el.open = false
    el.classList.remove("animating-closed")
    onComplete?.()
  }, durationMs)
}
