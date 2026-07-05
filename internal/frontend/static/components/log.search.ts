import { getData } from "../lib/dom"
import { getPersisted, setPersisted } from "../lib/storage"
import { registerComponent } from "../lib/component.registry"

const DEBOUNCE_MS = 100
const HIGHLIGHT_CLASS = "log-search-mark"

export class LogSearch {
  private logViewer: HTMLElement | null
  private input: HTMLInputElement
  private countEl: HTMLElement | null
  private clearBtn: HTMLButtonElement | null
  private iconEl: HTMLElement | null
  private emptyEl: HTMLElement | null
  private storageKey: string
  private debounceTimer: number | null = null
  private boundInput: () => void
  private boundClear: () => void
  private boundKeydown: (e: KeyboardEvent) => void
  private boundUserToggle: (e: Event) => void

  constructor(el: HTMLElement) {
    this.logViewer = el.closest<HTMLElement>('[data-component="log-viewer"]')

    const namespace = this.logViewer ? getData(this.logViewer, "namespace") : ""
    const runner = this.logViewer ? getData(this.logViewer, "runner") : ""
    const jobName = this.logViewer ? getData(this.logViewer, "job-name") : ""
    this.storageKey = `logSearch-${namespace}-${runner}-${jobName}`

    this.input = el.querySelector<HTMLInputElement>('[data-role="log-search-input"]')!
    this.countEl = el.querySelector<HTMLElement>('[data-role="log-search-count"]')
    this.clearBtn = el.querySelector<HTMLButtonElement>('[data-action="log-search-clear"]')
    this.iconEl = el.querySelector<HTMLElement>('[data-role="log-search-icon"]')
    this.emptyEl = el.querySelector<HTMLElement>('[data-role="log-search-empty"]')

    this.boundInput = this.handleInput.bind(this)
    this.boundClear = this.handleClear.bind(this)
    this.boundKeydown = this.handleKeydown.bind(this)
    this.boundUserToggle = this.handleUserToggle.bind(this)

    this.input.addEventListener("input", this.boundInput)
    if (this.clearBtn) {
      this.clearBtn.addEventListener("click", this.boundClear)
    }
    this.input.addEventListener("keydown", this.boundKeydown)
    if (this.logViewer) {
      this.logViewer.addEventListener("click", this.boundUserToggle, true)
    }

    const stored = getPersisted<string>(this.storageKey, "")
    if (stored) {
      this.input.value = stored
      this.applyQuery(stored, { skipFocus: true })
    }
  }

  private handleInput(): void {
    const { value } = this.input
    if (this.debounceTimer !== null) {
      window.clearTimeout(this.debounceTimer)
    }
    this.debounceTimer = window.setTimeout(() => {
      this.applyQuery(value)
    }, DEBOUNCE_MS)
  }

  private handleClear(): void {
    this.input.value = ""
    this.applyQuery("")
    this.input.focus()
  }

  private handleKeydown(e: KeyboardEvent): void {
    if (e.key === "Escape" && this.input.value !== "") {
      e.preventDefault()
      e.stopPropagation()
      this.input.value = ""
      this.applyQuery("")
    }
  }

  private handleUserToggle(e: Event): void {
    const target = e.target as HTMLElement | null
    if (!target) return
    const line = target.closest<HTMLElement>(".log-line")
    if (!line) return
    const { action } = line.dataset
    if (action !== "toggle-raw") return
    line.setAttribute("data-user-toggled", "true")
  }

  private applyQuery(rawQuery: string, opts: { skipFocus?: boolean } = {}): void {
    const query = rawQuery.trim()

    if (query === "") {
      setPersisted(this.storageKey, "")
    } else {
      setPersisted(this.storageKey, query)
    }

    this.updateClearButton(query)
    this.unmarkAll()

    if (!this.logViewer) return

    const lines = Array.from(this.logViewer.querySelectorAll<HTMLElement>(".log-line"))
    const lcQuery = query.toLowerCase()
    let matchCount = 0

    for (const line of lines) {
      const message = line.dataset.message ?? ""
      const raw = line.dataset.raw ?? ""
      const messageMatches = query !== "" && message.toLowerCase().includes(lcQuery)
      const rawMatches = query !== "" && raw.toLowerCase().includes(lcQuery)
      const isMatch = query === "" || messageMatches || rawMatches

      if (isMatch) {
        line.classList.remove("log-search-hidden")
        matchCount++
      } else {
        line.classList.add("log-search-hidden")
      }

      const isDetailsOnlyMatch = query !== "" && isMatch && !messageMatches && rawMatches
      const userToggled = line.hasAttribute("data-user-toggled")

      this.applyExpansion(line, isDetailsOnlyMatch, userToggled, query)
    }

    if (query !== "" && matchCount > 0) {
      this.markAll(query)
    }

    this.updateCount(matchCount, lines.length, query)
    this.updateEmptyState(query, matchCount)

    if (query !== "" && !opts.skipFocus) {
      this.scrollFirstMatchIntoView()
    }
  }

  private applyExpansion(
    line: HTMLElement,
    isDetailsOnlyMatch: boolean,
    userToggled: boolean,
    query: string
  ): void {
    const rawContent = line.querySelector<HTMLElement>(".log-raw-content")
    const rawText = line.querySelector<HTMLElement>(".log-raw-text")
    const chevron = line.querySelector<HTMLElement>(".log-chevron")
    if (!rawContent || !rawText || !chevron) return

    if (query === "") {
      if (!userToggled) {
        if (!rawContent.classList.contains("hidden")) {
          rawContent.classList.add("hidden")
        }
        if (chevron.classList.contains("rotate-90")) {
          chevron.classList.remove("rotate-90")
        }
      }
      return
    }

    if (userToggled) return

    if (isDetailsOnlyMatch) {
      this.populateRawText(line, rawText)
      rawContent.classList.remove("hidden")
      chevron.classList.add("rotate-90")
    } else {
      rawContent.classList.add("hidden")
      chevron.classList.remove("rotate-90")
    }
  }

  private populateRawText(line: HTMLElement, rawText: HTMLElement): void {
    if (rawText.textContent && rawText.textContent.length > 0) return
    const raw = line.dataset.raw ?? ""
    try {
      const parsed = JSON.parse(raw)
      rawText.textContent = JSON.stringify(parsed, null, 2)
    } catch {
      rawText.textContent = raw
    }
  }

  private unmarkAll(): void {
    if (!this.logViewer) return
    const marks = this.logViewer.querySelectorAll<HTMLElement>(`mark.${HIGHLIGHT_CLASS}`)
    marks.forEach((mark) => {
      const parent = mark.parentNode
      if (!parent) return
      while (mark.firstChild) {
        parent.insertBefore(mark.firstChild, mark)
      }
      parent.removeChild(mark)
      parent.normalize()
    })
  }

  private markAll(query: string): void {
    if (!this.logViewer) return
    const regex = buildSearchRegex(query)
    if (!regex) return

    const targets = this.logViewer.querySelectorAll<HTMLElement>(
      ".log-line:not(.log-search-hidden) [data-role='log-line-message'], " +
        ".log-line:not(.log-search-hidden) .log-raw-text"
    )
    targets.forEach((target) => {
      this.highlightTextNodes(target, regex)
    })
  }

  private highlightTextNodes(root: HTMLElement, regex: RegExp): void {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
      acceptNode: (node) => {
        const parent = node.parentNode
        if (!parent || parent.nodeType !== Node.ELEMENT_NODE) return NodeFilter.FILTER_REJECT
        if (!node.nodeValue || node.nodeValue.trim() === "") return NodeFilter.FILTER_REJECT
        return regex.test(node.nodeValue) ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT
      }
    })

    const textNodes: Text[] = []
    let current = walker.nextNode()
    while (current) {
      textNodes.push(current as Text)
      current = walker.nextNode()
    }

    for (const textNode of textNodes) {
      this.wrapMatches(textNode, regex)
    }
  }

  private wrapMatches(textNode: Text, regex: RegExp): void {
    const text = textNode.nodeValue ?? ""
    regex.lastIndex = 0
    if (!regex.test(text)) return
    regex.lastIndex = 0

    const fragment = document.createDocumentFragment()
    let lastIndex = 0
    let match: RegExpExecArray | null
    while ((match = regex.exec(text)) !== null) {
      if (match.index > lastIndex) {
        fragment.appendChild(document.createTextNode(text.slice(lastIndex, match.index)))
      }
      const mark = document.createElement("mark")
      mark.className = HIGHLIGHT_CLASS
      mark.textContent = match[0]
      fragment.appendChild(mark)
      lastIndex = match.index + match[0].length
      if (match[0].length === 0) regex.lastIndex++
    }
    if (lastIndex < text.length) {
      fragment.appendChild(document.createTextNode(text.slice(lastIndex)))
    }

    const parent = textNode.parentNode
    if (!parent) return
    parent.replaceChild(fragment, textNode)
  }

  private updateClearButton(query: string): void {
    if (this.clearBtn) {
      this.clearBtn.classList.toggle("hidden", query === "")
    }
    if (this.iconEl) {
      this.iconEl.classList.toggle("hidden", query !== "")
    }
  }

  private updateCount(matchCount: number, totalCount: number, query: string): void {
    if (!this.countEl) return
    if (query === "") {
      this.countEl.classList.add("hidden")
      this.countEl.textContent = ""
      return
    }
    this.countEl.textContent = `${matchCount}/${totalCount}`
    this.countEl.classList.remove("hidden")
  }

  private updateEmptyState(query: string, matchCount: number): void {
    if (!this.emptyEl) return
    if (query !== "" && matchCount === 0) {
      this.emptyEl.classList.remove("hidden")
    } else {
      this.emptyEl.classList.add("hidden")
    }
  }

  private scrollFirstMatchIntoView(): void {
    if (!this.logViewer) return
    const first = this.logViewer.querySelector<HTMLElement>(".log-line:not(.log-search-hidden)")
    if (!first) return
    const scrollBox = this.logViewer.querySelector<HTMLElement>('[data-ref="scrollBox"]')
    if (!scrollBox) return
    const autoscrollBtn = this.logViewer.querySelector<HTMLElement>(
      '[data-action="toggle-autoscroll"]'
    )
    if (autoscrollBtn && autoscrollBtn.getAttribute("aria-pressed") === "true") return
    const firstTop = first.offsetTop
    const firstBottom = firstTop + first.offsetHeight
    const visibleTop = scrollBox.scrollTop
    const visibleBottom = visibleTop + scrollBox.clientHeight
    if (firstTop < visibleTop || firstBottom > visibleBottom) {
      scrollBox.scrollTo({ top: Math.max(0, firstTop - 8), behavior: "smooth" })
    }
  }

  destroy(): void {
    if (this.debounceTimer !== null) {
      window.clearTimeout(this.debounceTimer)
    }
    this.input.removeEventListener("input", this.boundInput)
    if (this.clearBtn) {
      this.clearBtn.removeEventListener("click", this.boundClear)
    }
    this.input.removeEventListener("keydown", this.boundKeydown)
    if (this.logViewer) {
      this.logViewer.removeEventListener("click", this.boundUserToggle, true)
    }
    this.unmarkAll()
  }
}

function buildSearchRegex(query: string): RegExp | null {
  const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
  if (escaped.length === 0) return null
  return new RegExp(escaped, "gi")
}

export function initLogSearches(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="log-search"]').forEach((el) => {
    const component = new LogSearch(el)
    registerComponent(el, component)
  })
}
