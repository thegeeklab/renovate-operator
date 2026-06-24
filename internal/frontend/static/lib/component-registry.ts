interface Destroyable {
  destroy(): void
}

export const componentRegistry = new WeakMap<HTMLElement, Destroyable>()

export function registerComponent(el: HTMLElement, component: Destroyable): void {
  componentRegistry.set(el, component)
}
