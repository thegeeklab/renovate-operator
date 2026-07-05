export function populateRawText(line: HTMLElement): void {
  const rawText = line.querySelector<HTMLElement>(".log-raw-text")
  if (!rawText) return

  if (rawText.textContent && rawText.textContent.length > 0) return

  const raw = line.dataset.raw ?? ""
  try {
    const parsed = JSON.parse(raw)
    rawText.textContent = JSON.stringify(parsed, null, 2)
  } catch {
    rawText.textContent = raw
  }
}

export function setRawExpanded(line: HTMLElement, expanded: boolean): void {
  const rawContent = line.querySelector<HTMLElement>(".log-raw-content")
  const chevron = line.querySelector<HTMLElement>(".log-chevron")
  if (!rawContent || !chevron) return

  if (expanded) {
    populateRawText(line)
    rawContent.classList.remove("hidden")
    chevron.classList.add("rotate-90")
  } else {
    rawContent.classList.add("hidden")
    chevron.classList.remove("rotate-90")
  }
}

export function toggleRawLine(line: HTMLElement): void {
  const rawContent = line.querySelector<HTMLElement>(".log-raw-content")
  if (!rawContent) return
  setRawExpanded(line, rawContent.classList.contains("hidden"))
}
