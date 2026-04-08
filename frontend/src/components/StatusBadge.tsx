import { cn } from '@/lib/utils'
import type { AppStatus } from '@/App'

const labels: Record<AppStatus, string> = {
  idle: 'Idle',
  starting: 'Starting',
  running: 'Running',
  stopping: 'Stopping',
  restarting: 'Restarting',
  error: 'Error',
}

const dotColor: Record<AppStatus, string> = {
  idle: 'bg-muted-foreground',
  starting: 'bg-yellow-500 animate-pulse',
  running: 'bg-success shadow-[0_0_10px_hsl(var(--success))]',
  stopping: 'bg-orange-500 animate-pulse',
  restarting: 'bg-blue-500 animate-pulse',
  error: 'bg-destructive shadow-[0_0_10px_hsl(var(--destructive))]',
}

const textColor: Record<AppStatus, string> = {
  idle: 'text-muted-foreground',
  starting: 'text-yellow-500',
  running: 'text-success',
  stopping: 'text-orange-500',
  restarting: 'text-blue-500',
  error: 'text-destructive',
}

export function StatusBadge({ status }: { status: AppStatus }) {
  return (
    <div className="flex items-center gap-2 rounded-full border border-border/40 bg-background/40 px-3 py-1">
      <div className={cn('h-2 w-2 rounded-full', dotColor[status])} />
      <span className={cn('text-xs font-medium uppercase tracking-wider', textColor[status])}>
        {labels[status]}
      </span>
    </div>
  )
}
