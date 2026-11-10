# Ambient Music Dashboard

Rainmeter companion skin for YTMDesktop2. The clock skin is independent and untouched.

## Setup

Start guided setup from the repository root with `setup.bat` on Windows or `bash ./setup.sh` from Git Bash / WSL. Both launchers call `Scripts/Setup.ps1` through PowerShell. If YTMDesktop2 or Go is missing, setup asks whether to install it; YTMDesktop2 comes from the official stable GitHub release and Go is installed through winget. Use `-InstallYTMDesktop2` or `-InstallGo` to skip the corresponding prompt and install directly. The script preserves an existing playlist configuration, copies the skin and bridge into the detected Rainmeter installation, starts YTMDesktop2, guides API enablement and approval, stores the token in the current user's LocalAppData with a restricted file ACL, and starts the local bridge. It does not modify Rainmeter's global configuration; load `AmbientMusic\AmbientMusic.ini` once through Manage Rainmeter.

1. Enable YTMDesktop2's local API in **Settings → API & Integrations → API**. Keep its bind address on `127.0.0.1` and enable authorization.
2. Build the bridge: `go build -trimpath -o Bridge/dist/AmbientMusicBridge.exe .` from the `Bridge` directory, then copy `Bridge/playlists.json` beside the executable.
3. Copy the contents of `Rainmeter/AmbientMusic` to your Rainmeter `Skins/AmbientMusic` folder and the bridge executable plus playlist JSON to `Skins/AmbientMusic/@Resources/Bridge`.
4. Load `AmbientMusic\AmbientMusic.ini` in Rainmeter. The skin starts the bridge.
5. Pair the bridge once: POST `http://127.0.0.1:13091/auth/requestcode` with `{"appId":"ambientmusic","appName":"Ambient Music Dashboard","appVersion":"1.0.0"}`, approve it in YTMDesktop2 Settings → Authentication, then POST the returned code to `/auth/request`. Save the returned token (without quotes) to `%LOCALAPPDATA%\AmbientMusicDashboard\token`.
6. Replace the sample playlist IDs in `Bridge/dist/playlists.json` with IDs from YouTube Music URLs. Restart the bridge after changing the playlist list.

The bridge listens only on `127.0.0.1:38471`, calls only YTMDesktop2's documented local API, and stores its token outside the skin/repository. The default YTMDesktop2 API port is `13091`; set `YTM_API_URL` if customized. Launch Rainmeter at sign-in using its existing startup option. Add the skin to a Rainmeter layout if you want its position restored after a Rainmeter restart.

## Current scope and limits

Compact now-playing display, transport, playlist launch, and an expanded playlist chooser are included. The YTMDesktop2 API also documents seek, volume, shuffle, repeat and a WebSocket; these are not yet exposed in this first pass. Search is not listed among API routes, so a search UI would require a separate in-app webview/navigation design. Track snapshots currently poll at a low rate rather than using WebSocket events. The sample playlist entries are placeholders until configured.

API verification reference: [YTMDesktop2 Local API](https://youtube-music.app/docs/api/) and [Routes](https://youtube-music.app/docs/api/routes/).
