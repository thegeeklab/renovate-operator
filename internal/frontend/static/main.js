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
      })

      window.addEventListener("clear-selected-job", () => {
        this.selectedJob = ""
        this.activeLogUrl = ""
      })
    },

    selectJob(url, name) {
      this.selectedJob = name
      this.activeLogUrl = url
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
    },

    async downloadLog(url, filename) {
      try {
        const response = await fetch(url)
        if (!response.ok) {
          throw new Error("Failed to fetch log")
        }
        const blob = await response.blob()

        if ("showSaveFilePicker" in window) {
          const handle = await window.showSaveFilePicker({
            suggestedName: filename,
            types: [{ description: "Log file", accept: { "text/plain": [".log"] } }]
          })
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
        if (err.name !== "AbortError") {
          console.error("Download failed:", err)
        }
      }
    }
  }
})

Alpine.directive("tooltip", (el) => {
  el.addEventListener("mouseenter", () => {
    const r = el.getBoundingClientRect()
    el.style.setProperty("--tt-x", `${r.left + r.width / 2}px`)
    el.style.setProperty("--tt-y", `${r.top - 4}px`)
  })
})

Alpine.start()

const scrollStates = new Map()
let savedSearchSelection = null

document.addEventListener("htmx:beforeSwap", (e) => {
  const { target } = e.detail

  const scrollBox = target.querySelector('[x-ref="scrollBox"]')
  if (scrollBox) {
    scrollStates.set(scrollBox.id, scrollBox.scrollTop)
  }

  if (target.id === "dashboard-content") {
    const searchInput = target.querySelector('input[name="search"]')
    if (searchInput && document.activeElement === searchInput) {
      savedSearchSelection = {
        start: searchInput.selectionStart,
        end: searchInput.selectionEnd
      }
    } else {
      savedSearchSelection = null
    }
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

  const { target, xhr } = e.detail
  if (!target) return

  if (target.id === "dashboard-content") {
    const searchInput = target.querySelector('input[name="search"]')
    if (searchInput && savedSearchSelection !== null) {
      searchInput.focus({ preventScroll: true })
      searchInput.setSelectionRange(savedSearchSelection.start, savedSearchSelection.end)
      savedSearchSelection = null
      return
    }
  }

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
