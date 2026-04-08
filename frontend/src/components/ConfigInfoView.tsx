import { ArrowRight, Server, Shield, Globe2, Ban } from 'lucide-react'
import { cn } from '@/lib/utils'

interface OutboundInfo {
  tag: string
  protocol: string
  address?: string
  port?: number
  network?: string
  security?: string
}

interface RoutingRule {
  outboundTag: string
  type?: string
  domain?: string[]
  ip?: string[]
  port?: string
  network?: string
  source?: string[]
  protocol?: string[]
}

interface ConfigInfo {
  outbounds: OutboundInfo[]
  rules: RoutingRule[]
  default: string
  primary: string
}

function tagBadgeColor(tag: string) {
  if (tag === 'block' || tag === 'blocked') return 'bg-destructive/15 text-destructive'
  if (tag === 'direct') return 'bg-muted text-muted-foreground'
  return 'bg-primary/15 text-primary'
}

function tagIcon(tag: string, protocol: string) {
  if (tag === 'block' || protocol === 'blackhole') return Ban
  if (tag === 'direct' || protocol === 'freedom') return Globe2
  return Shield
}

function summarizeRule(r: RoutingRule): string {
  const parts: string[] = []
  if (r.domain && r.domain.length) {
    parts.push(`${r.domain.length} domain${r.domain.length > 1 ? 's' : ''}`)
  }
  if (r.ip && r.ip.length) {
    parts.push(`${r.ip.length} ip${r.ip.length > 1 ? 's' : ''}`)
  }
  if (r.port) parts.push(`port ${r.port}`)
  if (r.network) parts.push(r.network)
  if (r.protocol && r.protocol.length) parts.push(r.protocol.join(','))
  if (r.source && r.source.length) parts.push(`src:${r.source.length}`)
  return parts.join(' • ') || 'any'
}

function ruleSamples(r: RoutingRule): string[] {
  const out: string[] = []
  if (r.domain) out.push(...r.domain)
  if (r.ip) out.push(...r.ip)
  return out
}

export function ConfigInfoView({ info }: { info: ConfigInfo | null | undefined }) {
  if (!info) {
    return (
      <p className="py-4 text-center text-xs text-muted-foreground">No config loaded</p>
    )
  }
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto pr-1">
      {/* Outbounds */}
      <section>
        <h3 className="mb-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/80">
          Outbounds
        </h3>
        <div className="space-y-1.5">
          {info.outbounds.length === 0 && (
            <p className="text-[11px] text-muted-foreground">none</p>
          )}
          {info.outbounds.map((o) => {
            const Icon = tagIcon(o.tag, o.protocol)
            const isPrimary = o.tag === info.primary
            const isDefault = o.tag === info.default
            return (
              <div
                key={o.tag + o.protocol}
                className={cn(
                  'flex items-center gap-2 rounded-md border border-border/40 bg-background/40 px-2 py-1.5',
                  isPrimary && 'border-primary/50 ring-1 ring-primary/30',
                )}
              >
                <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span
                  className={cn(
                    'rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider',
                    tagBadgeColor(o.tag),
                  )}
                >
                  {o.tag || '—'}
                </span>
                <span className="text-[11px] text-muted-foreground">{o.protocol}</span>
                {isDefault && (
                  <span className="rounded bg-muted px-1 py-px text-[8px] font-semibold uppercase tracking-wider text-muted-foreground">
                    default
                  </span>
                )}
                {o.address && (
                  <span className="ml-auto truncate font-mono text-[10px] text-foreground/80">
                    {o.address}
                    {o.port ? `:${o.port}` : ''}
                  </span>
                )}
              </div>
            )
          })}
        </div>
      </section>

      {/* Routing Rules */}
      <section>
        <h3 className="mb-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/80">
          Routing rules
          <span className="ml-1 text-muted-foreground/60">({info.rules.length})</span>
        </h3>
        <div className="space-y-1">
          {info.rules.length === 0 && (
            <p className="text-[11px] text-muted-foreground">no routing rules</p>
          )}
          {info.rules.map((r, i) => {
            const samples = ruleSamples(r)
            return (
              <div
                key={i}
                className="flex flex-col gap-0.5 rounded-md border border-border/30 bg-background/30 px-2 py-1.5"
              >
                <div className="flex items-center gap-1.5">
                  <Server className="h-3 w-3 shrink-0 text-muted-foreground/70" />
                  <span className="text-[10px] text-muted-foreground">
                    {summarizeRule(r)}
                  </span>
                  <ArrowRight className="ml-auto h-3 w-3 text-muted-foreground/50" />
                  <span
                    className={cn(
                      'rounded px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wider',
                      tagBadgeColor(r.outboundTag),
                    )}
                  >
                    {r.outboundTag}
                  </span>
                </div>
                {samples.length > 0 && (
                  <ul className="ml-[18px] mt-0.5 space-y-px font-mono text-[10px] text-foreground/60">
                    {samples.map((s) => (
                      <li key={s} className="break-all">
                        {s}
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )
          })}
        </div>
      </section>
    </div>
  )
}
