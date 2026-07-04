import { getPersisted, setPersisted } from "../lib/storage"
import { getData } from "../lib/dom"
import { Dropdown } from "../lib/dropdown"
import { registerComponent } from "../lib/component.registry"

export class RepoSortComponent extends Dropdown {
  private sortKey: string
  private orderKey: string
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
      repoList.dispatchEvent(new Event("sort-changed"))
    }
  }

  private updateHxVals(repoList: HTMLElement): void {
    repoList.setAttribute("hx-vals", JSON.stringify({ sort: this.sort, order: this.order }))
  }

  private updateUI(): void {
    const activeOption = this.menu.querySelector<HTMLButtonElement>(`[data-value="${this.sort}"]`)
    if (activeOption) {
      const labelEl = this.el.querySelector<HTMLElement>('[data-role="sort-label"]')
      if (labelEl) {
        labelEl.textContent = activeOption.textContent?.trim() || ""
      }
    }

    this.menu.querySelectorAll<HTMLButtonElement>('[data-role="sort-option"]').forEach((btn) => {
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
        this.order === "asc" ? "Switch to descending order" : "Switch to ascending order"
      )
      orderBtn.setAttribute("aria-pressed", this.order === "asc" ? "true" : "false")
    }
    if (tooltipText) {
      tooltipText.textContent = this.order === "asc" ? "Sort ascending" : "Sort descending"
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

export function initRepoSorts(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="repo-sort"]').forEach((el) => {
    const component = new RepoSortComponent(el)
    registerComponent(el, component)
  })
}
