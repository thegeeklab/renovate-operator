import { getPersisted, setPersisted } from "../lib/storage"
import { getData } from "../lib/dom"
import { Dropdown } from "../lib/dropdown"
import { registerComponent } from "../lib/component.registry"
import { t } from "../lib/i18n"

const ALL_FILTERS = ["filterOpenPRs", "filterWarnings", "filterErrors"]

function getPersistedFilters(key: string): Set<string> {
  return new Set(getPersisted<string[]>(key, []))
}

function setListHxVals(
  repoList: HTMLElement,
  sort: string,
  order: string,
  filters: Set<string>
): void {
  const vals: Record<string, string | boolean> = { sort, order }

  for (const filter of ALL_FILTERS) {
    vals[filter] = filters.has(filter)
  }

  repoList.setAttribute("hx-vals", JSON.stringify(vals))
}

function dispatchListChanged(repoList: HTMLElement): void {
  repoList.dispatchEvent(new Event("list-changed"))
}

export class RepoSortComponent extends Dropdown {
  private sortKey: string
  private orderKey: string
  private filterKey: string
  private sort: string
  private order: string
  private boundOptionClicks: Map<HTMLButtonElement, () => void> = new Map()

  constructor(el: HTMLElement) {
    super(el, {
      buttonSelector: '[data-action="toggle-sort"]',
      menuSelector: '[data-role="sort-menu"]',
      placement: "bottom-start",
      strategy: "fixed",
      offset: 4,
      focusOnClose: true
    })

    this.sortKey = getData(el, "sort-key")
    this.orderKey = getData(el, "order-key")
    this.filterKey = getData(el, "filter-key")
    this.sort = getPersisted<string>(this.sortKey, "name")
    this.order = getPersisted<string>(this.orderKey, "asc")

    this.bindOrderToggle()
    this.bindOptions()
    this.updateUI()
  }

  private bindOrderToggle(): void {
    const orderBtn = this.el.querySelector<HTMLButtonElement>('[data-action="toggle-order"]')
    if (orderBtn) {
      orderBtn.addEventListener("click", () => {
        this.order = this.order === "asc" ? "desc" : "asc"
        setPersisted(this.orderKey, this.order)
        this.updateUI()
        this.dispatchSortChanged()
      })
    }
  }

  private bindOptions(): void {
    this.menu.querySelectorAll<HTMLButtonElement>('[data-role="sort-option"]').forEach((btn) => {
      const handler = () => this.handleOptionSelect(btn)
      this.boundOptionClicks.set(btn, handler)
      btn.addEventListener("click", handler)
    })
  }

  private handleOptionSelect(btn: HTMLButtonElement): void {
    const { value } = btn.dataset
    if (!value) return
    if (value === this.sort) {
      this.close()
      return
    }
    this.sort = value
    setPersisted(this.sortKey, this.sort)
    this.updateUI()
    this.close()
    this.dispatchSortChanged()
  }

  private dispatchSortChanged(): void {
    const repoList = this.el.querySelector<HTMLElement>('[data-ref="repoList"]')
    if (repoList) {
      this.updateHxVals(repoList)
      dispatchListChanged(repoList)
    }
  }

  private updateHxVals(repoList: HTMLElement): void {
    setListHxVals(repoList, this.sort, this.order, getPersistedFilters(this.filterKey))
  }

  private updateUI(): void {
    const options = Array.from(
      this.menu.querySelectorAll<HTMLButtonElement>('[data-role="sort-option"]')
    )
    const activeOption = options.find((btn) => btn.dataset.value === this.sort)
    if (activeOption) {
      const labelEl = this.el.querySelector<HTMLElement>('[data-role="sort-label"]')
      if (labelEl) {
        labelEl.textContent = activeOption.textContent?.trim() || ""
      }
    }

    options.forEach((btn) => {
      const isActive = btn.dataset.value === this.sort
      btn.classList.toggle("font-semibold", isActive)
      btn.setAttribute("aria-selected", isActive ? "true" : "false")
    })

    const iconAsc = this.el.querySelector<HTMLElement>('[data-role="sort-asc"]')
    const iconDesc = this.el.querySelector<HTMLElement>('[data-role="sort-desc"]')
    const orderBtn = this.el.querySelector<HTMLElement>('[data-action="toggle-order"]')
    const tooltipText = this.el.querySelector<HTMLElement>(
      '[data-role="sort-order"] .tooltip-text span:first-child'
    )

    if (iconAsc) iconAsc.classList.toggle("hidden", this.order !== "asc")
    if (iconDesc) iconDesc.classList.toggle("hidden", this.order !== "desc")
    if (orderBtn) {
      orderBtn.setAttribute(
        "aria-label",
        this.order === "asc" ? t("sort.switch_descending") : t("sort.switch_ascending")
      )
      orderBtn.setAttribute("aria-pressed", this.order === "asc" ? "true" : "false")
    }
    if (tooltipText) {
      tooltipText.textContent =
        this.order === "asc" ? t("sort.sort_ascending") : t("sort.sort_descending")
    }

    const repoList = this.el.querySelector<HTMLElement>('[data-ref="repoList"]')
    if (repoList) {
      this.updateHxVals(repoList)
    }
  }

  destroy(): void {
    super.destroy()
    this.boundOptionClicks.forEach((handler, btn) => {
      btn.removeEventListener("click", handler)
    })
    this.boundOptionClicks.clear()
  }
}

export class RepoFilterComponent extends Dropdown {
  private filterKey: string
  private filters: Set<string>
  private boundCheckboxChanges: Map<HTMLInputElement, () => void> = new Map()

  constructor(el: HTMLElement) {
    super(el, {
      buttonSelector: '[data-action="toggle-filter"]',
      menuSelector: '[data-role="filter-menu"]',
      placement: "bottom-start",
      strategy: "fixed",
      offset: 4,
      focusOnClose: true
    })

    const sortEl = el.closest<HTMLElement>('[data-component="repo-sort"]')

    this.filterKey = sortEl ? getData(sortEl, "filter-key") : ""
    this.filters = getPersistedFilters(this.filterKey)

    this.syncCheckboxes()
    this.bindCheckboxes()
  }

  private bindCheckboxes(): void {
    this.menu.querySelectorAll<HTMLInputElement>("input[data-filter]").forEach((cb) => {
      const handler = () => this.handleCheckboxChange(cb)
      this.boundCheckboxChanges.set(cb, handler)
      cb.addEventListener("change", handler)
    })
  }

  private handleCheckboxChange(cb: HTMLInputElement): void {
    const { filter } = cb.dataset
    if (!filter) return

    if (cb.checked) {
      this.filters.add(filter)
    } else {
      this.filters.delete(filter)
    }

    setPersisted(this.filterKey, Array.from(this.filters))
    this.dispatchFilterChanged()
  }

  private syncCheckboxes(): void {
    this.menu.querySelectorAll<HTMLInputElement>("input[data-filter]").forEach((cb) => {
      const { filter } = cb.dataset
      if (!filter) return

      cb.checked = this.filters.has(filter)
    })
  }

  private dispatchFilterChanged(): void {
    const sortEl = this.el.closest<HTMLElement>('[data-component="repo-sort"]')
    if (!sortEl) return

    const repoList = sortEl.querySelector<HTMLElement>('[data-ref="repoList"]')
    if (!repoList) return

    this.updateHxVals(repoList, sortEl)
    dispatchListChanged(repoList)
  }

  private updateHxVals(repoList: HTMLElement, sortEl: HTMLElement): void {
    const sortKey = getData(sortEl, "sort-key")
    const orderKey = getData(sortEl, "order-key")
    const sort = getPersisted<string>(sortKey, "name")
    const order = getPersisted<string>(orderKey, "asc")

    setListHxVals(repoList, sort, order, this.filters)
  }

  destroy(): void {
    super.destroy()

    this.boundCheckboxChanges.forEach((handler, cb) => {
      cb.removeEventListener("change", handler)
    })
    this.boundCheckboxChanges.clear()
  }
}

export function initRepoSorts(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="repo-sort"]').forEach((el) => {
    const component = new RepoSortComponent(el)
    registerComponent(el, component)
  })
}

export function initRepoFilters(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="repo-filter"]').forEach((el) => {
    const component = new RepoFilterComponent(el)
    registerComponent(el, component)
  })
}
