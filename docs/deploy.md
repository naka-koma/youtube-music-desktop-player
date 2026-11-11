# リリース手順

プルリクエスト、`v` で始まるタグのプッシュ、または GitHub Actions の手動実行で Windows ビルドとテストが走ります。配布ZIPと `SHA256SUMS.txt` は、実行結果にある `windows-x64` アーティファクトから取得できます。

## GitHub Release の公開

リリースするコミットに `v1.2.3` 形式のタグを付け、そのタグをプッシュします。

```bash
git tag -a v1.2.3 <commit> -m "Release v1.2.3"
git push origin v1.2.3
```

GitHub Actions がテスト、ビルド、配布ZIPのインストール検証を実行します。検証が通ると、配布ZIPとチェックサムを添付した GitHub Release を公開します。

## ローカルで配布ZIPを作る

Windows x64 の開発環境で、リポジトリのルートから実行します。

```powershell
pwsh -File Scripts/Build-Release.ps1
```

`Bridge/dist/` に `youtube-music-desktop-player-windows-x64.zip` と `SHA256SUMS.txt` が生成されます。
