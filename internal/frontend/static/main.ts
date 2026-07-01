import "./style.css"
import htmx from "htmx.org"
import "htmx-ext-sse"
import { initHtmxHooks } from "./htmx.hooks"
import { initKeyboard } from "./lib/keyboard"

declare global {
  interface Window {
    htmx: typeof htmx
  }
}

window.htmx = htmx
initHtmxHooks()
initKeyboard()
