import { useEffect, useRef, useState } from 'react'
import { Copy, Trash2, ChevronDown, Pause, Play } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { GetLogs, ClearLogs } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { cn } from '@/lib/utils'

function lineColor(line: string) {
  if (line.includes('[ERROR]') || line.includes('[PANIC]')) return 'text-destructive'
  if (line.includes('[WARN]')) return 'text-yellow-500'
  if (line.includes('[INFO]')) return 'text-foreground/85'
  if (line.includes('[CMD]') || line.includes('[PS]')) return 'text-muted-foreground/70'
  if (line.includes('[OK]')) return 'text-success'
  if (line.includes('[TRAY]') || line.includes('[SYS]')) return 'text-primary/80'
  return 'text-foreground/60'
}

export function LogsView() {
  const [lines, setLines] = useState<string[]>([])
  const [autoScroll, setAutoScroll] = useState(true)
  const [copied, setCopied] = useState(false)
  const scrollRef = useRef<HTMLDivElement>(null)

  // initial load + subscribe
  useEffect(() => {
    GetLogs().then(setLines).catch(() => {})
    const offLogs = EventsOn('logs', (newLines: string[]) => {
      setLines(newLines)
    })
    const offCleared = EventsOn('logs-cleared', () => setLines([]))
    return () => {
      if (typeof offLogs === 'function') offLogs()
      if (typeof offCleared === 'function') offCleared()
    }
  }, [])

  // auto scroll to bottom on new lines
  useEffect(() => {
    if (!autoScroll || !scrollRef.current) return
    scrollRef.current.scrollTop = scrollRef.current.scrollHeight
  }, [lines, autoScroll])

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(lines.join('\n'))
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch (e) {
      console.error('clipboard write failed:', e)
    }
  }

  const clear = () => {
    ClearLogs()
    setLines([])
  }

  const scrollToBottom = () => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
    setAutoScroll(true)
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2">
      <div className="flex shrink-0 items-center justify-between gap-1">
        <span className="text-[10px] uppercase tracking-wider text-muted-foreground">
          {lines.length} / 5000 lines
        </span>
        <div className="flex gap-1">
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={() => setAutoScroll((v) => !v)}
            title={autoScroll ? 'Pause autoscroll' : 'Resume autoscroll'}
          >
            {autoScroll ? <Pause className="h-3 w-3" /> : <Play className="h-3 w-3" />}
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={scrollToBottom}
            title="Jump to bottom"
          >
            <ChevronDown className="h-3 w-3" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={copy}
            title="Copy all"
          >
            <Copy className={cn('h-3 w-3', copied && 'text-success')} />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={clear}
            title="Clear logs"
          >
            <Trash2 className="h-3 w-3" />
          </Button>
        </div>
      </div>
      <div
        ref={scrollRef}
        onWheel={() => {
          if (!scrollRef.current) return
          const el = scrollRef.current
          const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 4
          setAutoScroll(atBottom)
        }}
        className="min-h-0 flex-1 overflow-y-auto rounded-md border border-border/40 bg-background/40 p-2 font-mono text-[10px] leading-snug"
      >
        {lines.length === 0 ? (
          <p className="py-4 text-center text-muted-foreground">No log entries</p>
        ) : (
          lines.map((line, i) => (
            <div key={i} className={cn('whitespace-pre-wrap break-all', lineColor(line))}>
              {line}
            </div>
          ))
        )}
      </div>
    </div>
  )
}
