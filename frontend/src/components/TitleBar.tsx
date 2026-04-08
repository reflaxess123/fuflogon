import { X, Sun, Moon, Power } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { WindowHide } from '../../wailsjs/runtime/runtime'
import { Quit } from '../../wailsjs/go/main/App'
import { useTheme } from '@/lib/theme'

export function TitleBar() {
  const { theme, toggle } = useTheme()

  return (
    <div
      className="flex h-10 items-center justify-between border-b border-border/50 bg-background/40 px-3 backdrop-blur-md"
      style={{ '--wails-draggable': 'drag' } as React.CSSProperties}
    >
      <div className="flex items-center gap-2">
        <div className="h-2 w-2 rounded-full bg-primary shadow-[0_0_8px_hsl(var(--primary))]" />
        <span className="text-xs font-semibold tracking-wider text-foreground/90">
          FUFLOGON
        </span>
      </div>
      <div
        className="flex gap-1"
        style={{ '--wails-draggable': 'no-drag' } as React.CSSProperties}
      >
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 hover:bg-muted/50"
          onClick={toggle}
          title={theme === 'dark' ? 'Switch to light' : 'Switch to dark'}
        >
          {theme === 'dark' ? (
            <Sun className="h-3.5 w-3.5" />
          ) : (
            <Moon className="h-3.5 w-3.5" />
          )}
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 hover:bg-muted/50"
          onClick={() => WindowHide()}
          title="Hide to tray"
        >
          <X className="h-3.5 w-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 hover:bg-destructive/80"
          onClick={() => Quit()}
          title="Quit Fuflogon (stops VPN)"
        >
          <Power className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  )
}
