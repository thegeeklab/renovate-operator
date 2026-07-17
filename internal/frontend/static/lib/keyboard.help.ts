interface Shortcut {
  steps: string[][]
  description: string
}

interface ShortcutSection {
  title: string
  columns: Shortcut[][]
  wide?: boolean
}

const sections: ShortcutSection[] = [
  {
    title: "General",
    wide: true,
    columns: [
      [
        { steps: [["?"]], description: "Show keyboard shortcuts" },
        { steps: [["Esc"]], description: "Close modal or blur input" }
      ],
      [
        { steps: [["r"]], description: "Refresh current view" },
        { steps: [["t"]], description: "Cycle theme" }
      ]
    ]
  },
  {
    title: "Navigation",
    wide: true,
    columns: [
      [
        { steps: [["j", "↓"]], description: "Next item" },
        { steps: [["k", "↑"]], description: "Previous item" },
        { steps: [["g"], ["g"]], description: "First item" },
        { steps: [["G"]], description: "Last item" }
      ],
      [
        { steps: [["/"]], description: "Focus search" },
        { steps: [["g"], ["h"]], description: "Go to home" },
        { steps: [["Enter", "o"]], description: "Open focused item" }
      ]
    ]
  },
  {
    title: "Log viewer",
    columns: [
      [
        { steps: [["c"]], description: "Close log viewer" },
        { steps: [["d"]], description: "Download log" },
        { steps: [["i"]], description: "Toggle details" }
      ]
    ]
  }
]

class KeyboardHelpModal {
  private previouslyFocused: HTMLElement | null = null

  constructor(private modal: HTMLElement) {
    this.bindEvents()
  }

  private bindEvents(): void {
    this.modal.querySelectorAll("[data-close]").forEach((el) => {
      el.addEventListener("click", () => this.hide())
    })
  }

  show(): void {
    this.previouslyFocused = document.activeElement as HTMLElement | null
    this.modal.classList.remove("hidden")
    document.body.style.overflow = "hidden"
    const closeBtn = this.modal.querySelector<HTMLButtonElement>("[data-autofocus]")
    closeBtn?.focus()
  }

  hide(): void {
    this.modal.classList.add("hidden")
    document.body.style.overflow = ""
    this.previouslyFocused?.focus()
    this.previouslyFocused = null
  }

  destroy(): void {
    this.modal.remove()
  }
}

const instances = new Map<HTMLElement, KeyboardHelpModal>()

function ensureInstance(): void {
  let el = document.getElementById("keyboard-help-modal")
  if (!el) {
    el = document.createElement("div")
    el.id = "keyboard-help-modal"
    el.className = "hidden"
    el.setAttribute("role", "dialog")
    el.setAttribute("aria-modal", "true")
    el.setAttribute("aria-labelledby", "keyboard-help-title")
    el.innerHTML = `
      <div class="fixed inset-0 z-[100] overflow-y-auto">
        <div class="flex items-center justify-center min-h-screen px-4 py-8 text-center sm:block sm:p-0">
          <div class="fixed inset-0 bg-gray-900/50 backdrop-blur-sm transition-opacity" data-close aria-hidden="true"></div>
          <span class="hidden sm:inline-block sm:align-middle sm:h-screen" aria-hidden="true">&#8203;</span>
          <div class="relative inline-block align-middle bg-white dark:bg-gray-800 rounded-lg text-left overflow-hidden shadow-2xl transform transition-all sm:my-8 sm:max-w-2xl sm:w-full z-10">
            <div class="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
              <h3 id="keyboard-help-title" class="text-base font-semibold text-gray-900 dark:text-gray-100">Keyboard shortcuts</h3>
              <button type="button" data-close data-autofocus class="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 focus:outline-none cursor-pointer rounded-md p-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors" aria-label="Close">
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 p-4">
              ${renderSections()}
            </div>
          </div>
        </div>
      </div>
    `
    document.body.appendChild(el)
  }
  if (!instances.has(el)) {
    instances.set(el, new KeyboardHelpModal(el))
  }
}

function renderShortcutRow(shortcut: Shortcut): string {
  return `
    <div class="flex items-center justify-between gap-4 px-4 py-2 border-b border-gray-100 dark:border-gray-700 last:border-b-0">
      <span class="text-sm text-gray-700 dark:text-gray-300">${shortcut.description}</span>
      <div class="flex items-center gap-1 shrink-0">
        ${renderShortcut(shortcut.steps)}
      </div>
    </div>
  `
}

function renderSections(): string {
  return sections
    .map(
      (section) => `
        <div class="rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden ${
          section.wide ? "sm:col-span-2" : ""
        }">
          <div class="bg-gray-50 dark:bg-gray-900 px-4 py-2 border-b border-gray-200 dark:border-gray-700">
            <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">${section.title}</h4>
          </div>
          <div class="${section.wide ? "sm:grid sm:grid-cols-2 sm:gap-x-6" : "flex flex-col"}">
            ${section.columns
              .map(
                (column) =>
                  `<div class="flex flex-col">${column.map(renderShortcutRow).join("")}</div>`
              )
              .join("")}
          </div>
        </div>
      `
    )
    .join("")
}

function renderShortcut(steps: string[][]): string {
  return steps
    .map((step) =>
      step
        .map((key, keyIdx) => {
          const sep = keyIdx > 0 ? `<span class="text-gray-500 text-[10px] px-0.5">or</span>` : ""
          return `${sep}<kbd>${key}</kbd>`
        })
        .join("")
    )
    .join("")
}

export function show(): void {
  ensureInstance()
  const el = document.getElementById("keyboard-help-modal")
  if (!el) return
  const modal = instances.get(el) ?? null
  if (modal) {
    modal.show()
  }
}

export function hide(): void {
  const el = document.getElementById("keyboard-help-modal")
  if (!el) return
  const modal = instances.get(el) ?? null
  if (modal) {
    modal.hide()
  }
}

export function toggle(): void {
  const el = document.getElementById("keyboard-help-modal")
  if (el && !el.classList.contains("hidden")) {
    hide()
  } else {
    show()
  }
}

export function isVisible(): boolean {
  const el = document.getElementById("keyboard-help-modal")
  return el !== null && !el.classList.contains("hidden")
}

export function destroy(): void {
  for (const modal of instances.values()) {
    modal.destroy()
  }
  instances.clear()
}
