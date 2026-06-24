import { initTooltips, hideActiveTooltip } from "./lib/tooltip"
import { initJobLists } from "./components/job-list"
import { initLogViewers } from "./components/log-viewer"
import { initRenovatorDetails } from "./components/renovator-details"
import { initRepoSorts } from "./components/repo-sort"

const scrollStates = new Map<string, number>()
let savedSearchSelection: { start: number; end: number } | null = null

function initComponents(root: ParentNode): void {
  initJobLists(root)
  initLogViewers(root)
  initRenovatorDetails(root)
  initRepoSorts(root)
  initTooltips(root)
  removeCloak(root)
}

function removeCloak(root: ParentNode): void {
  root.querySelectorAll<HTMLElement>("[data-cloak]").forEach((el) => {
    el.removeAttribute("data-cloak")
  })
}

export function initHtmxHooks(): void {
  document.addEventListener("htmx:configRequest", (e: Event) => {
    const { detail } = e as CustomEvent
    const searchInput = (e.target as HTMLElement).closest(
      'input[name="search"]'
    ) as HTMLInputElement | null
    if (searchInput && searchInput.value === "") {
      detail.path = "/"
      delete detail.parameters.search
    }
  })

  document.addEventListener("htmx:beforeSwap", (e: Event) => {
    const { detail } = e as CustomEvent
    const target = detail.target as HTMLElement

    const scrollBox = target.querySelector<HTMLElement>('[data-ref="scrollBox"]')
    if (scrollBox) {
      scrollStates.set(scrollBox.id, scrollBox.scrollTop)
    }

    if (target.id === "dashboard-content") {
      const searchInput = target.querySelector<HTMLInputElement>('input[name="search"]')
      if (searchInput && document.activeElement === searchInput) {
        savedSearchSelection = {
          start: searchInput.selectionStart ?? 0,
          end: searchInput.selectionEnd ?? 0
        }
      } else {
        savedSearchSelection = null
      }
    }
  })

  document.addEventListener("htmx:afterSettle", (e: Event) => {
    const { detail } = e as CustomEvent
    const target = detail.target as HTMLElement | null
    if (!target) return

    if (target.id === "job-list-container") {
      const jobListEl = document.querySelector<HTMLElement>('[data-component="job-list"]')
      if (jobListEl) {
        const selectedJob = jobListEl.querySelector<HTMLElement>(".\\!border-blue-400")
        if (selectedJob) {
          const jobName = selectedJob.getAttribute("data-job-name")
          if (jobName) {
            const stillExists = document.querySelector(
              `#job-list-container [data-job-name="${jobName}"]`
            )
            if (!stillExists) {
              window.dispatchEvent(new CustomEvent("clear-selected-job"))
            }
          }
        }
      }
    }
  })

  document.addEventListener("htmx:afterSwap", (e: Event) => {
    const { detail } = e as CustomEvent
    const target = detail.target as HTMLElement
    const xhr = detail.xhr as XMLHttpRequest | undefined
    if (!target) return

    hideActiveTooltip()

    if (scrollStates.size > 0) {
      requestAnimationFrame(() => {
        for (const [id, savedScroll] of scrollStates) {
          const scrollBox = document.getElementById(id)
          if (!scrollBox) continue

          const logViewerEl = scrollBox.closest('[data-component="log-viewer"]')
          if (logViewerEl) {
            const autoscrollBtn = logViewerEl.querySelector('[data-action="toggle-autoscroll"]')
            if (autoscrollBtn && autoscrollBtn.getAttribute("aria-pressed") === "true") {
              scrollBox.scrollTop = scrollBox.scrollHeight
              continue
            }
          }
          scrollBox.scrollTop = savedScroll
        }
        scrollStates.clear()
      })
    }

    if (target.id === "dashboard-content") {
      const searchInput = target.querySelector<HTMLInputElement>('input[name="search"]')
      if (searchInput && savedSearchSelection !== null) {
        searchInput.focus({ preventScroll: true })
        searchInput.setSelectionRange(savedSearchSelection.start, savedSearchSelection.end)
        savedSearchSelection = null
        return
      }
    }

    const boosted = xhr?.getResponseHeader("HX-Boosted") || target.closest("[hx-boost]")
    if (boosted || target.id === "dashboard-content") {
      const focusable =
        target.querySelector<HTMLElement>("[data-focus-target]") ||
        target.querySelector<HTMLElement>("h1, h2, [tabindex='-1']")
      if (focusable) {
        focusable.setAttribute("tabindex", "-1")
        focusable.focus({ preventScroll: true })
      }
    } else if (target.id === "log-viewer") {
      target.setAttribute("tabindex", "-1")
      target.focus({ preventScroll: true })
    }

    initComponents(target as ParentNode)
  })

  initComponents(document)
}
