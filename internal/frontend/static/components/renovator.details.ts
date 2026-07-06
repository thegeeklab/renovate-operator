import { getPersisted, setPersisted } from "../lib/storage"
import { getData } from "../lib/dom"

export class RenovatorDetailsComponent {
  private el: HTMLDetailsElement
  private persistKey: string
  private summary: HTMLElement
  private isAnimating = false
  private animationDurationMs: number

  constructor(el: HTMLDetailsElement) {
    this.el = el
    this.persistKey = getData(el, "persist-key")
    this.summary = el.querySelector("summary") as HTMLElement
    this.animationDurationMs = this.resolveAnimationDuration()

    const stored = getPersisted<boolean>(this.persistKey, false)
    this.el.open = stored

    if (this.summary) {
      this.summary.addEventListener("click", (e) => this.handleClick(e))
    }
  }

  private resolveAnimationDuration(): number {
    const raw = getComputedStyle(this.el).getPropertyValue("--details-animation-duration").trim()
    if (raw.endsWith("ms")) {
      return Number.parseInt(raw.slice(0, -2), 10) || 300
    }
    return 300
  }

  private handleClick(e: MouseEvent): void {
    e.preventDefault()

    if (this.isAnimating) {
      return
    }

    this.isAnimating = true

    if (this.el.open) {
      this.el.classList.add("animating-closed")
      setTimeout(() => {
        this.el.open = false
        this.el.classList.remove("animating-closed")
        this.isAnimating = false
        setPersisted(this.persistKey, false)
      }, this.animationDurationMs)
    } else {
      this.el.open = true
      setPersisted(this.persistKey, true)
      this.isAnimating = false
    }
  }
}

export function initRenovatorDetails(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="renovator-details"]').forEach((el) => {
    new RenovatorDetailsComponent(el as HTMLDetailsElement)
  })
}
