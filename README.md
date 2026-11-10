# YouTube Music Desktop Player

Rainmeter companion skin for YTMDesktop2. The clock skin is independent and untouched.

## Setup

Start guided setup from the repository root with `setup.bat` on Windows or `bash ./setup.sh` from Git Bash / WSL. Both launchers call `Scripts/Setup.ps1` through PowerShell. If YTMDesktop2 or Go is missing, setup asks whether to install it; YTMDesktop2 comes from the official stable GitHub release and Go is installed through winget. The script copies the single-panel skin and bridge into the detected Rainmeter installation, starts YTMDesktop2, guides API enablement and approval, stores the token in the current user's LocalAppData with a restricted file ACL, and starts the local bridge. Load `YouTube Music Desktop Player\YouTube Music Desktop Player.ini` through Manage Rainmeter.

The panel shows the current track and artwork, playback controls, a seek bar, and volume controls. The `⋯` menu includes Like, repeat, shuffle, Play queue, and Playlists.

### Local playlist bookmarks

1. Copy a YouTube Music playlist link, then open **⋯ → Playlists** and press **+**.
2. Click a bookmark to play it in YTMDesktop2. Hover over a row to rename or delete it. Press Enter to save a name or Esc to cancel. If more than six bookmarks are saved, scroll over the list.

Bookmarks are saved in `%LOCALAPPDATA%\AmbientMusicDashboard\playlists.json`. The bridge looks up display information through YouTube oEmbed and caches thumbnails from YouTube's image hosts. If no title is available, the bookmark is saved as **Playlist N**. The bookmark feature needs no Google API credentials and stores no track lists.

The separate quick slots in the More menu still read the first four entries in the installed `@Resources\Bridge\playlists.json`. Use a playlist URL or bare ID in `id` and a display label in `name`, then reopen the menu.

```json
{
  "playlists": [
    { "name": "My playlist", "id": "https://music.youtube.com/playlist?list=YOUR_PLAYLIST_ID" }
  ]
}
```

Click the queue icon beside `⋯` to open the current YTMDesktop2 queue below the player. Use the mouse wheel over the queue or the **Previous** and **Next** buttons to browse four tracks at a time. **Now playing** returns to the page containing the current track. Click a row to play it immediately. Selecting the immediately following track advances the current queue. Jumping to another track can rebuild the queue in YouTube Music.

### Import your playlists from Google

1. In a Google Cloud project, enable **YouTube Data API v3**. Set up Google Auth Platform and create an OAuth client of type **Desktop app**. If the project is in Testing, add your Google account as a test user. Download the client JSON and keep it outside the repository. [Google desktop OAuth setup](https://developers.google.com/identity/protocols/oauth2/native-app)
2. In PowerShell, from the repository root, run the following. Use the actual paths to your downloaded client JSON and installed skin's playlist settings.

   ```powershell
   cd Bridge
   go run . import-playlists 'C:\path\to\client_secret.json' 'C:\path\to\Rainmeter\Skins\YouTube Music Desktop Player\@Resources\Bridge\playlists.json'
   ```

3. Open the URL printed in the terminal in your browser and approve read access. Select a playlist for each menu slot or press Enter to keep its current value. Then reopen the Rainmeter menu.

The importer lists playlists owned by the authorized YouTube channel. It uses read-only access and keeps the Google access token in memory for that run. Playlists saved from other creators can still be added manually by ID. [YouTube playlist listing](https://developers.google.com/youtube/data/reference/rest/v3/playlists/list)

Enable YTMDesktop2's local API in **Settings → API & Integrations → API**, keep the bind address on `127.0.0.1`, and enable authorization when pairing. The setup script handles pairing and token storage. The bridge listens only on `127.0.0.1:38471`, calls YTMDesktop2's documented local API, and stores its token outside the skin/repository. The default YTMDesktop2 API port is `13091`; set `YTM_API_URL` if customized.

## Current scope and limits

Double-click the album cover or click its expand icon to open the desktop fullscreen skin. The regular panel is hidden while fullscreen is open and returns in place when you click the fullscreen background. Move the pointer to show the fullscreen controls. Artwork and scrim continue behind the clock. Player information and the progress bar stay outside the clock area.

Edit the installed `@Resources\Fullscreen.Settings.inc` to choose the Rainmeter monitor number and `FullscreenBackground` (`AlbumArt`, `None`, or `Custom`). For album art, set `FullscreenArtFit=Cover` to crop to the screen or use the default `Contain` to show the whole image over a blurred copy. `FullscreenArtAlign` sets the horizontal alignment in Contain mode: `Left` (default), `Center`, or `Right`.

The scrim starts at `FullscreenScrimStart=25` and reaches `FullscreenScrimOpacity=88` at `FullscreenScrimEnd=75`, measured in percent from the top. Reopen fullscreen after saving. For a custom background, copy `@Resources\Backgrounds\Custom.example.inc` to `Custom.inc`; its container now spans the entire screen. Setup preserves your settings.

Open **⋯ → Fonts** to pick fonts and weights for English and Japanese title and body text. The Windows font dialog's size, italic and color settings are ignored. **Reset to defaults** restores all four slots.

Search is not exposed by the documented YTMDesktop2 API. See [YTMDesktop2 routes](https://youtube-music.app/docs/api/routes/).
