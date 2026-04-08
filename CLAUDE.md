# Fuflogon — VPN launcher для xray-core

Кросс-платформенный (Windows / Linux / macOS) launcher для xray-core с UI на
React + Wails. На Windows работает полностью. Linux / macOS — backend пока
заглушки, но билд проходит и UI запускается.

## Стек

- **Go 1.23+** — backend
- **Wails v2.12** — десктоп фреймворк (Go ↔ WebView2/WKWebView/WebKit2GTK)
- **React 18 + TypeScript 5 + Vite 6** — frontend
- **Tailwind 3 + shadcn-style components** — UI
- **fyne.io/systray** — иконка в трее

## Структура репо

```
core/                 # платформо-независимая логика (constants, log, state, config, geo, updater, routing, progress)
platform/             # OS-специфичные части (windows.go ✅ / linux.go 🚧 / darwin.go 🚧)
  windows.go          # реализован полностью
  linux.go            # стабы
  darwin.go           # стабы
  common.go           # IsProcessRunning / KillProcess / StopExistingXray
frontend/             # Vite + React UI
  src/
    App.tsx
    components/
      TitleBar.tsx
      ConnectivityGrid.tsx
      ConfigInfoView.tsx
      LogsView.tsx
      ProgressOverlay.tsx
      ConfigSelect.tsx
      StatusBadge.tsx
      ui/                  # button.tsx, card.tsx, tabs.tsx — shadcn-style
    lib/
      utils.ts             # cn() helper
      theme.ts             # light/dark theme hook
    styles/globals.css     # Tailwind + theme variables
assets/               # эмбедятся в Go binary через //go:embed
  icon-idle.ico       # tray icon (серый globe)
  icon-running.ico    # tray icon (зелёный globe)
  wintun.dll          # эмбедится только на Windows билдe
build/                # Wails-сгенерированные манифесты, иконки приложения
  windows/
  darwin/
release/              # рантайм-папка для тестового запуска (в gitignore)
app.go                # Wails App struct + методы экспозированные в JS
main.go               # entry point: GUI mode (Wails) или CLI (start/stop/...)
tray_windows.go       # systray (только Windows)
tray_stub.go          # стаб для !windows
bootstrap_windows.go  # extractWintun
bootstrap_stub.go     # стаб для !windows
wails.json            # Wails config
go.mod / go.sum
```

## Архитектура

- **`core/`** — без зависимостей от платформы. Парсер xray-конфига
  (`ParseConfigInfo` → outbounds + routing rules + default/primary), резолвер
  outbound по hostname (`ResolveOutbound` с хардкод-хинтами для популярных
  geosite категорий — мы НЕ парсим бинарный geosite.dat), скачивание xray
  с GitHub releases (`DownloadXray`), скачивание geo баз с jsDelivr CDN
  (`UpdateGeo`), хвостер xray-error.log → in-memory log buffer
  (`TailXrayLog`), in-memory ring buffer на 5000 строк (`Logf` → `GetLogBuffer`),
  тип `Progress` для byte-level прогресса.
- **`platform/`** — OS-специфичные операции: `Start(rootDir, cfgName)` поднимает
  TUN адаптер, добавляет статические маршруты к VPS IP и default route через TUN,
  flushes DNS, запускает xray.exe; `Stop` — обратное; `EnsureAdmin` /
  `RelaunchAsAdmin`; `IsProcessRunning(pid)`. На Windows используется `wintun.dll`
  через xray, маршруты через `route.exe` / `netsh`, проверка адаптера через
  PowerShell `Get-NetAdapter`.
- **`tray_*.go`** — иконка в системном трее. Только иконка + tooltip + клик =
  показать окно. **Никакого контекстного меню** (флакает на Windows + UIPI).
  Quit делается из UI кнопкой Power в TitleBar.
- **`app.go`** — Wails App. `startup()` → `bootstrapAndStart()` → проверяет
  наличие wintun/xray/geo и автоматически их получает (wintun из embed, xray
  через GitHub, geo через jsDelivr) с показом ProgressOverlay, потом стартует
  тоннель. Все методы App автоматически экспозируются в JS как
  `window.go.main.App.Start()`, `Stop()`, `Restart()`, `CheckConnectivity()`,
  `UpdateGeo()`, `DownloadXray()`, `Quit()`, `GetState()`, `GetLogs()`,
  `ClearLogs()`, `SelectConfig()`. State эмитится в фронт через
  `wruntime.EventsEmit(ctx, "state", snapshot)` при каждом изменении.

## Bootstrap (первый запуск)

При старте `bootstrapAndStart()`:
1. **`extractWintun(rootDir)`** — пишет эмбедированный wintun.dll рядом с exe если его нет
2. **xray.exe** — если нет, `core.DownloadXray()` тянет с GitHub releases (~30 MB)
3. **geoip.dat / geosite.dat** — если нет, `core.UpdateGeo()` тянет с jsDelivr (~85 MB параллельно)
4. **Только потом** запускает тоннель — `Start()` или re-attach к уже бегущему

ProgressOverlay показывает реальный байтовый прогресс. Юзер видит `1.4 MB / 31.5 MB` + `45%`.

## Файлы которые таскать рядом с exe

Минимум для работы:
- `fuflogon.exe` — лаунчер (на Windows wintun.dll эмбеднут внутрь)
- `config.json` — твой xray-конфиг (см. ниже формат)

Опционально, чтобы не ждать первого запуска:
- `xray.exe` — иначе скачается на старте
- `geoip.dat` / `geosite.dat` (+ `.sha256sum`) — иначе скачаются на старте
- `wintun.dll` — Windows: эмбеднут, можно положить рядом для надёжности

Папка `release/` в репе используется для разработки. Перед commit она гитом
игнорится целиком (`.gitignore` строка `release/`).

## Билд

### Любая платформа

```bash
# Установить Wails CLI один раз
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor    # проверит окружение
```

### Windows

```bash
cd d:/repos/xray-core
wails build -platform windows/amd64
cp build/bin/fuflogon.exe release/fuflogon.exe   # в release/ для теста
```

Билд использует:
- `frontend:install` → `npm install`
- `frontend:build` → `npm run build` (`tsc -b && vite build`)
- Go компилятор бэкенда с эмбедингом ассетов
- Build tags: `windows` для `bootstrap_windows.go`, `tray_windows.go`,
  `platform/windows.go`

### macOS

```bash
cd /path/to/xray-core
wails build -platform darwin/arm64    # для Apple Silicon
# или
wails build -platform darwin/amd64    # Intel
# или универсальный билд:
wails build -platform darwin/universal
```

Артефакт: `build/bin/fuflogon.app` — открывается двойным кликом из Finder.
Нужно подписать (codesign) если будешь раздавать. Для своего теста можно
запустить через `open build/bin/fuflogon.app` или просто кликнуть.

**Что НЕ работает на macOS прямо сейчас**:
- `platform/darwin.go` — стабы. `Start/Stop/Status` возвращают ошибки.
- Tray-иконка работает (fyne.io/systray на Mac рисует через NSStatusItem).
- UI показывается, но кнопка Start упадёт с ошибкой.

**Что нужно дописать на macOS** (порядок):
1. `platform/darwin.go::EnsureAdmin()` — проверка root через `os.Geteuid() == 0`
2. `platform/darwin.go::RelaunchAsAdmin()` — перезапуск через
   `osascript -e 'do shell script "..." with administrator privileges'`
3. `platform/darwin.go::Start(rootDir, cfgName)`:
   - Создать utun интерфейс. xray-core на Mac использует `/dev/tun*` через
     darwin/utun. Адаптер появится автоматически когда xray запустится с
     `inbound type=tun`.
   - Найти default interface: `route -n get default | grep interface`
   - Сохранить старый default gateway: `route -n get default | grep gateway`
   - Записать runtime config с подменой interface
   - Запустить xray (`StartXrayDetached`)
   - Подождать появления utun*: `ifconfig | grep utun`
   - Назначить IP: `ifconfig utunN 198.18.0.1 198.18.0.2 netmask 255.255.255.252`
   - Добавить статический маршрут к VPS IP через старый gateway:
     `route -n add -host VPS_IP GATEWAY`
   - Заменить default route: `route -n change default -interface utunN`
4. `platform/darwin.go::Stop(rootDir)`:
   - Загрузить state.json
   - Удалить статический маршрут к VPS: `route -n delete -host VPS_IP`
   - Восстановить default: `route -n change default GATEWAY`
   - Убить xray pid
   - `dscacheutil -flushcache; killall -HUP mDNSResponder`
5. `platform/darwin.go::FlushDNS()` — `dscacheutil -flushcache`

### Linux

```bash
cd /path/to/xray-core
wails build -platform linux/amd64
```

Артефакт: `build/bin/fuflogon` (без расширения).

**Что НЕ работает на Linux**:
- `platform/linux.go` — стабы.
- На некоторых дистрах нужно поставить `libwebkit2gtk-4.0-dev` (или `4.1`)
  и `libgtk-3-dev` для билда. На Ubuntu:
  ```bash
  sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev pkg-config
  ```

**Что нужно дописать на Linux** (порядок):
1. `platform/linux.go::EnsureAdmin()` — `os.Geteuid() == 0`
2. `platform/linux.go::RelaunchAsAdmin()` — `pkexec` или `gksudo`, или просто
   просить запускать через `sudo` (вернуть err с инструкцией).
3. `platform/linux.go::Start(rootDir, cfgName)`:
   - Создать TUN интерфейс. xray-core на Linux использует `/dev/net/tun`.
     Нужно убедиться что модуль `tun` загружен: `modprobe tun`.
   - Найти default interface: `ip route show default | awk '{print $5}'`
   - Старый gateway: `ip route show default | awk '{print $3}'`
   - Запустить xray
   - Подождать tun-интерфейс: `ip link show xray0`
   - IP: `ip addr add 198.18.0.1/30 dev xray0`
   - Up: `ip link set xray0 up`
   - Маршрут к VPS: `ip route add VPS_IP/32 via GATEWAY dev REAL_IFACE`
   - Default через TUN: `ip route replace default dev xray0`
4. `platform/linux.go::Stop(rootDir)`:
   - `ip route del VPS_IP/32`
   - `ip route replace default via GATEWAY dev REAL_IFACE`
   - kill xray
   - `systemd-resolve --flush-caches` или `resolvectl flush-caches`
5. `platform/linux.go::FlushDNS()` — `resolvectl flush-caches`

**Tray на Linux** — `fyne.io/systray` использует libayatana-appindicator или
StatusNotifierItem. Работает в GNOME через расширение AppIndicator, KDE искаропки.

## Запуск в dev mode (горячая перезагрузка)

```bash
wails dev -platform windows/amd64    # или darwin/arm64, linux/amd64
```

Поднимает Vite dev server, открывает окно и hot-reload-ит React при сохранении.
Go код требует ребилда (Ctrl+R в окне Wails dev).

## Конфиг

Формат — стандартный xray-core JSON. Должен содержать:
- `inbounds[0].protocol == "tun"` с `tag: "tun-in"`
- `outbounds[0]` — `direct` (freedom) — это **default** для трафика без правил
- `outbounds[1+]` — твой proxy (vless/vmess/trojan) с `tag: "proxy"` или другим
- `routing.rules` — какие домены/IP куда

См. `release/config.json` (в gitignore — пример у тебя локально).

## Pack для друзей

```bash
cd release/
pack.bat                  # использует config.json
pack.bat config-ded.json  # или конкретный конфиг
```

Создаёт `fuflogon-bundle-YYYYMMDD.zip` (~120 MB) со всем что нужно для
оффлайн-старта. На Linux/macOS аналогичный скрипт ещё не написан — TODO.

## Где смотреть логи в рантайме

- `release/Logs/xray-launcher.log` — лог лаунчера (file-backed)
- `release/Logs/xray-error.log` — лог самого xray
- В UI: вкладка **Logs** показывает in-memory буфер последних 5000 строк
  (комбинация лаунчер + xray, раскрашенные)

## Команды разработки

```bash
# Билд
wails build -platform windows/amd64
wails build -platform darwin/arm64
wails build -platform linux/amd64

# Dev
wails dev

# Только Go-тесты (если будут)
go test ./core/...

# Lint
go vet ./...

# Чистый ребилд после смены sigantures в Go (Wails bindings)
wails build -clean -platform windows/amd64

# Frontend изолированно
cd frontend && npm run dev
cd frontend && npm run build
```

## Что было сделано / решения

- **Tray меню удалено** — getlantern/systray и fyne.io/systray оба флакают на
  Windows + UIPI после первого dismiss. Заменено на: иконка + клик = открыть
  окно. Quit — кнопка Power в TitleBar.
- **DPI/Frameless/Mica** — окно `Frameless: true`, `BackdropType: Mica` для
  Win11 эффекта прозрачности. Drag-region через `--wails-draggable: drag`.
- **Connectivity check delay 3s** — после Start/Restart ждём 3 секунды до
  пинга сервисов, чтобы TUN/маршруты стабилизировались.
- **Geo download через jsDelivr** — `cdn.jsdelivr.net/gh/runetfreedom/...`
  вместо `raw.githubusercontent.com` (тот rate-limited).
- **Stop xray перед UpdateGeo/DownloadXray** — иначе либо файл xray.exe
  залочен, либо весь download идёт через VPN тоннель.
- **wintun.dll эмбеднут** — `//go:embed assets/wintun.dll` + `extractWintun()`
  на старте. Юзеру не надо его вручную таскать.
- **Парсинг конфига через `map[string]interface{}`** — устойчив к разным типам
  (`port: 53` число vs `port: "443"` строка).
- **`Default` vs `Primary` outbound в ConfigInfo** — `Default` это первый в
  массиве (куда xray шлёт неcматченное), `Primary` — первый proxy outbound (для
  UI выделения). `ResolveOutbound` fallback на `Default`, не `Primary`.
- **Geosite hints хардкод** — мы НЕ парсим бинарный geosite.dat. Резолвер
  использует таблицу `geositeHints` для популярных категорий (`ru`,
  `ru-blocked`, `google`, `youtube`, `meta`, `twitter`, `discord`, `telegram`
  и т.д.). Совпадения помечаются `Confident: false`.

## TODO

- [ ] `platform/linux.go` — реальная реализация (см. шаги выше)
- [ ] `platform/darwin.go` — реальная реализация (см. шаги выше)
- [ ] `pack.sh` для Linux/macOS бандлов
- [ ] CI/CD — auto-build на push для трёх платформ
- [ ] Code signing для macOS .app и Windows .exe
- [ ] Subscription URL для конфигов (как в v2rayN/Hiddify)
