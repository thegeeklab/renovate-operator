const TOAST_DURATION = 5000
const TOAST_LIMIT = 5
const ANIM_DURATION = 300

// Ensure Tailwind generates these classes for the progress bar
const _twSafelist = ["bg-emerald-600", "bg-red-600", "bg-amber-600", "bg-blue-600"]
void _twSafelist

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
  toastContainer.className = "fixed bottom-4 right-4 z-50 flex flex-col-reverse gap-3 w-96"

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

function removeToast(el: HTMLElement, animate = true): void {
  if (animate) {
    el.style.transform = "translateY(120%)"
    el.style.opacity = "0"
    setTimeout(() => el.remove(), ANIM_DURATION)
  } else {
    el.remove()
  }
}

function anyHovered(): boolean {
  return activeToasts.some((t) => t.hovered)
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
    t.timer = setTimeout(() => dismissToast(t), t.remaining)
  }
}

function dismissToast(t: ToastState): void {
  const idx = activeToasts.indexOf(t)
  if (idx !== -1) activeToasts.splice(idx, 1)
  removeToast(t.el)
  if (activeToasts.length === 0) {
    toastContainer = null
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
      dismissToast(t)
    }
  }
  if (activeToasts.some((t) => t.remaining !== Infinity)) {
    requestAnimationFrame(tick)
  }
}

function createToast(message: string, type: "success" | "error" | "info" | "warning"): void {
  const container = getContainer()

  while (container.children.length >= TOAST_LIMIT) {
    const oldest = container.firstElementChild as HTMLElement
    if (oldest) removeToast(oldest, false)
  }

  const el = document.createElement("div")
  el.setAttribute("role", "status")

  const typeConfig: Record<string, { icon: string; color: string; barBg: string }> = {
    success: {
      icon: "✓",
      color: "text-emerald-600",
      barBg: "bg-emerald-600"
    },
    error: {
      icon: "✕",
      color: "text-red-600",
      barBg: "bg-red-600"
    },
    warning: {
      icon: "",
      color: "text-amber-600",
      barBg: "bg-amber-600"
    },
    info: {
      icon: "ℹ",
      color: "text-blue-600",
      barBg: "bg-blue-600"
    }
  }

  const config = typeConfig[type] || typeConfig.info

  el.className = `pointer-events-auto relative group overflow-hidden bg-white shadow-lg rounded-lg border border-gray-200 flex items-center gap-2.5 p-4 transition-all ease-out`
  el.style.transitionDuration = `${ANIM_DURATION}ms`
  el.style.transform = "translateY(120%)"
  el.style.opacity = "0"

  el.innerHTML = `
    <div class="shrink-0 w-5 h-5 ${config.color} flex items-center justify-center text-sm font-semibold">
      ${config.icon}
    </div>
    <div class="w-0 flex-1 flex flex-col">
      <p class="text-sm font-medium text-gray-900 break-words leading-snug">${message}</p>
    </div>
    <button class="shrink-0 p-0 cursor-pointer text-gray-400 hover:text-gray-600 transition-colors" aria-label="Dismiss">
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
      </svg>
    </button>
    <div class="absolute inset-x-0 bottom-0 h-1 overflow-hidden">
      <div class="absolute inset-0 bg-gray-200"></div>
      <div data-role="progress" class="absolute inset-y-0 left-0 z-10 ${config.barBg} rounded-r-full" style="transform-origin: left; transform: scaleX(1); width: 100%"></div>
    </div>
  `

  container.appendChild(el)

  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      el.style.transform = "translateY(0)"
      el.style.opacity = "1"
    })
  })

  const progressIndicator = el.querySelector('[data-role="progress"]') as HTMLElement
  const state: ToastState = {
    el,
    progressIndicator,
    remaining: TOAST_DURATION,
    startTime: Date.now(),
    timer: setTimeout(() => dismissToast(state), TOAST_DURATION),
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
    dismissToast(state)
  })
}

export const toast = {
  success: (msg: string) => createToast(msg, "success"),
  error: (msg: string) => createToast(msg, "error"),
  warning: (msg: string) => createToast(msg, "warning"),
  info: (msg: string) => createToast(msg, "info")
}
