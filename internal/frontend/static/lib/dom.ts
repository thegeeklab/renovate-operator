export function getRefs(root: HTMLElement): Record<string, HTMLElement> {
  const refs: Record<string, HTMLElement> = {}
  root.querySelectorAll<HTMLElement>("[data-ref]").forEach((el) => {
    const name = el.getAttribute("data-ref")
    if (name) {
      refs[name] = el
    }
  })
  return refs
}

export function getData(el: Element, attr: string): string {
  return el.getAttribute(`data-${attr}`) || ""
}

export function getBoolData(el: Element, attr: string): boolean {
  return el.getAttribute(`data-${attr}`) === "true"
}

export function nextFrame(): Promise<void> {
  return new Promise((resolve) => requestAnimationFrame(() => resolve()))
}
