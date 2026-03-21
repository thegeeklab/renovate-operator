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

    selectJob(name, namespace, runner) {
      this.selectedJob = name
      this.activeLogUrl = `/joblogs?namespace=${namespace}&runner=${runner}&job=${name}`
    },

    getSelectedClass(name) {
      if (this.selectedJob === name) {
        return "!border-blue-400 bg-gray-50"
      }
      return ""
    }
  }
})

Alpine.data("logViewer", function (jobName, isRunning) {
  return {
    autoscroll: this.$persist(false).as(`autoscroll-${jobName}`),
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
  const oldScrollBox =
    e.detail.target.querySelector('[x-ref="scrollBox"]') ||
    (e.detail.target.getAttribute("x-ref") === "scrollBox" ? e.detail.target : null)

  if (oldScrollBox && oldScrollBox.id) {
    scrollStates.set(oldScrollBox.id, oldScrollBox.scrollTop)
  }
})

document.addEventListener("htmx:afterSwap", (e) => {
  if (!e.detail.target.id) {
    return
  }

  const newScrollBox =
    document.getElementById(e.detail.target.id)?.querySelector('[x-ref="scrollBox"]') ||
    (e.detail.target.getAttribute("x-ref") === "scrollBox"
      ? document.getElementById(e.detail.target.id)
      : null)

  if (newScrollBox && window.Alpine) {
    window.Alpine.nextTick(() => {
      const data = window.Alpine.$data(newScrollBox)

      if (data && data.autoscroll && data.isRunning) {
        newScrollBox.scrollTop = newScrollBox.scrollHeight
      } else if (scrollStates.has(newScrollBox.id)) {
        newScrollBox.scrollTop = scrollStates.get(newScrollBox.id)
      }

      scrollStates.delete(newScrollBox.id)
    })
  }
})
