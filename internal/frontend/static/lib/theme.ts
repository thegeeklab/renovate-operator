import { getPersisted, setPersisted } from "./storage"

export type ThemeMode = "light" | "dark" | "auto"

const STORAGE_KEY = "theme"
const DARK_CLASS = "dark"

const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)")

function resolveEffective(mode: ThemeMode): boolean {
  if (mode === "auto") {
    return mediaQuery.matches
  }
  return mode === "dark"
}

function applyClass(dark: boolean): void {
  document.documentElement.classList.toggle(DARK_CLASS, dark)
}

function getStoredMode(): ThemeMode {
  const stored = getPersisted<string>(STORAGE_KEY, "auto")
  if (stored === "light" || stored === "dark" || stored === "auto") {
    return stored
  }
  return "auto"
}

function updateSwitcherUI(mode: ThemeMode): void {
  const switcher = document.querySelector<HTMLElement>('[data-component="theme-switcher"]')
  if (!switcher) return

  switcher.querySelectorAll<HTMLButtonElement>("[data-theme-option]").forEach((btn) => {
    const option = btn.getAttribute("data-theme-option")
    const isActive = option === mode
    btn.setAttribute("aria-pressed", String(isActive))
    btn.classList.toggle("bg-gray-700", isActive)
    btn.classList.toggle("text-white", isActive)
    btn.classList.toggle("text-gray-400", !isActive)
    btn.classList.toggle("hover:text-gray-200", !isActive)
    btn.classList.toggle("hover:bg-gray-700/50", !isActive)
  })
}

export function getTheme(): ThemeMode {
  return getStoredMode()
}

export function setTheme(mode: ThemeMode): void {
  setPersisted(STORAGE_KEY, mode)
  applyClass(resolveEffective(mode))
  updateSwitcherUI(mode)
}

function onSystemChange(): void {
  if (getStoredMode() === "auto") {
    applyClass(mediaQuery.matches)
  }
}

export function initTheme(): void {
  const mode = getStoredMode()
  applyClass(resolveEffective(mode))
  updateSwitcherUI(mode)

  mediaQuery.addEventListener("change", onSystemChange)
}

export function initThemeSwitcher(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="theme-switcher"]').forEach((switcher) => {
    if (switcher.dataset.themeBound) return
    switcher.dataset.themeBound = "true"

    updateSwitcherUI(getStoredMode())

    switcher.querySelectorAll<HTMLButtonElement>("[data-theme-option]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const option = btn.getAttribute("data-theme-option") as ThemeMode | null
        if (option === "light" || option === "dark" || option === "auto") {
          setTheme(option)
        }
      })
    })
  })
}
