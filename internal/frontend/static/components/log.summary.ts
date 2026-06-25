import { registerComponent } from "../lib/component.registry"

class LogSummaryComponent {
  private detailsContent: HTMLElement | null
  private chevron: HTMLElement | null
  private toggleBtn: HTMLElement | null
  private isOpen: boolean

  constructor(el: HTMLElement) {
    this.detailsContent = el.querySelector<HTMLElement>('[data-role="details-content"]')
    this.chevron = el.querySelector<HTMLElement>('[data-role="details-chevron"]')
    this.toggleBtn = el.querySelector<HTMLElement>('[data-action="toggle-details"]')
    this.isOpen = false

    this.bindEvents()
  }

  private bindEvents(): void {
    if (this.toggleBtn) {
      this.toggleBtn.addEventListener("click", () => this.toggle())
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
      if (this.isOpen) {
        this.chevron.style.transform = "rotate(90deg)"
      } else {
        this.chevron.style.transform = "rotate(0deg)"
      }
    }

    if (this.toggleBtn) {
      this.toggleBtn.setAttribute("aria-expanded", String(this.isOpen))
    }
  }

  destroy(): void {
    // Cleanup if needed
  }
}

export function initLogSummaries(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="log-summary"]').forEach((el) => {
    const component = new LogSummaryComponent(el)
    registerComponent(el, component)
  })
}
