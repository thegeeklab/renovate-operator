import { tinykeys } from "tinykeys"
import * as keyboardHelp from "./keyboard.help"
import { setTheme, getStoredMode, type ThemeMode } from "./theme"

type Unsubscribe = () => void

function isEditableTarget(): boolean {
  const el = document.activeElement
  if (!el) return false
  const tag = el.tagName
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true
  if ((el as HTMLElement).isContentEditable) return true
  return false
}

function guard(
  handler: (e: KeyboardEvent) => void,
  opts: { allowInEditable?: boolean } = {}
): (e: KeyboardEvent) => void {
  if (opts.allowInEditable) return handler
  return (e: KeyboardEvent) => {
    if (isEditableTarget()) return
    handler(e)
  }
}

function focusSearch(): void {
  const logViewer = document.getElementById("log-viewer")
  if (logViewer && !logViewer.classList.contains("hidden")) {
    const logSearch = logViewer.querySelector<HTMLInputElement>('input[name="log-search"]')
    if (logSearch) {
      logSearch.focus()
      logSearch.select()
      return
    }
  }
  const input = document.querySelector<HTMLInputElement>('input[name="search"]')
  if (!input) return
  input.focus()
  input.select()
}

function navigate(path: string): void {
  if (!window.htmx) return
  window.htmx.ajax("get", path, {
    target: document.body,
    swap: "innerHTML",
    headers: { "HX-Boosted": "true" },
    push: path
  })
}

function getKbNavItems(): HTMLElement[] {
  const scope = (document.activeElement as HTMLElement | null)?.closest<HTMLElement>(
    "[data-kb-nav-scope]"
  )
  const root: ParentNode = scope ?? document
  return Array.from(root.querySelectorAll<HTMLElement>("[data-kb-nav]"))
}

function findFocusedIndex(items: HTMLElement[]): number {
  const active = document.activeElement as HTMLElement | null
  if (!active) return -1
  return items.findIndex((el) => el === active || el.contains(active))
}

function focusItem(el: HTMLElement | undefined): void {
  if (!el) return
  el.focus({ preventScroll: false })
  el.scrollIntoView({ block: "nearest", behavior: "instant" })
}

function moveFocus(delta: number): boolean {
  const items = getKbNavItems()
  if (items.length === 0) return false
  const currentIdx = findFocusedIndex(items)
  let nextIdx: number
  if (currentIdx === -1) {
    nextIdx = delta > 0 ? 0 : items.length - 1
  } else {
    nextIdx = (currentIdx + delta + items.length) % items.length
  }
  focusItem(items[nextIdx])
  return true
}

function focusEdge(edge: "first" | "last"): void {
  const items = getKbNavItems()
  if (items.length === 0) return
  focusItem(edge === "first" ? items[0] : items[items.length - 1])
}

function activateFocused(): void {
  const active = document.activeElement as HTMLElement | null
  if (!active) return
  const target = active.closest<HTMLElement>("[data-kb-nav]") ?? active
  if (target.tagName === "A" || target.tagName === "BUTTON") {
    target.click()
    return
  }
  if (target.tagName === "SUMMARY") {
    const details = target.closest<HTMLDetailsElement>("details")
    if (details) {
      const summary = details.querySelector<HTMLElement>("summary")
      summary?.click()
    }
  }
}

function isLogViewerOpen(): boolean {
  const logViewer = document.getElementById("log-viewer")
  return !!logViewer && !logViewer.classList.contains("hidden")
}

function triggerLogAction(action: string): boolean {
  if (!isLogViewerOpen()) return false
  const btn = document
    .getElementById("log-viewer")
    ?.querySelector<HTMLButtonElement>(`[data-action="${action}"]`)
  if (!btn) return false
  btn.click()
  return true
}

function closeLogViewer(): boolean {
  return triggerLogAction("close-logs")
}

let refreshInFlight = false

function refreshCurrentView(): void {
  if (!window.htmx) return
  const dashboard = document.getElementById("dashboard-content")
  if (!dashboard) return
  if (refreshInFlight) return
  refreshInFlight = true
  const path = window.location.pathname + window.location.search
  window.htmx.ajax("get", path, {
    target: "#dashboard-content",
    swap: "innerHTML",
    headers: { "HX-Boosted": "true" }
  })
  setTimeout(() => {
    refreshInFlight = false
  }, 500)
}

const THEME_CYCLE: ThemeMode[] = ["light", "dark", "auto"]

function cycleTheme(): void {
  const current = getStoredMode()
  const idx = THEME_CYCLE.indexOf(current)
  const next = THEME_CYCLE[(idx + 1) % THEME_CYCLE.length]
  setTheme(next)
}

let u: Unsubscribe | null = null
let slashHandler: ((e: KeyboardEvent) => void) | null = null
let questionHandler: ((e: KeyboardEvent) => void) | null = null
let searchEscHandler: ((e: KeyboardEvent) => void) | null = null
let navigationHandler: ((e: KeyboardEvent) => void) | null = null

function cleanup(): void {
  if (u) {
    u()
    u = null
  }
  if (slashHandler) {
    window.removeEventListener("keydown", slashHandler)
    slashHandler = null
  }
  if (questionHandler) {
    window.removeEventListener("keydown", questionHandler)
    questionHandler = null
  }
  if (searchEscHandler) {
    document.removeEventListener("keydown", searchEscHandler, true)
    searchEscHandler = null
  }
  if (navigationHandler) {
    window.removeEventListener("keydown", navigationHandler)
    navigationHandler = null
  }
  keyboardHelp.destroy()
}

export function initKeyboard(): void {
  cleanup()

  slashHandler = (e: KeyboardEvent) => {
    if (isEditableTarget()) return
    if (e.key === "/") {
      e.preventDefault()
      focusSearch()
    }
  }
  window.addEventListener("keydown", slashHandler)

  questionHandler = (e: KeyboardEvent) => {
    if (isEditableTarget()) return
    if (e.key === "?") {
      e.preventDefault()
      keyboardHelp.toggle()
    }
  }
  window.addEventListener("keydown", questionHandler)

  searchEscHandler = (e: KeyboardEvent) => {
    if (e.key !== "Escape") return
    if (keyboardHelp.isVisible()) {
      keyboardHelp.hide()
      return
    }
    const target = e.target as HTMLElement | null
    if (target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA")) {
      target.blur()
    }
  }
  document.addEventListener("keydown", searchEscHandler, true)

  navigationHandler = (e: KeyboardEvent) => {
    if (isEditableTarget()) return
    if (e.metaKey || e.ctrlKey || e.altKey) return

    if (e.key === "j" || e.key === "ArrowDown") {
      if (moveFocus(1)) {
        e.preventDefault()
      }
      return
    }
    if (e.key === "k" || e.key === "ArrowUp") {
      if (moveFocus(-1)) {
        e.preventDefault()
      }
      return
    }
    if (e.key === "Enter" || e.key === "o") {
      const active = document.activeElement as HTMLElement | null
      const isKbNav =
        active?.hasAttribute("data-kb-nav") ||
        active?.closest<HTMLElement>("[data-kb-nav]") !== null
      if (isKbNav) {
        e.preventDefault()
        activateFocused()
      }
      return
    }
    if (e.key === "c") {
      if (closeLogViewer()) {
        e.preventDefault()
      }
      return
    }
    if (e.key === "d") {
      if (triggerLogAction("download-log")) {
        e.preventDefault()
      }
      return
    }
    if (e.key === "i") {
      if (triggerLogAction("toggle-details")) {
        e.preventDefault()
      }
      return
    }
    if (e.key === "r") {
      e.preventDefault()
      refreshCurrentView()
      return
    }
    if (e.key === "t") {
      e.preventDefault()
      cycleTheme()
      return
    }
    if (e.key === "G") {
      e.preventDefault()
      focusEdge("last")
      return
    }
  }
  window.addEventListener("keydown", navigationHandler)

  u = tinykeys(window, {
    "g h": guard(() => {
      navigate("/")
    }),
    "g g": guard(() => {
      focusEdge("first")
    })
  })
}
