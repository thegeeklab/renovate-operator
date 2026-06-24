import { getPersisted, setPersisted } from "../lib/storage"
import { getData } from "../lib/dom"

export class RepoSortComponent {
  private el: HTMLElement
  private sortKey: string
  private orderKey: string
  private sort: string
  private order: string

  constructor(el: HTMLElement) {
    this.el = el
    this.sortKey = getData(el, "sort-key")
    this.orderKey = getData(el, "order-key")
    this.sort = getPersisted<string>(this.sortKey, "name")
    this.order = getPersisted<string>(this.orderKey, "asc")

    this.bindEvents()
    this.updateUI()
  }

  private bindEvents(): void {
    const select = this.el.querySelector<HTMLSelectElement>("select[name='sort']")
    if (select) {
      select.value = this.sort
      select.addEventListener("change", () => {
        this.sort = select.value
        setPersisted(this.sortKey, this.sort)
        this.dispatchSortChanged()
      })
    }

    const orderBtn = this.el.querySelector<HTMLElement>('[data-action="toggle-order"]')
    if (orderBtn) {
      orderBtn.addEventListener("click", () => {
        this.order = this.order === "asc" ? "desc" : "asc"
        setPersisted(this.orderKey, this.order)
        this.updateUI()
        this.dispatchSortChanged()
      })
    }
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
    const iconAsc = this.el.querySelector<HTMLElement>('[data-role="sort-asc"]')
    const iconDesc = this.el.querySelector<HTMLElement>('[data-role="sort-desc"]')
    const orderBtn = this.el.querySelector<HTMLElement>('[data-action="toggle-order"]')

    if (iconAsc) iconAsc.style.display = this.order === "asc" ? "" : "none"
    if (iconDesc) iconDesc.style.display = this.order === "desc" ? "" : "none"
    if (orderBtn) {
      orderBtn.setAttribute(
        "aria-label",
        this.order === "asc" ? "Switch to descending order" : "Switch to ascending order"
      )
    }

    const repoList = this.el.querySelector<HTMLElement>('[data-ref="repoList"]')
    if (repoList) {
      this.updateHxVals(repoList)
    }
  }
}

export function initRepoSorts(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>('[data-component="repo-sort"]').forEach((el) => {
    new RepoSortComponent(el)
  })
}
