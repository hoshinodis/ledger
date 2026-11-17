/** nav navigates the SPA without a router library. */
export function nav(to: string) {
  window.history.pushState({}, '', to)
  window.dispatchEvent(new PopStateEvent('popstate'))
}
