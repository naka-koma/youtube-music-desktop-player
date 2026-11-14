# YouTube Music Desktop Player

[English](README.md)

YTMDesktop2 と連携する Windows 向けの Rainmeter スキンです。デスクトップから YouTube Music の再生を操作できます。スキンには曲名、アートワーク、再生キューが表示されます。

## 主な機能

- 曲名とアートワークを表示し、再生や音量の操作、再生キューの確認ができます。再生ボタンは再生中に一時停止、一時停止中に再生のアイコンを表示します。
- プレイリストをブックマークやクイックスロットに保存し、自分の YouTube プレイリストをクイックスロットに取り込めます。
- 好きなモニターにアートワークを全画面表示し、背景をカスタマイズできます。
- フォントを変更し、描画モードを Power Saver、Balanced、Smooth から選べます。Balanced と Smooth では長い曲名がスクロールします。

操作方法やプレイリスト、表示設定の詳細は [利用ガイド](docs/usage.ja.md) を参照してください。

## セットアップ

1. Rainmeter をインストールし、一度起動します。
2. [GitHub Releases](https://github.com/naka-koma/youtube-music-desktop-player/releases) から Windows x64 用の `youtube-music-desktop-player-windows-x64.zip` をダウンロードして展開し、`setup.bat` を実行します。配布ZIPには `YouTubeMusicDesktopPlayerBridge.exe` が入っているため、Goのインストールは不要です。
3. 案内に沿って YTMDesktop2 の「Settings → API & Integrations → API」でローカル API を有効にします。バインド先は `127.0.0.1` に設定します。認証が有効な場合は YTMDesktop2 でペアリングを承認します。
4. Rainmeter の管理画面で「スキン → YouTube Music Desktop Player」を開きます。次に `YouTube Music Desktop Player.ini` を読み込みます。

セットアップはスキンとブリッジを配置して起動します。認証トークンは現在の Windows ユーザーのローカルデータに保存します。ソースからインストールする場合も `setup.bat` を実行できます。ソースからは Go でブリッジをビルドします。
