import {
  computePosition,
  flip,
  offset,
  shift,
  type Placement,
  type Strategy
} from "@floating-ui/dom"

export interface DropdownConfig {
  buttonSelector: string
  menuSelector: string
  placement?: Placement
  strategy?: Strategy
  offset?: number
  focusOnClose?: boolean
}

export class Dropdown {
  protected el: HTMLElement
  protected button: HTMLButtonElement
  protected menu: HTMLDivElement
  protected isOpen = false

  private cfg: Required<DropdownConfig>
  private boundButtonClick: (e: Event) => void
  private boundDocumentClick: (e: Event) => void
  private boundKeydown: (e: KeyboardEvent) => void
  private boundFocusOut: (e: FocusEvent) => void

  constructor(el: HTMLElement, config: DropdownConfig) {
    this.el = el
    this.cfg = {
      buttonSelector: config.buttonSelector,
      menuSelector: config.menuSelector,
      placement: config.placement ?? "bottom-start",
      strategy: config.strategy ?? "absolute",
      offset: config.offset ?? 4,
      focusOnClose: config.focusOnClose ?? false
    }
    this.button = el.querySelector<HTMLButtonElement>(this.cfg.buttonSelector)!
    this.menu = el.querySelector<HTMLDivElement>(this.cfg.menuSelector)!

    this.boundButtonClick = this.handleButtonClick.bind(this)
    this.boundDocumentClick = this.handleDocumentClick.bind(this)
    this.boundKeydown = this.handleKeydown.bind(this)
    this.boundFocusOut = this.handleFocusOut.bind(this)

    this.button.addEventListener("click", this.boundButtonClick)
    document.addEventListener("click", this.boundDocumentClick)
    document.addEventListener("keydown", this.boundKeydown)
    this.el.addEventListener("focusout", this.boundFocusOut)
  }

  private handleButtonClick(e: Event): void {
    e.stopPropagation()
    this.toggle()
  }

  private handleDocumentClick(e: Event): void {
    if (this.isOpen && !this.el.contains(e.target as Node)) {
      this.close()
    }
  }

  private handleKeydown(e: KeyboardEvent): void {
    if (e.key === "Escape" && this.isOpen) {
      e.preventDefault()
      this.close()
      if (this.cfg.focusOnClose) {
        this.button.focus()
      }
    }
  }

  private handleFocusOut(e: FocusEvent): void {
    if (this.isOpen && !this.el.contains(e.relatedTarget as Node)) {
      this.close()
    }
  }

  protected toggle(): void {
    if (this.isOpen) {
      this.close()
    } else {
      this.open()
    }
  }

  protected async open(): Promise<void> {
    this.isOpen = true
    this.menu.classList.remove("hidden")
    this.button.setAttribute("aria-expanded", "true")

    const { x, y } = await computePosition(this.button, this.menu, {
      placement: this.cfg.placement,
      strategy: this.cfg.strategy,
      middleware: [offset(this.cfg.offset), flip(), shift({ padding: 8 })]
    })

    const style: Record<string, string> = { left: `${x}px`, top: `${y}px` }
    if (this.cfg.strategy === "fixed") {
      style.position = "fixed"
    }
    Object.assign(this.menu.style, style)
  }

  protected close(): void {
    this.isOpen = false
    this.menu.classList.add("hidden")
    this.button.setAttribute("aria-expanded", "false")
  }

  destroy(): void {
    this.button.removeEventListener("click", this.boundButtonClick)
    document.removeEventListener("click", this.boundDocumentClick)
    document.removeEventListener("keydown", this.boundKeydown)
    this.el.removeEventListener("focusout", this.boundFocusOut)
  }
}
