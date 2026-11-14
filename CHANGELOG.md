# Changelog

## [1.0.1] - 2026-11-14

- Fix the mini-player playback button. It now shows Pause while playing and Play while paused, and redraws when the playback state arrives. ([#1](https://github.com/naka-koma/youtube-music-desktop-player/issues/1))
- Move YTMDesktop2 playback integration behind an internal Go interface while preserving the Rainmeter skin's local HTTP API.

## [1.0.0] - 2026-11-11

First public release for Windows x64.

- Control YouTube Music from Rainmeter through YTMDesktop2, with playback, seeking, volume, and access to the play queue.
- Save playlists as local bookmarks or quick slots. Import playlists you own on YouTube into quick slots.
- Show album artwork and playback controls in fullscreen on a chosen monitor, with customizable backgrounds and fonts.
- Choose Power Saver, Balanced, or Smooth in either view. Long titles scroll in Balanced and Smooth, and Smooth targets about 30 fps.
- The skin starts the bridge automatically and stores generated artwork in the Windows temporary folder.
- The release ZIP includes the bridge and `setup.bat`. See the [setup instructions](README.md#setup) and [player guide](docs/usage.md).

[1.0.1]: https://github.com/naka-koma/youtube-music-desktop-player/releases/tag/v1.0.1
[1.0.0]: https://github.com/naka-koma/youtube-music-desktop-player/releases/tag/v1.0.0
