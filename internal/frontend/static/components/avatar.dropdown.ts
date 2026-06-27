import { Dropdown } from "../lib/dropdown"
import { registerComponent } from "../lib/component.registry"

export class AvatarDropdown extends Dropdown {
  constructor(element: HTMLElement) {
    super(element, {
      buttonSelector: "[data-avatar-button]",
      menuSelector: "[data-avatar-menu]",
      placement: "bottom-end",
      offset: 8
    })
  }
}

export function initAvatarDropdown(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="avatar-dropdown"]').forEach((el) => {
    const component = new AvatarDropdown(el)
    registerComponent(el, component)
  })
}
