interface Destroyable {
  destroy(): void
}

export const componentRegistry = new WeakMap<HTMLElement, Destroyable>()

export function registerComponent(el: HTMLElement, component: Destroyable): void {
  componentRegistry.set(el, component)
}

export function destroyComponents(root: ParentNode): void {
  root.querySelectorAll<HTMLElement>("[data-component]").forEach((el) => {
    const component = componentRegistry.get(el)
    if (component) {
      component.destroy()
      componentRegistry.delete(el)
    }
  })
}
