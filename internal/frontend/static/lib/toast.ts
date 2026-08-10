import checkCircleSvg from "lucide-static/icons/circle-check.svg?raw"
import xCircleSvg from "lucide-static/icons/circle-x.svg?raw"
import exclamationTriangleSvg from "lucide-static/icons/triangle-alert.svg?raw"
import informationCircleSvg from "lucide-static/icons/info.svg?raw"
import xMarkSvg from "lucide-static/icons/x.svg?raw"
import { t } from "./i18n"

const TOAST_DURATION = 5000
const TOAST_LIMIT = 5
const ANIM_DURATION = 300
const GAP = 12

let toastContainer: HTMLElement | null = null

interface ToastState {
  el: HTMLElement
  progressIndicator: HTMLElement
  remaining: number
  startTime: number
  timer: ReturnType<typeof setTimeout>
  hovered: boolean
}

const activeToasts: ToastState[] = []

function getContainer(): HTMLElement {
  if (toastContainer) return toastContainer

  toastContainer = document.createElement("div")
  toastContainer.setAttribute("aria-live", "polite")
  toastContainer.setAttribute("aria-atomic", "true")
  toastContainer.className = "fixed bottom-4 right-4 z-50 w-96"

  toastContainer.addEventListener("mouseenter", () => {
    for (const t of activeToasts) {
      t.hovered = true
    }
    pauseAll()
  })

  toastContainer.addEventListener("mouseleave", () => {
    for (const t of activeToasts) {
      t.hovered = false
    }
    resumeAll()
  })

  document.body.appendChild(toastContainer)

  return toastContainer
}

function recomputeOffsets(): void {
  if (!toastContainer) return
  const children = Array.from(toastContainer.children) as HTMLElement[]
  let totalHeight = 0
  for (let i = 0; i < children.length; i++) {
    let offset = 0
    for (let j = i + 1; j < children.length; j++) {
      offset += children[j].offsetHeight + GAP
    }
    children[i].style.setProperty("--offset", `${offset}px`)
    totalHeight += children[i].offsetHeight
  }
  if (children.length > 0) {
    totalHeight += (children.length - 1) * GAP
  }
  toastContainer.style.height = `${totalHeight}px`
}

function cleanupContainer(): void {
  if (toastContainer && toastContainer.children.length === 0) {
    toastContainer.remove()
    toastContainer = null
  }
}

function removeToast(state: ToastState, animate = true): void {
  const idx = activeToasts.indexOf(state)
  if (idx !== -1) activeToasts.splice(idx, 1)
  clearTimeout(state.timer)

  if (animate) {
    state.el.style.opacity = "0"
    const { el } = state
    setTimeout(() => {
      el.remove()
      recomputeOffsets()
      cleanupContainer()
    }, ANIM_DURATION)
  } else {
    state.el.remove()
    recomputeOffsets()
    cleanupContainer()
  }
}

function anyHovered(): boolean {
  return activeToasts.some((t) => t.hovered)
}

function escapeHtml(unsafe: string): string {
  const div = document.createElement("div")
  div.textContent = unsafe
  return div.innerHTML
}

function pauseAll(): void {
  for (const t of activeToasts) {
    if (t.remaining === Infinity) continue
    clearTimeout(t.timer)
  }
}

function resumeAll(): void {
  for (const t of activeToasts) {
    if (t.remaining === Infinity || t.hovered) continue
    t.startTime = Date.now() - (TOAST_DURATION - t.remaining)
    t.timer = setTimeout(() => removeToast(t), t.remaining)
  }
}

function tick(): void {
  if (anyHovered()) {
    requestAnimationFrame(tick)
    return
  }
  for (let i = activeToasts.length - 1; i >= 0; i--) {
    const t = activeToasts[i]
    if (t.remaining === Infinity) continue
    const elapsed = Date.now() - t.startTime
    t.remaining = Math.max(0, TOAST_DURATION - elapsed)
    t.progressIndicator.style.transform = `scaleX(${t.remaining / TOAST_DURATION})`
    if (t.remaining <= 0) {
      removeToast(t)
    }
  }
  if (activeToasts.some((t) => t.remaining !== Infinity)) {
    requestAnimationFrame(tick)
  }
}

function createToast(message: string, type: "success" | "error" | "info" | "warning"): void {
  const container = getContainer()

  while (activeToasts.length >= TOAST_LIMIT) {
    const oldest = activeToasts.shift()
    if (!oldest) break
    removeToast(oldest, false)
  }

  const el = document.createElement("div")
  el.setAttribute("role", "status")

  const typeConfig: Record<string, { icon: string; color: string; barBg: string }> = {
    success: {
      icon: checkCircleSvg,
      color: "text-emerald-600",
      barBg: "bg-emerald-600"
    },
    error: {
      icon: xCircleSvg,
      color: "text-red-600",
      barBg: "bg-red-600"
    },
    warning: {
      icon: exclamationTriangleSvg,
      color: "text-amber-600",
      barBg: "bg-amber-600"
    },
    info: {
      icon: informationCircleSvg,
      color: "text-blue-600",
      barBg: "bg-blue-600"
    }
  }

  const config = typeConfig[type] || typeConfig.info

  el.className =
    "pointer-events-auto absolute bottom-0 left-0 right-0 group overflow-hidden bg-white dark:bg-gray-800 shadow-lg rounded-lg border border-gray-200 dark:border-gray-700 flex items-center gap-2.5 p-4"
  el.style.transition = `transform ${ANIM_DURATION}ms ease-out, opacity ${ANIM_DURATION}ms ease-out`
  el.style.transform = "translateY(calc(-1 * var(--offset, 0px)))"
  el.style.opacity = "0"

  el.innerHTML = `
    <div class="shrink-0 w-5 h-5 ${config.color} flex items-center justify-center [&>svg]:w-full [&>svg]:h-full">
      ${config.icon}
    </div>
    <div class="w-0 flex-1 flex flex-col">
      <p class="text-sm font-medium text-gray-900 dark:text-gray-100 break-words leading-snug">${escapeHtml(message)}</p>
    </div>
    <button class="shrink-0 p-0 cursor-pointer text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 transition-colors" aria-label="${t("common.dismiss")}">
      <span class="block w-4 h-4">${xMarkSvg}</span>
    </button>
    <div class="absolute inset-x-0 bottom-0 h-1 overflow-hidden">
      <div class="absolute inset-0 bg-gray-200 dark:bg-gray-700"></div>
      <div data-role="progress" class="absolute inset-y-0 left-0 z-10 ${config.barBg} rounded-r-full" style="transform-origin: left; transform: scaleX(1); width: 100%"></div>
    </div>
  `

  container.appendChild(el)
  recomputeOffsets()

  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      el.style.opacity = "1"
    })
  })

  const progressIndicator = el.querySelector('[data-role="progress"]') as HTMLElement
  const state: ToastState = {
    el,
    progressIndicator,
    remaining: TOAST_DURATION,
    startTime: Date.now(),
    timer: setTimeout(() => removeToast(state), TOAST_DURATION),
    hovered: false
  }

  activeToasts.push(state)

  if (!anyHovered()) requestAnimationFrame(tick)

  el.addEventListener("click", (e) => {
    if ((e.target as HTMLElement).closest("button")) return
    for (const t of activeToasts) {
      clearTimeout(t.timer)
      t.remaining = Infinity
    }
  })

  el.querySelector("button")?.addEventListener("click", (e) => {
    e.stopPropagation()
    removeToast(state)
  })
}

export const toast = {
  success: (msg: string) => createToast(msg, "success"),
  error: (msg: string) => createToast(msg, "error"),
  warning: (msg: string) => createToast(msg, "warning"),
  info: (msg: string) => createToast(msg, "info")
}
