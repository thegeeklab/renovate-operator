const el = document.getElementById("i18n-data")
const data = JSON.parse(el?.getAttribute("data-json") || "{}")

export function t(key: string, args?: Record<string, string>): string {
  const keys = key.split(".")
  let value: unknown = data
  for (const k of keys) {
    if (value === null || value === undefined) return key
    value = (value as Record<string, unknown>)[k]
  }
  if (!value || typeof value !== "string") return key
  if (args) {
    return Object.entries(args).reduce(
      (s, [k, v]) =>
        s.replace(
          new RegExp(`\\{\\{\\.?\\s*${k.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}\\s*\\}\\}`, "g"),
          v
        ),
      value
    )
  }
  return value
}
