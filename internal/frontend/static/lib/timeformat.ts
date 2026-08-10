import { getLocale } from "./locale"

const MS_SECOND = 1000
const MS_MINUTE = 60 * MS_SECOND
const MS_HOUR = 60 * MS_MINUTE
const MS_DAY = 24 * MS_HOUR
const MS_MONTH = 30 * MS_DAY
const MS_YEAR = 365 * MS_DAY

function formatAbsolute(
  date: Date,
  locale: string,
  style: "short" | "medium" | "long" = "medium"
): string {
  return new Intl.DateTimeFormat(locale, {
    dateStyle: style,
    timeStyle: style === "short" ? undefined : "short"
  }).format(date)
}

function formatRelative(date: Date, locale: string): string {
  const diff = date.getTime() - Date.now()
  const absDiff = Math.abs(diff)

  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" })

  if (absDiff < MS_MINUTE) {
    return rtf.format(Math.round(diff / MS_SECOND), "second")
  }

  if (absDiff < MS_HOUR) {
    return rtf.format(Math.round(diff / MS_MINUTE), "minute")
  }

  if (absDiff < 90 * MS_MINUTE) {
    return rtf.format(Math.round(diff / MS_HOUR), "hour")
  }

  if (absDiff < 25 * MS_DAY) {
    return rtf.format(Math.round(diff / MS_DAY), "day")
  }

  if (absDiff < 45 * MS_DAY) {
    return rtf.format(Math.round(diff / MS_MONTH), "month")
  }

  if (absDiff < 13 * MS_MONTH) {
    return rtf.format(Math.round(diff / MS_MONTH), "month")
  }

  if (absDiff < 18 * MS_MONTH) {
    return rtf.format(Math.round(diff / MS_YEAR), "year")
  }

  return rtf.format(Math.round(diff / MS_YEAR), "year")
}

function formatElement(el: HTMLElement, locale: string): void {
  const timestamp = el.getAttribute("data-timestamp")
  if (!timestamp) return

  const date = new Date(timestamp)
  if (isNaN(date.getTime())) return

  const format = el.getAttribute("data-format") || "relative"

  switch (format) {
    case "relative":
      el.textContent = formatRelative(date, locale)
      break
    case "short":
      el.textContent = formatAbsolute(date, locale, "short")
      break
    case "long":
      el.textContent = formatAbsolute(date, locale, "long")
      break
    case "medium":
    default:
      el.textContent = formatAbsolute(date, locale, "medium")
      break
  }
}

export function initTimeFormat(root: ParentNode = document): void {
  const locale = getLocale()

  root.querySelectorAll<HTMLElement>("[data-timestamp]").forEach((el) => {
    formatElement(el, locale)
  })
}
