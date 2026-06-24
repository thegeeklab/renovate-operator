import { computePosition, flip, offset, shift } from "@floating-ui/dom"

const TOOLTIP_ATTR = "data-tooltip"
let activeTooltip: HTMLElement | null = null

function getTooltipContent(trigger: HTMLElement): HTMLElement | null {
  return trigger.querySelector(".tooltip-text")
}

async function showTooltip(trigger: HTMLElement): Promise<void> {
  const tooltip = getTooltipContent(trigger)
  if (!tooltip) return

  tooltip.style.position = "fixed"
  tooltip.style.opacity = "1"
  activeTooltip = tooltip

  const { x, y } = await computePosition(trigger, tooltip, {
    placement: "top",
    middleware: [offset(8), flip(), shift({ padding: 8 })]
  })

  Object.assign(tooltip.style, {
    left: `${x}px`,
    top: `${y}px`
  })
}

function hideTooltip(trigger: HTMLElement): void {
  const tooltip = getTooltipContent(trigger)
  if (!tooltip) return

  tooltip.style.opacity = "0"
  tooltip.style.left = "-9999px"
  tooltip.style.top = "-9999px"
  activeTooltip = null
}

export function initTooltips(root: ParentNode = document): void {
  const triggers = root.querySelectorAll<HTMLElement>(`[${TOOLTIP_ATTR}]`)
  triggers.forEach((trigger) => {
    trigger.addEventListener("mouseenter", () => showTooltip(trigger))
    trigger.addEventListener("mouseleave", () => hideTooltip(trigger))
    trigger.addEventListener("focusin", () => showTooltip(trigger))
    trigger.addEventListener("focusout", () => hideTooltip(trigger))
  })
}

export function hideActiveTooltip(): void {
  if (activeTooltip) {
    activeTooltip.style.opacity = "0"
    activeTooltip.style.left = "-9999px"
    activeTooltip.style.top = "-9999px"
    activeTooltip = null
  }
}
