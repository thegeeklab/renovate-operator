import { getPersisted, setPersisted } from "../lib/storage"
import { getData, nextFrame } from "../lib/dom"
import { registerComponent, componentRegistry } from "../lib/component.registry"

export class JobListComponent {
  private el: HTMLElement
  private repoId: string
  private activeLogUrl: string
  private selectedJob: string
  private jobButtons: HTMLElement[]
  private clearSelectedJobHandler: () => void

  constructor(el: HTMLElement) {
    this.el = el
    this.repoId = getData(el, "repo-id")
    this.activeLogUrl = getPersisted(`activeLogUrl-${this.repoId}`, "")
    this.selectedJob = getPersisted(`selectedJob-${this.repoId}`, "")
    this.jobButtons = Array.from(el.querySelectorAll<HTMLElement>("button[data-job-name]"))

    this.clearSelectedJobHandler = () => {
      this.selectedJob = ""
      this.activeLogUrl = ""
      setPersisted(`selectedJob-${this.repoId}`, "")
      setPersisted(`activeLogUrl-${this.repoId}`, "")
      this.updateUI()
    }

    this.bindEvents()
    this.init()
  }

  destroy(): void {
    window.removeEventListener("clear-selected-job", this.clearSelectedJobHandler)
  }

  private bindEvents(): void {
    this.el.addEventListener("click", (e: MouseEvent) => {
      const btn = (e.target as HTMLElement).closest<HTMLElement>("button[data-job-name]")
      if (btn && this.el.contains(btn)) {
        const url = btn.getAttribute("hx-get") || ""
        const name = getData(btn, "job-name")
        this.selectJob(url, name)
      }
    })

    window.addEventListener("clear-selected-job", this.clearSelectedJobHandler)
  }

  private async init(): Promise<void> {
    if (this.selectedJob) {
      const stillExists = this.el.querySelector(
        `button[data-job-name="${CSS.escape(this.selectedJob)}"]`
      )
      if (!stillExists) {
        this.selectedJob = ""
        this.activeLogUrl = ""
        setPersisted(`selectedJob-${this.repoId}`, "")
        setPersisted(`activeLogUrl-${this.repoId}`, "")
      }
    }

    await nextFrame()
    if (this.activeLogUrl && window.htmx) {
      window.htmx.ajax("get", this.activeLogUrl, {
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

  refresh(): void {
    this.jobButtons = Array.from(this.el.querySelectorAll<HTMLElement>("button[data-job-name]"))
    this.updateUI()
  }
}

export function initJobLists(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="job-list"]').forEach((el) => {
    const component = new JobListComponent(el)
    registerComponent(el, component)
  })
}

export function destroyJobLists(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="job-list"]').forEach((el) => {
    const component = componentRegistry.get(el)
    if (component) {
      component.destroy()
      componentRegistry.delete(el)
    }
  })
}
