import { Check, ChevronDown } from 'lucide-react'
import { useState, useRef, useEffect } from 'react'
import { cn } from '@/lib/utils'

interface Props {
  configs: string[]
  selected: string
  onSelect: (cfg: string) => void
  disabled?: boolean
}

function basename(p: string) {
  const i = Math.max(p.lastIndexOf('/'), p.lastIndexOf('\\'))
  return i >= 0 ? p.slice(i + 1) : p
}

export function ConfigSelect({ configs, selected, onSelect, disabled }: Props) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    if (open) document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [open])

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        className={cn(
          'flex w-full items-center justify-between gap-2 rounded-md border border-border/50 bg-background/40 px-3 py-2 text-xs transition-colors hover:bg-background/60',
          'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
          'disabled:cursor-not-allowed disabled:opacity-50',
        )}
      >
        <span className="truncate font-medium">{basename(selected) || 'Select config'}</span>
        <ChevronDown className={cn('h-3.5 w-3.5 transition-transform', open && 'rotate-180')} />
      </button>
      {open && (
        <div className="absolute left-0 right-0 top-full z-50 mt-1 overflow-hidden rounded-md border border-border/50 bg-card/95 shadow-xl backdrop-blur-xl">
          {configs.map((c) => (
            <button
              key={c}
              type="button"
              onClick={() => {
                onSelect(c)
                setOpen(false)
              }}
              className="flex w-full items-center justify-between gap-2 px-3 py-2 text-xs transition-colors hover:bg-accent"
            >
              <span className="truncate">{basename(c)}</span>
              {c === selected && <Check className="h-3.5 w-3.5 text-primary" />}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
