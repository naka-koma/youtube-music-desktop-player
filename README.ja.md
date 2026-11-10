# YouTube Music Desktop Player

[English](README.md)

YTMDesktop2 と連携する Windows 向けの Rainmeter スキンです。デスクトップから YouTube Music の再生を操作できます。スキンには曲名、アートワーク、再生キューが表示されます。

- [セットアップ](#セットアップ)
- [プレイヤーの使い方](#プレイヤーの使い方)
- [プレイリスト](#プレイリスト)
- [表示設定](#表示設定)

## セットアップ

1. Rainmeter をインストールします。
2. リポジトリのルートで `setup.bat` を実行します。Git Bash や WSL からは `bash ./setup.sh` でも起動できます。YTMDesktop2 や Go が見つからない場合は、セットアップがインストールするか確認します。
3. 案内に沿って YTMDesktop2 の「Settings → API & Integrations → API」でローカル API を有効にします。バインド先は `127.0.0.1` に設定します。認証が有効な場合は YTMDesktop2 でペアリングを承認します。
4. Rainmeter の管理画面で「スキン → YouTube Music Desktop Player」を開きます。次に `YouTube Music Desktop Player.ini` を読み込みます。

セットアップはスキンを配置し、Go ブリッジをビルドして起動します。認証トークンは現在の Windows ユーザーのローカルデータに保存します。

### API の接続先

ブリッジの待ち受け先は `127.0.0.1:38471` です。YTMDesktop2 の API ポートの既定値は `13091` です。ポートを変更した場合は、ブリッジの環境変数 `YTM_API_URL` に接続先を指定します。

## プレイヤーの使い方

### 再生操作

パネルで再生・一時停止、前の曲・次の曲への移動、シーク、音量調整ができます。「⋯」メニューから高評価、リピート、シャッフル、再生キュー、プレイリストにアクセスできます。

このプレイヤーは曲の検索に対応していません。 [YTMDesktop2 の API ルート](https://youtube-music.app/docs/api/routes/)

### 再生キュー

「⋯」の隣にあるキューアイコンを押すと、パネルの下に再生キューが開きます。マウスホイールや「Previous / Next」で4曲ずつ移動でき、「Now playing」で再生中の曲があるページに戻れます。

行をクリックすると、その曲を再生します。直後の曲を選ぶと現在のキューが進みます。別の曲を選んだ場合は、YouTube Music がキューを作り直すことがあります。

### 全画面表示

ジャケットをダブルクリックするか、右上の拡大ボタンを押すと全画面表示になります。マウスを動かすと操作ボタンが現れ、背景をクリックすると通常表示に戻ります。 タイトルの文字サイズは固定です。`Balanced` / `Smooth` では、通常表示・全画面表示ともに長いタイトルだけ横にスクロールします。先頭と末尾で少し停止し、曲が変わると先頭に戻ります。右クリックメニューの「Exit fullscreen」でも通常表示に戻れます。

表示先は初期設定で2番目のモニターです。右クリックメニューの「Move to next monitor」で切り替えられ、選択は保存されます。設定ファイル `@Resources\Fullscreen.Settings.inc` の `FullscreenMonitor` にモニター番号を指定することもできます。変更後はフルスクリーンのスキンを再読み込みしてください。Rainmeter 管理画面の座標指定より、この設定が優先されます。

フルスクリーンは再生情報を独立して取得するため、ミニウインドウの表示・更新に依存しません。

## プレイリスト

### ブックマーク

YouTube Music のプレイリスト URL をコピーし、**⋯ → Playlists → ＋** を押すとブックマークできます。

ブックマークの行をクリックすると YTMDesktop2 で再生します。行にカーソルを合わせると、名前の変更や削除ができます。名前の変更は Enter で確定し、Esc で取り消します。ブックマークが7件以上ある場合は、一覧をスクロールできます。

ブックマークの保存先は `%LOCALAPPDATA%\AmbientMusicDashboard\playlists.json` です。表示名は YouTube の oEmbed から取得し、取得できない場合は「Playlist N」として保存します。サムネイルは YouTube の画像配信先から取得してキャッシュします。ブックマークに Google API の認証情報は不要で、曲の一覧も保存しません。

### クイックスロット

「⋯」メニューのクイックスロットには、スキンの `@Resources\Bridge\playlists.json` にある先頭4件が表示されます。`id` にプレイリストの URL または ID、`name` に表示名を指定し、メニューを開き直してください。

```json
{
  "playlists": [
    { "name": "作業用BGM", "id": "https://music.youtube.com/playlist?list=YOUR_PLAYLIST_ID" }
  ]
}
```

### Google アカウントからクイックスロットに取り込む

1. Google Cloud プロジェクトで「YouTube Data API v3」を有効にします。続いて Google Auth Platform で「デスクトップ アプリ」用の OAuth クライアントを作成します。プロジェクトがテスト中の場合は、利用する Google アカウントをテストユーザーに追加します。ダウンロードしたクライアント JSON はリポジトリの外に保存してください。 [Google のデスクトップアプリ向け OAuth ガイド](https://developers.google.com/identity/protocols/oauth2/native-app)
2. リポジトリのルートから PowerShell で次を実行します。ファイルのパスは実際の場所に置き換えてください。

   ```powershell
   cd Bridge
   go run . import-playlists 'C:\path\to\client_secret.json' 'C:\path\to\Rainmeter\Skins\YouTube Music Desktop Player\@Resources\Bridge\playlists.json'
   ```

3. ターミナルに表示された URL をブラウザで開き、読み取りアクセスを承認します。各スロットに入れるプレイリストを選びます。既存の値を残す場合は Enter を押します。完了後、Rainmeter のメニューを開き直してください。

取り込みの対象は、認証した YouTube チャンネルが所有するプレイリストです。他のクリエイターのプレイリストは URL や ID で追加できます。Google のアクセストークンは取り込みの実行中だけメモリに保持します。

## 表示設定

### 描画負荷

通常表示・全画面表示の右クリックメニューから選べます。現在のモードには `(selected)` が付きます。両方の表示で共通のメニューが直接開き、描画モードとその他の操作は区切り線で分かれています。最前面などの Rainmeter 標準設定は、メニュー末尾の「Rainmeter menu」から開けます。

| モード | 描画頻度 | 長いタイトル |
| --- | --- | --- |
| `Power Saver` | 10fps | 静止 |
| `Balanced`（既定） | 約15fps | 横スクロール |
| `Smooth` | 約30fps | 横スクロール |

選択は両方の表示に反映され、再起動後も保持されます。描画頻度は目安で、実際の頻度はPC環境によって異なります。`Smooth` は描画とマウス位置の取得を高頻度で行うため、滑らかさを優先する分、負荷が増えます。描画頻度が低くても、再生情報は約1秒ごとに取得し、操作ボタンは引き続き使えます。全画面では変化した部分だけを更新し、再生時間の更新で背景やタイトルを作り直しません。CPU 使用率は解像度、背景、他のスキンにも左右されます。

保存先は `@Resources\Performance.Settings.inc` です。このファイルはメニューから設定され、セットアップを再実行しても保持されます。更新間隔などは同梱プリセットから読み込むため、保存したモードを変えずに改善が反映されます。

### 全画面の設定

全画面の設定はインストール先にある `@Resources\Fullscreen.Settings.inc` で変更します。保存したら、全画面を開き直してください。

| 設定 | 内容 |
| --- | --- |
| `FullscreenMonitor` | 表示先の Rainmeter モニター番号 |
| `FullscreenBackground` | `AlbumArt` はアートワーク、`None` は背景なし、`Custom` はカスタム背景 |
| `FullscreenArtFit` | `Contain`（既定）は画像全体をぼかし背景の上に表示。 `Cover` は画面を埋めるように拡大し、はみ出す部分を切り抜く |
| `FullscreenArtAlign` | `Contain` の横位置を `Left`（既定）、`Center`、`Right` から選択 |
| `FullscreenScrim` | 暗幕を `1` で表示し、`0` で非表示 |
| `FullscreenScrimStart` / `FullscreenScrimEnd` | 画面上端から何%の位置で暗幕を始め、何%の位置で最大の濃さにするか。既定値はそれぞれ `25` と `75` |
| `FullscreenScrimOpacity` | 暗幕の最大不透明度。既定は `88`% |

カスタム背景を使う場合は、`@Resources\Backgrounds\Custom.example.inc` を同じフォルダに `Custom.inc` としてコピーして編集します。セットアップを再実行しても、個人設定は引き継がれます。

### フォント

「⋯ → Fonts」では曲名とその他の文字について、英語・日本語それぞれのフォントと太さを選べます。Windows のフォント選択画面で指定したサイズ、斜体、色は適用されません。「Reset to defaults」で4枠すべてを既定値に戻せます。
