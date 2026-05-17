/// <reference types="vite/client" />

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          RunDiagnosis(manualProxy: string): Promise<unknown>
          ScanLocalProxies(): Promise<unknown>
          TestManualProxy(manualProxy: string): Promise<unknown>
        }
      }
    }
  }
}

export {}
