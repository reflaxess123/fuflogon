import { Loader2 } from 'lucide-react'
import { cn } from '@/lib/utils'

export interface Progress {
  active: boolean
  stage: string
  file?: string
  downloaded: number
  total: number
  step?: number
  stepCount?: number
}

function fmtBytes(n: number): string {
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB'
  return (n / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}

export function ProgressOverlay({ progress }: { progress: Progress }) {
  if (!progress.active) return null

  const pct =
    progress.total > 0 ? Math.min(100, (progress.downloaded / progress.total) * 100) : 0
  const indeterminate = progress.total <= 0

  return (
    <div className="absolute inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm">
      <div className="mx-6 w-full max-w-sm rounded-xl border border-border/50 bg-card p-5 shadow-2xl">
        <div className="mb-3 flex items-center gap-2">
          <Loader2 className="h-4 w-4 animate-spin text-primary" />
          <h3 className="text-sm font-semibold">{progress.stage}</h3>
        </div>

        {progress.file && (
          <p className="mb-3 truncate font-mono text-[11px] text-muted-foreground">
            {progress.file}
          </p>
        )}

        {/* Progress bar */}
        <div className="relative h-2 w-full overflow-hidden rounded-full bg-muted">
          {indeterminate ? (
            <div className="absolute inset-y-0 w-1/3 animate-[shimmer_1.4s_ease-in-out_infinite] rounded-full bg-primary" />
          ) : (
            <div
              className="h-full rounded-full bg-primary transition-[width] duration-150 ease-out"
              style={{ width: pct + '%' }}
            />
          )}
        </div>

        <div className="mt-2 flex items-center justify-between text-[10px] text-muted-foreground">
          <span className={cn(indeterminate && 'opacity-0')}>
            {fmtBytes(progress.downloaded)}
            {progress.total > 0 ? ' / ' + fmtBytes(progress.total) : ''}
          </span>
          <span>
            {progress.stepCount && progress.stepCount > 1
              ? `step ${progress.step} / ${progress.stepCount}`
              : indeterminate
                ? 'working...'
                : pct.toFixed(0) + '%'}
          </span>
        </div>
      </div>
    </div>
  )
}
