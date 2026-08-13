export function getData(el: Element, attr: string): string {
  return el.getAttribute(`data-${attr}`) || ""
}

export function getBoolData(el: Element, attr: string): boolean {
  return el.getAttribute(`data-${attr}`) === "true"
}

export function nextFrame(): Promise<void> {
  return new Promise((resolve) => requestAnimationFrame(() => resolve()))
}
