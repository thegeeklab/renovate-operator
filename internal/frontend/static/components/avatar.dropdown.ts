import { Dropdown } from "../lib/dropdown"
import { registerComponent } from "../lib/component.registry"
import { setLocale } from "../lib/locale"

export class AvatarDropdown extends Dropdown {
  constructor(element: HTMLElement) {
    super(element, {
      buttonSelector: "[data-avatar-button]",
      menuSelector: "[data-avatar-menu]",
      placement: "bottom-end",
      offset: 8
    })

    this.bindLocale()
  }

  private bindLocale(): void {
    this.menu
      .querySelectorAll<HTMLSelectElement>('[data-action="change-locale"]')
      .forEach((select) => {
        select.addEventListener("change", () => {
          setLocale(select.value)
        })
      })
  }
}

export function initAvatarDropdown(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="avatar-dropdown"]').forEach((el) => {
    const component = new AvatarDropdown(el)
    registerComponent(el, component)
  })
}
