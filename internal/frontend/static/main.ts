import "./style.css"
import htmx from "htmx.org"
import "htmx-ext-sse"
import { initHtmxHooks } from "./htmx.hooks"
import { initKeyboard } from "./lib/keyboard"
import { toast } from "./lib/toast"

declare global {
  interface Window {
    htmx: typeof htmx
    toast?: typeof toast
  }
}

window.htmx = htmx

if (import.meta.env.DEV) {
  window.toast = toast
}

initHtmxHooks()
initKeyboard()
