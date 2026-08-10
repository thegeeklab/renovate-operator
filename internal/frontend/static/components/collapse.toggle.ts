import { setPersisted } from "../lib/storage"
import { getData } from "../lib/dom"
import { animateDetailsElement } from "../lib/animation"
import { registerComponent } from "../lib/component.registry"

export class CollapseToggleComponent {
  private button: HTMLElement
  private collapseIcon: HTMLElement
  private expandIcon: HTMLElement
  private allExpanded: boolean
  private isAnimating = false
  private boundToggle: () => void
  private boundDetailsToggle: () => void

  constructor(el: HTMLElement) {
    this.button = el
    this.collapseIcon = el.querySelector("[data-collapse-icon]") as HTMLElement
    this.expandIcon = el.querySelector("[data-expand-icon]") as HTMLElement
    this.allExpanded = this.areAllExpanded()
    this.updateIcon()
    this.boundToggle = this.toggle.bind(this)
    this.boundDetailsToggle = () => {
      this.allExpanded = this.areAllExpanded()
      this.updateIcon()
    }
    this.button.addEventListener("click", this.boundToggle)
    document.addEventListener("renovator-details-toggle", this.boundDetailsToggle)
  }

  destroy(): void {
    this.button.removeEventListener("click", this.boundToggle)
    document.removeEventListener("renovator-details-toggle", this.boundDetailsToggle)
  }

  private getDetails(): HTMLDetailsElement[] {
    return Array.from(
      document.querySelectorAll<HTMLDetailsElement>('[data-component="renovator-details"]')
    )
  }

  private areAllExpanded(): boolean {
    return this.getDetails().every((d) => d.open)
  }

  private updateIcon(): void {
    if (this.allExpanded) {
      this.collapseIcon.classList.remove("hidden")
      this.expandIcon.classList.add("hidden")
      this.button.setAttribute("aria-pressed", "true")
    } else {
      this.collapseIcon.classList.add("hidden")
      this.expandIcon.classList.remove("hidden")
      this.button.setAttribute("aria-pressed", "false")
    }
  }

  private toggle(): void {
    if (this.isAnimating) {
      return
    }

    this.isAnimating = true
    this.allExpanded = !this.allExpanded
    const targetState = this.allExpanded
    const details = this.getDetails()
    let completed = 0

    const onComplete = (): void => {
      completed++
      if (completed >= details.length) {
        this.isAnimating = false
      }
    }

    for (const el of details) {
      const persistKey = getData(el, "persist-key")
      animateDetailsElement(el, targetState, () => {
        setPersisted(persistKey, targetState)
        onComplete()
      })
    }

    if (details.length === 0) {
      this.isAnimating = false
    }

    this.updateIcon()
  }
}

export function initCollapseToggles(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="collapse-toggle"]').forEach((el) => {
    const component = new CollapseToggleComponent(el)
    registerComponent(el, component)
  })
}
