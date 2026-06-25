import { computePosition, flip, offset, shift } from "@floating-ui/dom"

export class AvatarDropdown {
  private element: HTMLElement
  private button: HTMLButtonElement
  private menu: HTMLDivElement
  private isOpen = false

  constructor(element: HTMLElement) {
    this.element = element
    this.button = element.querySelector("[data-avatar-button]")!
    this.menu = element.querySelector("[data-avatar-menu]")!

    this.button.addEventListener("click", this.handleButtonClick.bind(this))
    document.addEventListener("click", this.handleDocumentClick.bind(this))
    document.addEventListener("keydown", this.handleKeydown.bind(this))
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
    this.button.removeEventListener("click", this.handleButtonClick.bind(this))
    document.removeEventListener("click", this.handleDocumentClick.bind(this))
    document.removeEventListener("keydown", this.handleKeydown.bind(this))
  }
}

export function initAvatarDropdown(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="avatar-dropdown"]').forEach((el) => {
    new AvatarDropdown(el)
  })
}
