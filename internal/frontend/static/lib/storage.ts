import store from "store2"

export function getPersisted<T>(key: string, defaultValue: T): T {
  const value = store.get(key)
  if (value === null || value === undefined) {
    return defaultValue
  }
  return value as T
}

export function setPersisted(key: string, value: unknown): void {
  store.set(key, value)
}
