import { cn } from '@/lib/utils'
import type { OutboundResult } from '@/App'

interface Props {
  ruServices: string[]
  blockedServices: string[]
  ruStatus: number[]
  blockedStatus: number[]
  ruOutbounds: OutboundResult[]
  blockedOutbounds: OutboundResult[]
}

function StatusDot({ status }: { status: number }) {
  return (
    <div
      className={cn(
        'h-2 w-2 shrink-0 rounded-full transition-all duration-300',
        status === 0 && 'bg-muted-foreground/40',
        status === 1 && 'bg-success shadow-[0_0_6px_hsl(var(--success))]',
        status === 2 && 'bg-destructive shadow-[0_0_6px_hsl(var(--destructive))]',
        status === 3 && 'animate-pulse bg-yellow-500',
      )}
    />
  )
}

function outboundClass(tag: string) {
  if (tag === 'block' || tag === 'blocked') {
    return 'bg-destructive/15 text-destructive border-destructive/30'
  }
  if (tag === 'direct') {
    return 'bg-muted/60 text-muted-foreground border-border/40'
  }
  if (!tag) {
    return 'bg-muted/30 text-muted-foreground/60 border-border/30'
  }
  // proxy / vless / etc.
  return 'bg-primary/15 text-primary border-primary/30'
}

function shortTag(tag: string) {
  if (!tag) return '?'
  if (tag === 'direct') return 'direct'
  if (tag === 'block' || tag === 'blocked') return 'block'
  if (tag.length > 10) return tag.slice(0, 9) + '…'
  return tag
}

function OutboundBadge({ result }: { result?: OutboundResult }) {
  if (!result) return null
  return (
    <span
      className={cn(
        'rounded border px-1 py-px text-[8px] font-semibold uppercase leading-none tracking-wider',
        outboundClass(result.outboundTag),
        !result.confident && 'opacity-70',
      )}
      title={`${result.outboundTag || '(default)'} via ${result.matchedBy}${
        result.confident ? '' : ' (geosite hint)'
      }`}
    >
      {shortTag(result.outboundTag)}
    </span>
  )
}

function ServiceRow({
  name,
  status,
  outbound,
}: {
  name: string
  status: number
  outbound?: OutboundResult
}) {
  return (
    <div className="flex h-[22px] items-center gap-1.5">
      <StatusDot status={status} />
      <span className="truncate text-[11px] leading-none text-foreground/85">{name}</span>
      <span className="ml-auto">
        <OutboundBadge result={outbound} />
      </span>
    </div>
  )
}

export function ConnectivityGrid({
  ruServices,
  blockedServices,
  ruStatus,
  blockedStatus,
  ruOutbounds,
  blockedOutbounds,
}: Props) {
  if (ruServices.length === 0 && blockedServices.length === 0) {
    return (
      <p className="py-4 text-center text-xs text-muted-foreground">
        No connectivity data yet
      </p>
    )
  }
  return (
    <div className="grid grid-cols-2 gap-x-3">
      <div>
        <div className="mb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">
          RU
        </div>
        {ruServices.map((s, i) => (
          <ServiceRow
            key={s}
            name={s}
            status={ruStatus[i] ?? 0}
            outbound={ruOutbounds[i]}
          />
        ))}
      </div>
      <div>
        <div className="mb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">
          Blocked
        </div>
        {blockedServices.map((s, i) => (
          <ServiceRow
            key={s}
            name={s}
            status={blockedStatus[i] ?? 0}
            outbound={blockedOutbounds[i]}
          />
        ))}
      </div>
    </div>
  )
}
