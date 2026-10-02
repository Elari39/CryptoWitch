import { createApp, h, nextTick, type Component } from 'vue'

export function deferred<T = any>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

export async function flush() {
  for (let i = 0; i < 12; i++) await Promise.resolve()
  await nextTick()
}

export function mount(component: Component, props: Record<string, unknown> = {}) {
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp({ render: () => h(component, props) })
  app.mount(host)
  return { host, unmount() { app.unmount(); host.remove() } }
}
