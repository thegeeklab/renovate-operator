import { getPersisted, setPersisted } from "../lib/storage"
import { getData, getBoolData, nextFrame } from "../lib/dom"
import { registerComponent, destroyComponents } from "../lib/component.registry"
import { toggleRawLine } from "../lib/log.raw"
import { toast } from "../lib/toast"
import { t } from "../lib/i18n"

export class LogViewerComponent {
  private el: HTMLElement
  private autoscroll: boolean
  private isRunning: boolean

  private boundClick: (e: Event) => void

  constructor(el: HTMLElement) {
    this.el = el
    this.isRunning = getBoolData(el, "is-running")
    const key = `autoscroll-${getData(el, "namespace")}-${getData(el, "runner")}-${getData(el, "job-name")}`
    this.autoscroll = getPersisted(key, false)

    this.boundClick = this.handleClick.bind(this)

    this.el.addEventListener("click", this.boundClick)
    this.init()
  }

  private handleClick(e: Event): void {
    const target = e.target as HTMLElement | null
    if (!target) return
    const actionEl = target.closest<HTMLElement>("[data-action]")
    if (!actionEl || !this.el.contains(actionEl)) return

    switch (actionEl.dataset.action) {
      case "toggle-autoscroll":
        this.toggleAutoscroll()
        break
      case "close-logs":
        this.closeLogs()
        break
      case "download-log":
        this.downloadLog(getData(actionEl, "url"), getData(actionEl, "filename"))
        break
      case "toggle-raw":
        toggleRawLine(actionEl)
        break
    }
  }

  destroy(): void {
    this.el.removeEventListener("click", this.boundClick)
  }

  private getScrollBox(): HTMLElement | null {
    return this.el.querySelector<HTMLElement>('[data-ref="scrollBox"]')
  }

  private async init(): Promise<void> {
    this.updateAutoscrollUI()
    await nextFrame()
    const scrollBox = this.getScrollBox()
    if (this.autoscroll && this.isRunning && scrollBox) {
      scrollBox.scrollTop = scrollBox.scrollHeight
    }
  }

  private toggleAutoscroll(): void {
    this.autoscroll = !this.autoscroll
    const key = `autoscroll-${getData(this.el, "namespace")}-${getData(this.el, "runner")}-${getData(this.el, "job-name")}`
    setPersisted(key, this.autoscroll)
    this.updateAutoscrollUI()

    if (this.autoscroll) {
      nextFrame().then(() => {
        const scrollBox = this.getScrollBox()
        if (scrollBox) {
          scrollBox.scrollTop = scrollBox.scrollHeight
        }
      })
    }
  }

  private updateAutoscrollUI(): void {
    const iconOn = this.el.querySelector<HTMLElement>('[data-role="autoscroll-icon-on"]')
    const iconOff = this.el.querySelector<HTMLElement>('[data-role="autoscroll-icon-off"]')
    const tooltipText = this.el.querySelector<HTMLElement>(
      '[data-role="autoscroll"] .tooltip-text span:first-child'
    )
    const toggleBtn = this.el.querySelector<HTMLElement>('[data-action="toggle-autoscroll"]')

    if (iconOn) iconOn.classList.toggle("hidden", !this.autoscroll)
    if (iconOff) iconOff.classList.toggle("hidden", this.autoscroll)
    if (tooltipText)
      tooltipText.textContent = this.autoscroll
        ? t("log.autoscroll_enabled")
        : t("log.autoscroll_disabled")
    if (toggleBtn) toggleBtn.setAttribute("aria-pressed", String(this.autoscroll))
  }

  private closeLogs(): void {
    const logViewer = document.getElementById("log-viewer")
    if (logViewer) {
      destroyComponents(logViewer)
      logViewer.innerHTML = ""
    }
    window.dispatchEvent(new CustomEvent("clear-selected-job"))
  }

  private async downloadLog(url: string, filename: string): Promise<void> {
    try {
      const response = await fetch(url)
      if (!response.ok) {
        throw new Error(t("log.failed_fetch_log"))
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
        toast.error(t("log.failed_download_log"))
      }
    }
  }
}

export function initLogViewers(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="log-viewer"]').forEach((el) => {
    const component = new LogViewerComponent(el)
    registerComponent(el, component)
  })
}
