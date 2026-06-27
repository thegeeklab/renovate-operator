import { computePosition, flip, offset, shift } from "@floating-ui/dom"
import { registerComponent } from "../lib/component.registry"
import { getData } from "../lib/dom"
import { getPersisted, setPersisted } from "../lib/storage"

const ALL_LEVELS = [10, 20, 30, 40, 50, 60]

export class LogLevelFilter {
  private el: HTMLElement
  private button: HTMLButtonElement
  private menu: HTMLDivElement
  private isOpen = false
  private activeLevels: Set<number>
  private storageKey: string

  private boundToggle: (e: Event) => void
  private boundOutside: (e: Event) => void
  private boundKeydown: (e: KeyboardEvent) => void

  constructor(el: HTMLElement) {
    this.el = el
    this.button = el.querySelector<HTMLButtonElement>('[data-action="toggle-level-filter"]')!
    this.menu = el.querySelector<HTMLDivElement>('[data-role="level-menu"]')!

    const logViewer = el.closest('[data-component="log-viewer"]')
    const namespace = logViewer ? getData(logViewer, "namespace") : ""
    const runner = logViewer ? getData(logViewer, "runner") : ""
    const jobName = logViewer ? getData(logViewer, "job-name") : ""
    this.storageKey = `loglevels-${namespace}-${runner}-${jobName}`

    const stored = getPersisted<number[] | null>(this.storageKey, null)
    this.activeLevels = new Set(stored && stored.length > 0 ? stored : ALL_LEVELS)

    this.boundToggle = this.handleToggle.bind(this)
    this.boundOutside = this.handleOutsideClick.bind(this)
    this.boundKeydown = this.handleKeydown.bind(this)

    this.syncCheckboxes()
    this.applyFilter()
    this.updateCount()
    this.bindEvents()
  }

  private bindEvents(): void {
    this.button.addEventListener("click", this.boundToggle)
    document.addEventListener("click", this.boundOutside)
    document.addEventListener("keydown", this.boundKeydown)

    this.menu.querySelectorAll<HTMLInputElement>('input[type="checkbox"]').forEach((cb) => {
      cb.addEventListener("change", () => this.handleLevelChange(cb))
    })
  }

  private handleToggle(e: Event): void {
    e.stopPropagation()
    if (this.isOpen) {
      this.close()
    } else {
      this.open()
    }
  }

  private handleOutsideClick(e: Event): void {
    if (this.isOpen && !this.el.contains(e.target as Node)) {
      this.close()
    }
  }

  private handleKeydown(e: KeyboardEvent): void {
    if (e.key === "Escape" && this.isOpen) {
      this.close()
      this.button.focus()
    }
  }

  private async open(): Promise<void> {
    this.isOpen = true
    this.menu.classList.remove("hidden")
    this.button.setAttribute("aria-expanded", "true")

    const { x, y } = await computePosition(this.button, this.menu, {
      placement: "bottom-start",
      strategy: "fixed",
      middleware: [offset(4), flip(), shift({ padding: 8 })]
    })

    Object.assign(this.menu.style, {
      left: `${x}px`,
      top: `${y}px`,
      position: "fixed"
    })
  }

  private close(): void {
    this.isOpen = false
    this.menu.classList.add("hidden")
    this.button.setAttribute("aria-expanded", "false")
  }

  private handleLevelChange(checkbox: HTMLInputElement): void {
    const level = parseInt(checkbox.dataset.level || "0", 10)
    if (!level) return

    if (checkbox.checked) {
      this.activeLevels.add(level)
    } else {
      this.activeLevels.delete(level)
    }

    this.persist()
    this.applyFilter()
    this.updateCount()
  }

  private syncCheckboxes(): void {
    this.menu.querySelectorAll<HTMLInputElement>('input[type="checkbox"]').forEach((cb) => {
      const level = parseInt(cb.dataset.level || "0", 10)
      cb.checked = this.activeLevels.has(level)
    })
  }

  private applyFilter(): void {
    const logViewer = this.el.closest('[data-component="log-viewer"]')
    if (!logViewer) return

    logViewer.querySelectorAll<HTMLElement>(".log-line[data-level]").forEach((line) => {
      const level = parseInt(line.dataset.level || "0", 10)
      line.classList.toggle("hidden", !this.activeLevels.has(level))
    })
  }

  private updateCount(): void {
    const countEl = this.el.querySelector<HTMLElement>('[data-role="level-count"]')
    if (countEl) {
      countEl.textContent = `${this.activeLevels.size}/${ALL_LEVELS.length}`
    }
  }

  private persist(): void {
    setPersisted(this.storageKey, Array.from(this.activeLevels).sort())
  }

  destroy(): void {
    this.button.removeEventListener("click", this.boundToggle)
    document.removeEventListener("click", this.boundOutside)
    document.removeEventListener("keydown", this.boundKeydown)
  }
}

export function initLogLevelFilters(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="log-level-filter"]').forEach((el) => {
    const component = new LogLevelFilter(el)
    registerComponent(el, component)
  })
}
