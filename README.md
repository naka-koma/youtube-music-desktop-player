# Ambient Music Dashboard

Rainmeter companion skin for YTMDesktop2. The clock skin is independent and untouched.

## Setup

Start guided setup from the repository root with `setup.bat` on Windows or `bash ./setup.sh` from Git Bash / WSL. Both launchers call `Scripts/Setup.ps1` through PowerShell. If YTMDesktop2 or Go is missing, setup asks whether to install it; YTMDesktop2 comes from the official stable GitHub release and Go is installed through winget. The script copies the single-panel skin and bridge into the detected Rainmeter installation, starts YTMDesktop2, guides API enablement and approval, stores the token in the current user's LocalAppData with a restricted file ACL, and starts the local bridge. Load `AmbientMusic\AmbientMusic.ini` through Manage Rainmeter.

The panel provides current track and artwork, transport, a seek bar that follows the pointer while dragging, ±10 seconds, a volume slider that follows the pointer while dragging and supports 5% mouse-wheel steps, mute, repeat, and shuffle. Click the `⋯` button to open the in-panel menu for Like and playlist selection. The menu's four playlist actions follow the order in `@Resources\Bridge\playlists.json`; update the `Text` values in `MeterPlaylist1` through `MeterPlaylist4` in `AmbientMusic.ini` if you rename the sample categories. Playlist IDs remain editable in JSON without rebuilding. Re-run setup to update the installed skin and bridge, then refresh the skin in Rainmeter.

Enable YTMDesktop2's local API in **Settings → API & Integrations → API**, keep the bind address on `127.0.0.1`, and enable authorization when pairing. The setup script handles pairing and token storage. The bridge listens only on `127.0.0.1:38471`, calls YTMDesktop2's documented local API, and stores its token outside the skin/repository. The default YTMDesktop2 API port is `13091`; set `YTM_API_URL` if customized.

## Current scope and limits

The skin uses a single persistent panel; there is no separate expanded view. Track data is polled every two seconds. Search is not exposed by the documented YTMDesktop2 API, so it is not included. YTMDesktop2 API reference: [Routes](https://youtube-music.app/docs/api/routes/).
