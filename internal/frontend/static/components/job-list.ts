import { getPersisted, setPersisted } from "../lib/storage"
import { getData, nextFrame } from "../lib/dom"

interface HtmxWindow {
  htmx: {
    ajax: (method: string, url: string, options: { target: string | HTMLElement }) => void
  }
}

export class JobListComponent {
  private el: HTMLElement
  private repoId: string
  private activeLogUrl: string
  private selectedJob: string
  private jobButtons: HTMLElement[]

  constructor(el: HTMLElement) {
    this.el = el
    this.repoId = getData(el, "repo-id")
    this.activeLogUrl = getPersisted(`activeLogUrl-${this.repoId}`, "")
    this.selectedJob = getPersisted(`selectedJob-${this.repoId}`, "")
    this.jobButtons = Array.from(el.querySelectorAll<HTMLElement>("[data-job-name]"))

    this.bindEvents()
    this.init()
  }

  private bindEvents(): void {
    this.jobButtons.forEach((btn) => {
      btn.addEventListener("click", () => {
        const url = btn.getAttribute("hx-get") || ""
        const name = getData(btn, "job-name")
        this.selectJob(url, name)
      })
    })

    window.addEventListener("clear-selected-job", () => {
      this.selectedJob = ""
      this.activeLogUrl = ""
      setPersisted(`selectedJob-${this.repoId}`, "")
      setPersisted(`activeLogUrl-${this.repoId}`, "")
      this.updateUI()
    })
  }

  private async init(): Promise<void> {
    await nextFrame()
    if (this.activeLogUrl && (window as unknown as HtmxWindow).htmx) {
      ;(window as unknown as HtmxWindow).htmx.ajax("GET", this.activeLogUrl, {
        target: "#log-viewer"
      })
    }
    this.updateUI()
  }

  private selectJob(url: string, name: string): void {
    this.selectedJob = name
    this.activeLogUrl = url
    setPersisted(`selectedJob-${this.repoId}`, name)
    setPersisted(`activeLogUrl-${this.repoId}`, url)
    this.updateUI()
  }

  private updateUI(): void {
    this.jobButtons.forEach((btn) => {
      const name = getData(btn, "job-name")
      if (name === this.selectedJob) {
        btn.classList.add("!border-blue-400", "bg-gray-50")
      } else {
        btn.classList.remove("!border-blue-400", "bg-gray-50")
      }
    })

    const placeholder = this.el.querySelector<HTMLElement>('[data-role="placeholder"]')
    const logViewer = this.el.querySelector<HTMLElement>('[data-role="log-viewer"]')

    if (placeholder) {
      placeholder.style.display = this.selectedJob ? "none" : ""
      placeholder.classList.toggle("hidden", !!this.selectedJob)
    }
    if (logViewer) {
      logViewer.style.display = this.selectedJob ? "" : "none"
      logViewer.classList.toggle("hidden", !this.selectedJob)
    }
  }
}

export function initJobLists(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="job-list"]').forEach((el) => {
    new JobListComponent(el)
  })
}
