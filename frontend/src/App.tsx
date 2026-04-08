import { useEffect, useState, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { TitleBar } from '@/components/TitleBar'
import { StatusBadge } from '@/components/StatusBadge'
import { ConnectivityGrid } from '@/components/ConnectivityGrid'
import { ConfigSelect } from '@/components/ConfigSelect'
import { ConfigInfoView } from '@/components/ConfigInfoView'
import { LogsView } from '@/components/LogsView'
import { ProgressOverlay, type Progress } from '@/components/ProgressOverlay'
import { Play, Square, RotateCcw, Download, RefreshCw, Globe, Route, Terminal } from 'lucide-react'
import {
  GetState,
  Start,
  Stop,
  Restart,
  CheckConnectivity,
  UpdateGeo,
  DownloadXray,
  SelectConfig,
} from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'

export type AppStatus = 'idle' | 'starting' | 'running' | 'stopping' | 'restarting' | 'error'

export interface ConfigInfo {
  outbounds: Array<{
    tag: string
    protocol: string
    address?: string
    port?: number
    network?: string
    security?: string
  }>
  rules: Array<{
    outboundTag: string
    type?: string
    domain?: string[]
    ip?: string[]
    port?: string
    network?: string
    source?: string[]
    protocol?: string[]
  }>
  default: string
  primary: string
}

export interface OutboundResult {
  outboundTag: string
  ruleIndex: number
  matchedBy: string
  confident: boolean
}

export interface State {
  status: AppStatus
  message: string
  configs: string[]
  selectedConfig: string
  xrayVersion: string
  configInfo?: ConfigInfo
  ruStatus: number[]
  blockedStatus: number[]
  ruServices: string[]
  blockedServices: string[]
  ruOutbounds: OutboundResult[]
  blockedOutbounds: OutboundResult[]
  checking: boolean
  progress: Progress
}

const emptyState: State = {
  status: 'idle',
  message: '',
  configs: [],
  selectedConfig: '',
  xrayVersion: '',
  ruStatus: [],
  blockedStatus: [],
  ruServices: [],
  blockedServices: [],
  ruOutbounds: [],
  blockedOutbounds: [],
  checking: false,
  progress: { active: false, stage: '', downloaded: 0, total: 0 },
}

function App() {
  const [state, setState] = useState<State>(emptyState)
  const [tab, setTab] = useState<string>('connectivity')

  const refresh = useCallback(async () => {
    try {
      const s = await GetState()
      setState(s as State)
    } catch (e) {
      console.error('GetState failed:', e)
    }
  }, [])

  useEffect(() => {
    refresh()
    const unsub = EventsOn('state', (s: State) => {
      setState(s)
    })
    return () => {
      if (typeof unsub === 'function') unsub()
    }
  }, [refresh])

  const busy =
    state.status === 'starting' ||
    state.status === 'stopping' ||
    state.status === 'restarting'

  return (
    <div className="relative flex h-full flex-col overflow-hidden bg-gradient-to-br from-background via-background to-secondary/20">
      <TitleBar />
      <ProgressOverlay progress={state.progress} />
      <main className="flex flex-1 flex-col gap-3 overflow-hidden px-4 pb-4 pt-3">
        {/* Status Card */}
        <Card className="shrink-0">
          <CardHeader className="space-y-0 p-3 pb-2">
            <div className="flex items-center justify-between">
              <CardTitle>Connection</CardTitle>
              <StatusBadge status={state.status} />
            </div>
          </CardHeader>
          <CardContent className="space-y-2 p-3 pt-0">
            <ConfigSelect
              configs={state.configs}
              selected={state.selectedConfig}
              onSelect={async (c) => {
                await SelectConfig(c)
                refresh()
              }}
              disabled={busy}
            />
            <div className="flex h-4 items-center">
              {state.message ? (
                <p className="truncate text-[11px] text-muted-foreground">{state.message}</p>
              ) : state.xrayVersion ? (
                <p className="truncate text-[10px] text-muted-foreground/70">
                  {state.xrayVersion}
                </p>
              ) : null}
            </div>

            {state.configInfo && state.configInfo.outbounds.length > 0 && (
              <div className="flex flex-wrap items-center gap-1">
                <span className="text-[9px] uppercase tracking-wider text-muted-foreground/70">
                  Outbounds:
                </span>
                {state.configInfo.outbounds.map((o) => {
                  const isPrimary = o.tag === state.configInfo!.primary
                  const isDefault = o.tag === state.configInfo!.default
                  return (
                    <span
                      key={o.tag}
                      className={
                        'rounded border px-1.5 py-px text-[9px] font-semibold uppercase tracking-wider ' +
                        (isPrimary
                          ? 'border-primary/60 bg-primary/15 text-primary'
                          : isDefault
                            ? 'border-border bg-muted/60 text-muted-foreground'
                            : o.tag === 'block'
                              ? 'border-destructive/30 bg-destructive/10 text-destructive'
                              : 'border-border/40 bg-background/40 text-muted-foreground')
                      }
                      title={`${o.protocol}${o.address ? ' → ' + o.address : ''}`}
                    >
                      {o.tag}
                    </span>
                  )
                })}
              </div>
            )}

            <div className="grid grid-cols-3 gap-2">
              <Button
                variant="success"
                size="sm"
                onClick={() => Start()}
                disabled={busy || state.status === 'running'}
              >
                <Play className="h-3.5 w-3.5" />
                Start
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={() => Stop()}
                disabled={busy || state.status !== 'running'}
              >
                <Square className="h-3.5 w-3.5" />
                Stop
              </Button>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => Restart()}
                disabled={busy}
              >
                <RotateCcw className="h-3.5 w-3.5" />
                Restart
              </Button>
            </div>
          </CardContent>
        </Card>

        {/* Connectivity / Routing tabs */}
        <Card className="flex min-h-0 flex-1 flex-col">
          <Tabs
            value={tab}
            onValueChange={setTab}
            className="flex min-h-0 flex-1 flex-col"
          >
            <CardHeader className="space-y-0 p-3 pb-2">
              <div className="flex items-center justify-between gap-2">
                <TabsList>
                  <TabsTrigger value="connectivity">
                    <Globe className="h-3.5 w-3.5" />
                    Net
                  </TabsTrigger>
                  <TabsTrigger value="routing">
                    <Route className="h-3.5 w-3.5" />
                    Routing
                  </TabsTrigger>
                  <TabsTrigger value="logs">
                    <Terminal className="h-3.5 w-3.5" />
                    Logs
                  </TabsTrigger>
                </TabsList>
                {tab === 'connectivity' && (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7"
                    onClick={() => CheckConnectivity()}
                    disabled={state.checking}
                  >
                    <RefreshCw
                      className={'h-3.5 w-3.5 ' + (state.checking ? 'animate-spin' : '')}
                    />
                  </Button>
                )}
              </div>
            </CardHeader>
            <CardContent className="flex min-h-0 flex-1 flex-col p-3 pt-0">
              <TabsContent value="connectivity">
                <ConnectivityGrid
                  ruServices={state.ruServices}
                  blockedServices={state.blockedServices}
                  ruStatus={state.ruStatus}
                  blockedStatus={state.blockedStatus}
                  ruOutbounds={state.ruOutbounds}
                  blockedOutbounds={state.blockedOutbounds}
                />
              </TabsContent>
              <TabsContent value="routing">
                <ConfigInfoView info={state.configInfo} />
              </TabsContent>
              <TabsContent value="logs">
                <LogsView />
              </TabsContent>
            </CardContent>
          </Tabs>
        </Card>

        {/* Footer actions */}
        <div className="grid shrink-0 grid-cols-2 gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => UpdateGeo()}
            disabled={busy}
          >
            <Download className="h-3.5 w-3.5" />
            Update geo
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => DownloadXray()}
            disabled={busy}
          >
            <Download className="h-3.5 w-3.5" />
            Update xray
          </Button>
        </div>
      </main>
    </div>
  )
}

export default App
