declare module 'katex/contrib/auto-render' {
  import type { KatexOptions } from 'katex'

  interface AutoRenderDelimiter {
    left: string
    right: string
    display: boolean
  }

  interface AutoRenderOptions extends KatexOptions {
    delimiters?: AutoRenderDelimiter[]
    ignoredTags?: string[]
    ignoredClasses?: string[]
    preProcess?: (math: string) => string
    errorCallback?: (message: string, error: unknown) => void
  }

  export default function renderMathInElement(element: HTMLElement, options?: AutoRenderOptions): void
}
