import { Dropdown } from "../lib/dropdown"
import { registerComponent } from "../lib/component.registry"
import { getData } from "../lib/dom"
import { getPersisted, setPersisted } from "../lib/storage"

const ALL_LEVELS = [10, 20, 30, 40, 50, 60]

export class LogLevelFilter extends Dropdown {
  private logViewer: Element | null
  private activeLevels: Set<number>
  private storageKey: string
  private boundCheckboxChanges: Map<HTMLInputElement, () => void> = new Map()

  constructor(el: HTMLElement) {
    super(el, {
      buttonSelector: '[data-action="toggle-level-filter"]',
      menuSelector: '[data-role="level-menu"]',
      placement: "bottom-start",
      strategy: "fixed",
      offset: 4,
      focusOnClose: true
    })

    this.logViewer = el.closest('[data-component="log-viewer"]')
    const namespace = this.logViewer ? getData(this.logViewer, "namespace") : ""
    const runner = this.logViewer ? getData(this.logViewer, "runner") : ""
    const jobName = this.logViewer ? getData(this.logViewer, "job-name") : ""
    this.storageKey = `logLevels-${namespace}-${runner}-${jobName}`

    const stored = getPersisted<number[] | null>(this.storageKey, null)
    this.activeLevels = new Set(stored !== null ? stored : ALL_LEVELS)

    this.syncCheckboxes()
    this.applyFilter()
    this.updateCount()
    this.bindCheckboxEvents()
  }

  private bindCheckboxEvents(): void {
    this.menu.querySelectorAll<HTMLInputElement>('input[type="checkbox"]').forEach((cb) => {
      const handler = () => this.handleLevelChange(cb)
      this.boundCheckboxChanges.set(cb, handler)
      cb.addEventListener("change", handler)
    })
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
    if (!this.logViewer) return

    this.logViewer.querySelectorAll<HTMLElement>(".log-line[data-level]").forEach((line) => {
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
    super.destroy()

    this.boundCheckboxChanges.forEach((handler, cb) => {
      cb.removeEventListener("change", handler)
    })

    this.boundCheckboxChanges.clear()
  }
}

export function initLogLevelFilters(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="log-level-filter"]').forEach((el) => {
    const component = new LogLevelFilter(el)
    registerComponent(el, component)
  })
}
