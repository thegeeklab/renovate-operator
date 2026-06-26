import { getPersisted, setPersisted } from "../lib/storage"
import { getRefs, getData, getBoolData, nextFrame } from "../lib/dom"

export class LogViewerComponent {
  private el: HTMLElement
  private autoscroll: boolean
  private isRunning: boolean
  private refs: Record<string, HTMLElement>

  constructor(el: HTMLElement) {
    this.el = el
    this.isRunning = getBoolData(el, "is-running")
    const key = `autoscroll-${getData(el, "namespace")}-${getData(el, "runner")}-${getData(el, "job-name")}`
    this.autoscroll = getPersisted(key, false)
    this.refs = getRefs(el)

    this.bindEvents()
    this.init()
  }

  private bindEvents(): void {
    this.el.querySelectorAll<HTMLElement>('[data-action="toggle-autoscroll"]').forEach((btn) => {
      btn.addEventListener("click", () => this.toggleAutoscroll())
    })

    this.el.querySelectorAll<HTMLElement>('[data-action="close-logs"]').forEach((btn) => {
      btn.addEventListener("click", () => this.closeLogs())
    })

    this.el.querySelectorAll<HTMLElement>('[data-action="download-log"]').forEach((btn) => {
      btn.addEventListener("click", () => {
        const url = getData(btn, "url")
        const filename = getData(btn, "filename")
        this.downloadLog(url, filename)
      })
    })

    this.el.querySelectorAll<HTMLElement>('[data-action="toggle-raw"]').forEach((line) => {
      line.addEventListener("click", () => this.toggleRawLine(line))
    })
  }

  private async init(): Promise<void> {
    this.updateAutoscrollUI()
    await nextFrame()
    if (this.autoscroll && this.isRunning && this.refs.scrollBox) {
      this.refs.scrollBox.scrollTop = this.refs.scrollBox.scrollHeight
    }
  }

  private toggleAutoscroll(): void {
    this.autoscroll = !this.autoscroll
    const key = `autoscroll-${getData(this.el, "namespace")}-${getData(this.el, "runner")}-${getData(this.el, "job-name")}`
    setPersisted(key, this.autoscroll)
    this.updateAutoscrollUI()

    if (this.autoscroll && this.refs.scrollBox) {
      nextFrame().then(() => {
        if (this.refs.scrollBox) {
          this.refs.scrollBox.scrollTop = this.refs.scrollBox.scrollHeight
        }
      })
    }
  }

  private updateAutoscrollUI(): void {
    const iconOn = this.el.querySelector<HTMLElement>('[data-role="autoscroll-icon-on"]')
    const iconOff = this.el.querySelector<HTMLElement>('[data-role="autoscroll-icon-off"]')
    const label = this.el.querySelector<HTMLElement>('[data-role="autoscroll-label"]')
    const toggleBtn = this.el.querySelector<HTMLElement>('[data-action="toggle-autoscroll"]')

    if (iconOn) iconOn.style.display = this.autoscroll ? "" : "none"
    if (iconOff) iconOff.style.display = this.autoscroll ? "none" : ""
    if (label) label.textContent = this.autoscroll ? "Auto-scroll enabled" : "Auto-scroll disabled"
    if (toggleBtn) toggleBtn.setAttribute("aria-pressed", String(this.autoscroll))
  }

  private closeLogs(): void {
    const logViewer = document.getElementById("log-viewer")
    if (logViewer) {
      logViewer.innerHTML = ""
    }
    window.dispatchEvent(new CustomEvent("clear-selected-job"))
  }

  private toggleRawLine(line: HTMLElement): void {
    const rawContent = line.querySelector<HTMLElement>(".log-raw-content")
    const rawText = line.querySelector<HTMLElement>(".log-raw-text")
    const chevron = line.querySelector<HTMLElement>(".log-chevron")
    if (!rawContent || !rawText || !chevron) return

    const isExpanded = !rawContent.classList.contains("hidden")

    if (!isExpanded) {
      const raw = getData(line, "raw")
      try {
        const parsed = JSON.parse(raw)
        rawText.textContent = JSON.stringify(parsed, null, 2)
      } catch {
        rawText.textContent = raw
      }
    }

    rawContent.classList.toggle("hidden")
    chevron.style.transform = isExpanded ? "rotate(0deg)" : "rotate(90deg)"
  }

  private async downloadLog(url: string, filename: string): Promise<void> {
    try {
      const response = await fetch(url)
      if (!response.ok) {
        throw new Error("Failed to fetch log")
      }
      const blob = await response.blob()

      if ("showSaveFilePicker" in window) {
        const { showSaveFilePicker } = window as unknown as {
          showSaveFilePicker: (options: unknown) => Promise<unknown>
        }
        const handle = (await showSaveFilePicker.call(window, {
          suggestedName: filename,
          types: [{ description: "Log file", accept: { "text/plain": [".log"] } }]
        })) as FileSystemFileHandle
        const writable = await handle.createWritable()
        await writable.write(blob)
        await writable.close()
      } else {
        const objectUrl = URL.createObjectURL(blob)
        const a = document.createElement("a")
        a.href = objectUrl
        a.download = filename
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        URL.revokeObjectURL(objectUrl)
      }
    } catch (err) {
      if (err instanceof Error && err.name !== "AbortError") {
        console.error("Download failed:", err)
      }
    }
  }
}

export function initLogViewers(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="log-viewer"]').forEach((el) => {
    new LogViewerComponent(el)
  })
}
