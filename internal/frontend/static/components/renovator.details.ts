import { getPersisted, setPersisted } from "../lib/storage"
import { getData } from "../lib/dom"

export class RenovatorDetailsComponent {
  private el: HTMLDetailsElement
  private persistKey: string

  constructor(el: HTMLDetailsElement) {
    this.el = el
    this.persistKey = getData(el, "persist-key")

    const stored = getPersisted<boolean>(this.persistKey, false)
    this.el.open = stored

    this.el.addEventListener("toggle", () => {
      setPersisted(this.persistKey, this.el.open)
    })
  }
}

export function initRenovatorDetails(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="renovator-details"]').forEach((el) => {
    new RenovatorDetailsComponent(el as HTMLDetailsElement)
  })
}
