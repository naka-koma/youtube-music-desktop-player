# YouTube Music Desktop Player

[日本語](README.ja.md)

A Windows Rainmeter skin for controlling YouTube Music through YTMDesktop2.

- [Setup](#setup)
- [Using the player](#using-the-player)
- [Playlists](#playlists)
- [Display settings](#display-settings)

## Setup

1. Install Rainmeter and launch it once.
2. Download and extract `youtube-music-desktop-player-windows-x64.zip` from [GitHub Releases](https://github.com/naka-koma/youtube-music-desktop-player/releases), then run `setup.bat`. The ZIP bundles `YouTubeMusicDesktopPlayerBridge.exe`, so the packaged install requires no Go toolchain.
3. Follow the prompts to enable the YTMDesktop2 local API under **Settings → API & Integrations → API**. Keep the bind address on `127.0.0.1` and enable authorization when pairing.
4. In Manage Rainmeter, load `YouTube Music Desktop Player\YouTube Music Desktop Player.ini`.

Setup installs the skin and bridge, starts YTMDesktop2 and the bridge, and guides you through pairing. It saves the API token outside the skin and repository, in the current user's LocalAppData, with restricted file permissions.

To install from source, run `setup.bat` from the checkout. Setup builds the bridge using Go.

### Creating a release

Pushes to `main` and pull requests run the Windows tests and build the ZIP and SHA-256 checksum as Actions artifacts. Push a tag such as `v1.2.3` to publish a GitHub Release after the same checks pass. To build the distribution locally, run `pwsh -File Scripts/Build-Release.ps1` and find the files under `Bridge/dist/`.

### Local API connection

The bridge listens on `127.0.0.1:38471`. The default YTMDesktop2 API port is `13091`. If you changed it, set the bridge's `YTM_API_URL` environment variable to the new address.

## Using the player

### Playback controls

The panel shows the current track and artwork, playback controls, a seek bar, and volume controls. The `⋯` menu includes Like, repeat, shuffle, Play queue, and Playlists.

Search is not exposed by the documented YTMDesktop2 API. See [YTMDesktop2 routes](https://youtube-music.app/docs/api/routes/).

### Play queue

Click the queue icon beside `⋯` to open the current YTMDesktop2 queue below the player. Use the mouse wheel over the queue or the **Previous** and **Next** buttons to browse four tracks at a time. **Now playing** returns to the page containing the current track. Click a row to play it immediately. Selecting the immediately following track advances the current queue. Jumping to another track can rebuild the queue in YouTube Music.

### Fullscreen view

Double-click the album cover or click its expand icon to open the desktop fullscreen skin. The regular panel is hidden while fullscreen is open and returns in place when you click the fullscreen background. Move the pointer to show the fullscreen controls. The title uses a fixed font size. In Balanced and Smooth modes, long titles in both views scroll horizontally, pause at each end, and restart from the beginning when the track changes. Titles that fit stay still. You can also return to the panel with **Exit fullscreen** in the right-click menu.

Fullscreen opens on monitor 2 by default. Choose **Move to next monitor** in its right-click menu to change and save the target display. You can also edit `FullscreenMonitor` in `@Resources\Fullscreen.Settings.inc` and refresh the fullscreen skin. This setting takes precedence over coordinates in Rainmeter Manage. Fullscreen fetches playback independently of the compact panel.

## Playlists

### Local bookmarks

1. Copy a YouTube Music playlist link, then open **⋯ → Playlists** and press **+**.
2. Click a bookmark to play it in YTMDesktop2. Hover over a row to rename or delete it. Press Enter to save a name or Esc to cancel. If more than six bookmarks are saved, scroll over the list.

Bookmarks are saved in `%LOCALAPPDATA%\AmbientMusicDashboard\playlists.json`. The bridge looks up display information through YouTube oEmbed and caches thumbnails from YouTube's image hosts. If no title is available, the bookmark is saved as **Playlist N**. The bookmark feature needs no Google API credentials and stores no track lists.

### Quick slots

The quick slots in the More menu still read the first four entries in the installed `@Resources\Bridge\playlists.json`. Use a playlist URL or bare ID in `id` and a display label in `name`, then reopen the menu.

```json
{
  "playlists": [
    { "name": "My playlist", "id": "https://music.youtube.com/playlist?list=YOUR_PLAYLIST_ID" }
  ]
}
```

### Import from Google into quick slots

1. In a Google Cloud project, enable **YouTube Data API v3**. Set up Google Auth Platform and create an OAuth client of type **Desktop app**. If the project is in Testing, add your Google account as a test user. Download the client JSON and keep it outside the repository. [Google desktop OAuth setup](https://developers.google.com/identity/protocols/oauth2/native-app)
2. In PowerShell, from the repository root, run the following. Use the actual paths to your downloaded client JSON and installed skin's playlist settings.

   ```powershell
   cd Bridge
   go run . import-playlists 'C:\path\to\client_secret.json' 'C:\path\to\Rainmeter\Skins\YouTube Music Desktop Player\@Resources\Bridge\playlists.json'
   ```

3. Open the URL printed in the terminal in your browser and approve read access. Select a playlist for each menu slot or press Enter to keep its current value. Then reopen the Rainmeter menu.

The importer lists playlists owned by the authorized YouTube channel. It uses read-only access and keeps the Google access token in memory for that run. Playlists saved from other creators can still be added manually by ID. [YouTube playlist listing](https://developers.google.com/youtube/data/reference/rest/v3/playlists/list)

## Display settings

### Artwork cache

Generated blurred backgrounds and fullscreen artwork are stored under the current user's temporary directory, normally `%TEMP%\YouTubeMusicDesktopPlayer\artwork`, rather than `@Resources\Bridge`.

You can delete this cache folder; required images are regenerated. On startup, the bridge removes only recognized generated artwork files left in `@Resources\Bridge` by older versions. Custom images, settings, and saved playlists are preserved.

### Rendering performance

Right-click either view and choose a mode. The active option is marked `(selected)`. Both views use the same direct menu, with separators between rendering modes and other actions. Open **Rainmeter menu** at the bottom for standard Rainmeter settings such as window position.

| Mode | Rendering rate | Long titles |
| --- | --- | --- |
| `Power Saver` | 10fps | Static |
| `Balanced` (default) | About 15fps | Scroll horizontally |
| `Smooth` | About 30fps | Scroll horizontally |

The selection applies to both views and persists across restarts. Rendering rates are targets; actual cadence depends on your system. Smooth renders and samples the pointer more frequently, trading higher load for smoother motion and more responsive dragging. Playback information still refreshes roughly once per second, and playback controls remain available in every mode. Fullscreen updates only changed elements, so playback-time updates do not rebuild the background or title. CPU usage also depends on resolution, backgrounds, and other skins.

The menu saves the selection in `@Resources\Performance.Settings.inc`. Setup preserves this personal settings file. Timings come from bundled presets, so improvements apply without changing your saved mode.

For the component boundaries and maintenance rules, see the [rendering developer guide](docs/rendering.md) (Japanese).

### Fullscreen settings

Edit the installed `@Resources\Fullscreen.Settings.inc` to choose the Rainmeter monitor number and `FullscreenBackground` (`AlbumArt`, `None`, or `Custom`). For album art, set `FullscreenArtFit=Cover` to crop to the screen or use the default `Contain` to show the whole image over a blurred copy. `FullscreenArtAlign` sets the horizontal alignment in Contain mode: `Left` (default), `Center`, or `Right`.

The scrim starts at `FullscreenScrimStart=25` and reaches `FullscreenScrimOpacity=88` at `FullscreenScrimEnd=75`, measured in percent from the top. Reopen fullscreen after saving. For a custom background, copy `@Resources\Backgrounds\Custom.example.inc` to `Custom.inc`; its container now spans the entire screen. Setup preserves your settings.

### Fonts

Open **⋯ → Fonts** to pick fonts and weights for English and Japanese title and body text. The Windows font dialog's size, italic and color settings are ignored. **Reset to defaults** restores all four slots.
