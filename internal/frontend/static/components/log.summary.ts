import { registerComponent } from "../lib/component.registry"

class LogSummaryComponent {
  private detailsContent: HTMLElement | null
  private chevron: HTMLElement | null
  private toggleBtn: HTMLElement | null
  private isOpen: boolean
  private boundToggle: () => void

  constructor(el: HTMLElement) {
    this.detailsContent = el.querySelector<HTMLElement>('[data-role="details-content"]')
    this.chevron = el.querySelector<HTMLElement>('[data-role="details-chevron"]')
    this.toggleBtn = el.querySelector<HTMLElement>('[data-action="toggle-details"]')
    this.isOpen = false
    this.boundToggle = this.toggle.bind(this)

    this.bindEvents()
  }

  private bindEvents(): void {
    if (this.toggleBtn) {
      this.toggleBtn.addEventListener("click", this.boundToggle)
    }
  }

  private toggle(): void {
    this.isOpen = !this.isOpen

    if (this.detailsContent) {
      if (this.isOpen) {
        this.detailsContent.classList.remove("hidden")
      } else {
        this.detailsContent.classList.add("hidden")
      }
    }

    if (this.chevron) {
      this.chevron.classList.toggle("rotate-90", this.isOpen)
    }

    if (this.toggleBtn) {
      this.toggleBtn.setAttribute("aria-expanded", String(this.isOpen))
    }
  }

  destroy(): void {
    if (this.toggleBtn) {
      this.toggleBtn.removeEventListener("click", this.boundToggle)
    }
  }
}

export function initLogSummaries(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="log-summary"]').forEach((el) => {
    const component = new LogSummaryComponent(el)
    registerComponent(el, component)
  })
}
