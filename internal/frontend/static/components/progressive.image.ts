import { registerComponent } from "../lib/component.registry"

export class ProgressiveImageComponent {
  private img: HTMLImageElement
  private fallback: HTMLElement
  private boundLoad: () => void
  private boundError: () => void

  constructor(el: HTMLElement) {
    this.img = el.querySelector("img")!
    this.fallback =
      el.querySelector("[data-fallback]") || (this.img.nextElementSibling as HTMLElement)

    this.boundLoad = () => this.onLoaded()
    this.boundError = () => this.onError()

    if (this.img.complete) {
      if (this.img.naturalWidth > 0) {
        this.onLoaded()
      } else {
        this.onError()
      }
    } else {
      this.img.addEventListener("load", this.boundLoad)
      this.img.addEventListener("error", this.boundError)
    }
  }

  destroy(): void {
    this.img.removeEventListener("load", this.boundLoad)
    this.img.removeEventListener("error", this.boundError)
  }

  private onLoaded(): void {
    this.img.classList.remove("hidden")
    this.fallback.classList.add("hidden")
  }

  private onError(): void {
    this.img.classList.add("hidden")
    this.fallback.classList.remove("hidden")
  }
}

export function initProgressiveImages(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="progressive-image"]').forEach((el) => {
    const component = new ProgressiveImageComponent(el)
    registerComponent(el, component)
  })
}
