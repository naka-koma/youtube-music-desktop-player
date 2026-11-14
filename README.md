# YouTube Music Desktop Player

[日本語](README.ja.md)

A Windows Rainmeter skin for controlling YouTube Music through YTMDesktop2.

## Features

- View the current track and artwork, control playback and volume, and browse the play queue. The playback button shows Pause while playing and Play while paused.
- Save playlists as local bookmarks or quick slots, and import your own YouTube playlists into quick slots.
- Open a fullscreen artwork view on a monitor of your choice and customize the background.
- Customize fonts and choose Power Saver, Balanced, or Smooth rendering. Long titles scroll in Balanced and Smooth modes.

See the [player guide](docs/usage.md) for controls, playlists, and display settings.

## Setup

1. Install Rainmeter and launch it once.
2. Download and extract `youtube-music-desktop-player-windows-x64.zip` from [GitHub Releases](https://github.com/naka-koma/youtube-music-desktop-player/releases), then run `setup.bat`. The ZIP bundles `YouTubeMusicDesktopPlayerBridge.exe`, so the packaged install requires no Go toolchain.
3. Follow the prompts to enable the YTMDesktop2 local API under **Settings → API & Integrations → API**. Keep the bind address on `127.0.0.1` and enable authorization when pairing.
4. In Manage Rainmeter, load `YouTube Music Desktop Player\YouTube Music Desktop Player.ini`.

Setup installs the skin and bridge, starts YTMDesktop2 and the bridge, and guides you through pairing. It saves the API token outside the skin and repository, in the current user's LocalAppData, with restricted file permissions.

To install from source, run `setup.bat` from the checkout. Setup builds the bridge using Go.
