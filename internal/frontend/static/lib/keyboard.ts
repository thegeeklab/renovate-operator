import { tinykeys } from "tinykeys"

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

let u: Unsubscribe | null = null
let slashHandler: ((e: KeyboardEvent) => void) | null = null
let searchEscHandler: ((e: KeyboardEvent) => void) | null = null

function cleanup(): void {
  if (u) {
    u()
    u = null
  }
  if (slashHandler) {
    window.removeEventListener("keydown", slashHandler)
    slashHandler = null
  }
  if (searchEscHandler) {
    document.removeEventListener("keydown", searchEscHandler, true)
    searchEscHandler = null
  }
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

  searchEscHandler = (e: KeyboardEvent) => {
    if (e.key !== "Escape") return
    const target = e.target as HTMLElement | null
    if (target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA")) {
      target.blur()
    }
  }
  document.addEventListener("keydown", searchEscHandler, true)

  u = tinykeys(window, {
    "g h": guard(() => {
      navigate("/")
    })
  })
}

export function destroyKeyboard(): void {
  cleanup()
}
