import { getPersisted, setPersisted } from "../lib/storage"
import { getData } from "../lib/dom"
import { animateDetailsElement } from "../lib/animation"

export class RenovatorDetailsComponent {
  private el: HTMLDetailsElement
  private persistKey: string
  private summary: HTMLElement
  private isAnimating = false

  constructor(el: HTMLDetailsElement) {
    this.el = el
    this.persistKey = getData(el, "persist-key")
    this.summary = el.querySelector("summary") as HTMLElement

    const stored = getPersisted<boolean>(this.persistKey, false)
    this.el.open = stored

    if (this.summary) {
      this.summary.addEventListener("click", (e) => this.handleClick(e))
    }
  }

  private handleClick(e: MouseEvent): void {
    e.preventDefault()

    if (this.isAnimating) {
      return
    }

    this.isAnimating = true

    if (this.el.open) {
      animateDetailsElement(this.el, false, () => {
        this.isAnimating = false
        setPersisted(this.persistKey, false)
        this.el.dispatchEvent(new CustomEvent("renovator-details-toggle", { bubbles: true }))
      })
    } else {
      animateDetailsElement(this.el, true, () => {
        this.isAnimating = false
        setPersisted(this.persistKey, true)
        this.el.dispatchEvent(new CustomEvent("renovator-details-toggle", { bubbles: true }))
      })
    }
  }
}

export function initRenovatorDetails(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="renovator-details"]').forEach((el) => {
    new RenovatorDetailsComponent(el as HTMLDetailsElement)
  })
}
