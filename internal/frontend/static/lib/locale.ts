import store from "store2"

const STORAGE_KEY = "locale"
const COOKIE_NAME = "locale"

export function getLocale(): string {
  const stored = store.get(STORAGE_KEY)
  if (stored) return stored

  const match = document.cookie.match(new RegExp(`${COOKIE_NAME}=([^;]+)`))
  if (match) return match[1]

  return document.documentElement.lang || "en"
}

const VALID_LOCALE = /^[a-zA-Z0-9-]{2,10}$/

export function setLocale(locale: string): void {
  if (!VALID_LOCALE.test(locale)) return

  // Validate against the server-rendered locale options when available, so
  // programmatic callers cannot pick an unsupported value that would fall
  // back to English after reload.
  const select = document.querySelector<HTMLSelectElement>('[data-action="change-locale"]')
  if (select) {
    const allowed = Array.from(select.options).map((o) => o.value)
    if (!allowed.includes(locale)) return
  }

  store.set(STORAGE_KEY, locale)
  document.cookie = `${COOKIE_NAME}=${locale};path=/;max-age=31536000;samesite=lax`
  window.location.reload()
}
