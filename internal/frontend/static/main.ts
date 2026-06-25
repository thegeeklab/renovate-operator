import "./style.css"
import htmx from "htmx.org"
import "htmx-ext-sse"
import { initHtmxHooks } from "./htmx.hooks"

declare global {
  interface Window {
    htmx: typeof htmx
  }
}

window.htmx = htmx
initHtmxHooks()
