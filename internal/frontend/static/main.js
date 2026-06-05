import "./style.css"
import "./htmx.js"

import "htmx-ext-sse"

import Alpine from "alpinejs"
import persist from "@alpinejs/persist"

const { htmx } = window

window.Alpine = Alpine
Alpine.plugin(persist)

Alpine.data("jobList", function (repoId) {
  return {
    activeLogUrl: this.$persist("").as(`activeLogUrl-${repoId}`),
    selectedJob: this.$persist("").as(`selectedJob-${repoId}`),

    init() {
      this.$nextTick(() => {
        if (this.activeLogUrl && window.htmx) {
          htmx.ajax("GET", this.activeLogUrl, { target: "#log-viewer" })
        }
        this.updateSelectionStyles()
      })

      window.addEventListener("clear-selected-job", () => {
        this.selectedJob = ""
        this.activeLogUrl = ""
        this.updateSelectionStyles()
      })
    },

    selectJob(name, namespace, runner) {
      this.selectedJob = name
      this.activeLogUrl = `/joblogs?namespace=${namespace}&runner=${runner}&job=${name}`
      this.$nextTick(() => {
        this.updateSelectionStyles()
      })
    },

    getSelectedClass(name) {
      if (this.selectedJob === name) {
        return "!border-blue-400 bg-gray-50"
      }
      return ""
    },

    updateSelectionStyles() {
      const items = this.$el.querySelectorAll("[data-job-name]")
      items.forEach((item) => {
        const isSelected = item.dataset.jobName === this.selectedJob
        item.classList.toggle("!border-blue-400", isSelected)
        item.classList.toggle("bg-gray-50", isSelected)
      })
    }
  }
})

Alpine.data("logViewer", function (namespace, runner, jobName, isRunning) {
  return {
    autoscroll: this.$persist(false).as(`autoscroll-${namespace}-${runner}-${jobName}`),
    isRunning,

    init() {
      this.$nextTick(() => {
        if (this.autoscroll && this.isRunning) {
          this.$refs.scrollBox.scrollTop = this.$refs.scrollBox.scrollHeight
        }
      })
    },

    toggleAutoscroll() {
      this.autoscroll = !this.autoscroll
      if (this.autoscroll) {
        this.$nextTick(() => {
          this.$refs.scrollBox.scrollTop = this.$refs.scrollBox.scrollHeight
        })
      }
    },

    closeLogs() {
      const logViewer = document.getElementById("log-viewer")
      if (logViewer) {
        logViewer.innerHTML = ""
      }
      window.dispatchEvent(new CustomEvent("clear-selected-job"))
    }
  }
})

Alpine.start()

const scrollStates = new Map()

document.addEventListener("htmx:beforeSwap", (e) => {
  const { target } = e.detail
  const scrollBox = target.querySelector('[x-ref="scrollBox"]')
  if (scrollBox) {
    scrollStates.set(scrollBox.id, scrollBox.scrollTop)
  }
})

document.addEventListener("htmx:afterSwap", (e) => {
  if (scrollStates.size > 0) {
    requestAnimationFrame(() => {
      for (const [id, savedScroll] of scrollStates) {
        const scrollBox = document.getElementById(id)
        if (!scrollBox) continue

        const alpineEl = scrollBox.closest("[x-data]")
        if (alpineEl) {
          const data = Alpine.$data(alpineEl)
          if (data && data.autoscroll) {
            scrollBox.scrollTop = scrollBox.scrollHeight
            continue
          }
        }
        scrollBox.scrollTop = savedScroll
      }
      scrollStates.clear()
    })
  }

  const jobListEl = document.querySelector("[x-data^='jobList']")
  if (jobListEl) {
    const data = Alpine.$data(jobListEl)
    if (data && data.updateSelectionStyles) {
      requestAnimationFrame(() => data.updateSelectionStyles())
    }
  }

  const { target, xhr } = e.detail
  if (!target) return

  const boosted = xhr?.getResponseHeader("HX-Boosted") || target.closest("[hx-boost]")
  if (boosted || target.id === "dashboard-content") {
    const focusable = target.querySelector("h1, h2, h3, [tabindex='-1']")
    if (focusable) {
      focusable.setAttribute("tabindex", "-1")
      focusable.focus({ preventScroll: true })
    }
  } else if (target.id === "log-viewer") {
    target.setAttribute("tabindex", "-1")
    target.focus({ preventScroll: true })
  }
})
