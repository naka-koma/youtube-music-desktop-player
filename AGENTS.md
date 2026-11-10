# 作業ガイド

## 構成

- Windows 用の Rainmeter スキンと、YTMDesktop2 のローカル API を仲介する Go ブリッジ。
- `Rainmeter/YouTube Music Desktop Player/` が配布するスキンのソース。
- 通常表示はルートの INI、全画面は `Fullscreen/Fullscreen.ini`。共通 Lua と設定は `@Resources/`。
- `Bridge/` は Go 1.23 以降を使用。`127.0.0.1:38471` でスキン向けの API を提供する。
- `scripts/Install-PlayerSkin.ps1` はビルドと配置、`scripts/Setup.ps1` は初回セットアップを担当する。
- 描画部品の構成と責任は `docs/rendering.md` にまとめている。スクロール計算は `@Resources/Components/Marquee.lua`、再描画を含むRainmeter操作は `Components/Renderer.lua` に集約する。

## 編集と個人設定

- インストール先だけを直さず、先にリポジトリのソースを変更する。
- 既存の Rainmeter ファイルは UTF-16 LE が中心。文字コードと改行を保ち、ファイル全体を不要に変換しない。新規の ASCII Lua は UTF-8 でもよい。
- `Fonts.User.inc`、`Fullscreen.Settings.inc`、`Performance.Settings.inc`、`Backgrounds/Custom.inc` は個人設定。インストール時に保持し、Git に追加しない。
- 認証トークン、OAuth クライアント JSON、生成された EXE、キャッシュ画像、ブックマークの実データをコミットしない。
- UI のメニューは英語。利用者向けの変更は `README.md` と `README.ja.md` の両方に反映する。

## 描画と操作

- 描画負荷の選択肢は `Power Saver / Balanced / Smooth`。選択は通常表示と全画面で共有し、再インストールでも保持する。
- 各モードの更新間隔・取得周期は `@Resources/Performance.Profiles/` を正とする。個人設定には選択モードだけを書き、古い個人設定に残った派生値はプリセットで上書きする。
- 描画モードは通常・全画面とも右クリックで直接選べるようにする。両方で `!SkinCustomMenu` を使い、描画モードを先頭の3項目にそろえる。操作の種類ごとに区切り線を入れ、標準設定への入口として末尾に `Rainmeter menu` を残す。
- 更新間隔を変える場合は、WebParser の取得周期、ホバー待ち、シーク待ち、操作ボタンのフェード時間も確認する。秒数をフレーム数だけで決めない。
- 長いタイトルのスクロールでは文字のレイアウトを毎フレーム作り直さない。描画済みの文字を移動し、表示領域をコンテナーで制限する。
- 全メーターが `UpdateDivider=-1` のスキンは、Lua の `SetX` だけでは再描画されない。スクロールには再描画も要求する `!MoveMeter` を使う。Lua の実行頻度を実際の描画fpsと取り違えない。
- 静的な画像・マスクは `UpdateDivider=-1` を使い、必要な時だけ明示的に更新する。毎フレームの `!Redraw` を追加する前に、スキンの通常更新と重複しないか確認する。
- 全画面の再生位置更新で `FSContent` 全体を更新しない。タイトル、時刻、操作ボタン、背景は変更のあった部分だけを更新する。
- タイトルの文字サイズは固定。短いタイトルを自動拡大しない。
- 全画面の再生情報とブリッジの復旧を、通常表示が読み込まれていることに依存させない。
- 全画面には背景クリックと右クリックの `Exit fullscreen` を残す。シーク操作の終了クリックで誤って閉じないようにする。
- モニター番号は Rainmeter の番号。保存された配置設定、負の座標、モニター不足のフォールバックを考慮する。

## 確認

- Go の変更は `Bridge/` で `go test ./...` と `go vet ./...` を実行する。
- Lua と INI の変更は、可能なら実際の Rainmeter でも確認する。通常・全画面、長短のタイトル、設定の再読込、ミニ側の読み込み解除を確認する。
- 描画設定は3モードの切り替えと保存を確認し、更新頻度を変えても再生情報や操作が止まらないことを確認する。
- `Scripts/tests/rendering.lua` は Lua 5.1 用の回帰テスト。`dofile('.../Scripts/tests/rendering.lua')('.../リポジトリのルート')` で実行でき、Rainmeter の Lua からも呼び出せる。
- `Scripts/tests/fullscreen.lua` では部分更新、背景到着時の更新、不要な再描画の抑止、シーク後の閉じる判定を確認する。同じ方法で実行できる。
- `Scripts/tests/components.lua` では部品単体の時間計算、座標の丸め、再描画要求、不要な更新の省略を確認する。
- CPU 比較は同じ表示モード・曲・測定時間で行う。Rainmeter 全体の値には他のスキンも含まれるため、単発の数値から改善率を断定しない。
- 滑らかさの確認では描画要求の発生・フレーム間隔の分布・最大停止時間・マウス位置の取得周期も確認する。Smooth の目標は約30fps。描画だけを速めて入力取得が約100msのままにならないようにする。
- `git diff --check` を実行し、実際に確認できた結果と未再現の問題を区別して報告する。
- 配置には `pwsh -NoProfile -File scripts/Install-PlayerSkin.ps1 -RainmeterRoot '<Rainmeter のフォルダー>'` を使用できる。個人設定を保ち、確認後に表示先やモードを元に戻す。
