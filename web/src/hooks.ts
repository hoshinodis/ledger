import { useCallback, useEffect, useState } from 'react'

/** useApi runs an async loader and returns [data, error, reload].
 * Extra deps re-run the load when they change. */
export function useApi<T>(load: () => Promise<T>, deps: unknown[] = []): [T | null, string | null, () => void] {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [tick, setTick] = useState(0)

  useEffect(() => {
    let alive = true
    load()
      .then((d) => {
        if (alive) {
          setData(d)
          setError(null)
        }
      })
      .catch((e: unknown) => {
        if (alive) setError(e instanceof Error ? e.message : String(e))
      })
    return () => {
      alive = false
    }
    // deps and load are captured from the calling render
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tick, ...deps])

  const reload = useCallback(() => setTick((t) => t + 1), [])
  return [data, error, reload]
}

/** useField is a small controlled input helper: [value, onChange, setValue]. */
export function useField(initial: string) {
  const [value, setValue] = useState(initial)
  const onChange = (e: { target: { value: string } }) => setValue(e.target.value)
  return [value, onChange, setValue] as const
}
