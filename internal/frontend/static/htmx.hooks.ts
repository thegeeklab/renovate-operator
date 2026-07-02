import { initTooltips, hideActiveTooltip } from "./lib/tooltip"
import { initJobLists } from "./components/job.list"
import { initLogViewers } from "./components/log.viewer"
import { initLogSummaries } from "./components/log.summary"
import { initLogLevelFilters } from "./components/log.level.filter"
import { initRenovatorDetails } from "./components/renovator.details"
import { initRepoSorts } from "./components/repo.sort"
import { initAvatarDropdown } from "./components/avatar.dropdown"
import { initProgressiveImages } from "./components/progressive.image"
import { componentRegistry, destroyComponents } from "./lib/component.registry"
import { getPersisted } from "./lib/storage"

const scrollStates = new Map<string, number>()
let savedSearchSelection: { start: number; end: number } | null = null
let savedJobListFocus: string | null = null

function initComponents(root: ParentNode): void {
  initJobLists(root)
  initLogViewers(root)
  initLogSummaries(root)
  initLogLevelFilters(root)
  initRenovatorDetails(root)
  initRepoSorts(root)
  initAvatarDropdown(root)
  initProgressiveImages(root)
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
    const target = e.target as HTMLElement
    const searchInput = target.closest('input[name="search"]') as HTMLInputElement | null
    if (searchInput && searchInput.value === "") {
      detail.path = "/"
      delete detail.parameters.search
    }
  })

  document.addEventListener("htmx:beforeSwap", (e: Event) => {
    const { detail } = e as CustomEvent
    const target = detail.target as HTMLElement

    destroyComponents(target)

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

    if (target.id === "job-list-container") {
      const focused = document.activeElement as HTMLElement | null
      if (focused && focused.hasAttribute("data-job-name")) {
        savedJobListFocus = focused.getAttribute("data-job-name")
      } else {
        savedJobListFocus = null
      }
    }
  })

  document.addEventListener("htmx:afterSettle", (e: Event) => {
    const { detail } = e as CustomEvent
    const target = detail.target as HTMLElement | null
    if (!target || target.id !== "job-list-container") return

    const jobListEl = document.querySelector<HTMLElement>('[data-component="job-list"]')
    if (!jobListEl) return

    const repoId = jobListEl.getAttribute("data-repo-id")
    if (!repoId) return

    const selectedJob = getPersisted(`selectedJob-${repoId}`, "")
    if (!selectedJob) return

    const currentContainer = document.getElementById("job-list-container")
    if (!currentContainer) return

    const stillExists = currentContainer.querySelector(
      `button[data-job-name="${CSS.escape(selectedJob)}"]`
    )
    if (!stillExists) {
      window.dispatchEvent(new CustomEvent("clear-selected-job"))
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

    let focusRestored = false
    if (target.id === "dashboard-content") {
      const searchInput = target.querySelector<HTMLInputElement>('input[name="search"]')
      if (searchInput && savedSearchSelection !== null) {
        searchInput.focus({ preventScroll: true })
        searchInput.setSelectionRange(savedSearchSelection.start, savedSearchSelection.end)
        savedSearchSelection = null
        focusRestored = true
      }
    }

    if (!focusRestored) {
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
    }

    if (target.isConnected) {
      initComponents(target as ParentNode)
    }

    // After outerHTML swap, the target is detached. Initialize components on the live DOM.
    if (target.getAttribute("data-component") === "log-viewer") {
      const jobId = target.id
      const newLogViewer = document.getElementById(jobId)
      if (newLogViewer) {
        const parent = newLogViewer.parentElement
        if (parent) {
          initComponents(parent)
        }
      }
    }

    if (target.id === "job-list-container") {
      const newContainer = document.getElementById("job-list-container")
      if (newContainer) {
        initComponents(newContainer)
        const jobListEl = document.querySelector<HTMLElement>('[data-component="job-list"]')
        if (jobListEl) {
          const component = componentRegistry.get(jobListEl)
          if (component && "refresh" in component) {
            ;(component as { refresh: () => void }).refresh()
          }
        }

        if (savedJobListFocus !== null) {
          const focusedBtn = newContainer.querySelector<HTMLElement>(
            `button[data-job-name="${CSS.escape(savedJobListFocus)}"]`
          )
          if (focusedBtn) {
            focusedBtn.focus({ preventScroll: true })
          }
          savedJobListFocus = null
        }
      }
    }
  })

  initComponents(document)
}
