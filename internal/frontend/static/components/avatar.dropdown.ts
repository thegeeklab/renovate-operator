import { computePosition, flip, offset, shift } from "@floating-ui/dom"
import { registerComponent } from "../lib/component.registry"

export class AvatarDropdown {
  private element: HTMLElement
  private button: HTMLButtonElement
  private menu: HTMLDivElement
  private isOpen = false

  private boundHandleButtonClick: (e: Event) => void
  private boundHandleDocumentClick: (e: Event) => void
  private boundHandleKeydown: (e: KeyboardEvent) => void

  constructor(element: HTMLElement) {
    this.element = element
    this.button = element.querySelector("[data-avatar-button]")!
    this.menu = element.querySelector("[data-avatar-menu]")!

    this.boundHandleButtonClick = this.handleButtonClick.bind(this)
    this.boundHandleDocumentClick = this.handleDocumentClick.bind(this)
    this.boundHandleKeydown = this.handleKeydown.bind(this)

    this.button.addEventListener("click", this.boundHandleButtonClick)
    document.addEventListener("click", this.boundHandleDocumentClick)
    document.addEventListener("keydown", this.boundHandleKeydown)
  }

  private handleButtonClick(e: Event) {
    e.stopPropagation()
    this.toggle()
  }

  private handleDocumentClick(e: Event) {
    if (this.isOpen && !this.element.contains(e.target as Node)) {
      this.close()
    }
  }

  private handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape" && this.isOpen) {
      this.close()
    }
  }

  private toggle() {
    if (this.isOpen) {
      this.close()
    } else {
      this.open()
    }
  }

  private async open() {
    this.isOpen = true
    this.menu.classList.remove("hidden")
    this.button.setAttribute("aria-expanded", "true")

    const { x, y } = await computePosition(this.button, this.menu, {
      placement: "bottom-end",
      middleware: [offset(8), flip(), shift({ padding: 8 })]
    })

    Object.assign(this.menu.style, {
      left: `${x}px`,
      top: `${y}px`
    })
  }

  private close() {
    this.isOpen = false
    this.menu.classList.add("hidden")
    this.button.setAttribute("aria-expanded", "false")
  }

  destroy() {
    this.button.removeEventListener("click", this.boundHandleButtonClick)
    document.removeEventListener("click", this.boundHandleDocumentClick)
    document.removeEventListener("keydown", this.boundHandleKeydown)
  }
}

export function initAvatarDropdown(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="avatar-dropdown"]').forEach((el) => {
    const component = new AvatarDropdown(el)
    registerComponent(el, component)
  })
}
