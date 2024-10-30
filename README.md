# PitTouchPro3 Simulator (pro3sim)

[ピットタッチ･プロ3](https://www.sstinc.co.jp/products/pittouch-pro3/)(以下Pro3)上で動作するアプリ(コンテンツセット)の
開発をサポートするツールです。

本シミュレーターを使うと、PC上のwebブラウザーでコンテンツセットの動作確認とデバッグを行えます。

# シミュレートするPro3の機能

- カードのタッチ
- ネットワーク状態の変化
- WebSQL
- ファイル操作 (profileoperate.js)

# 準備

以下のソフトウェアをインストールしてください。

- Docker
- Docker-Compose

Windowsの場合は、[Docker Desktop](https://docs.docker.com/desktop/)をインストールするのが簡単です。Docker Desktopには、DockerとDocker-Composeが含まれています。


# 実行方法

ソースコードをcloneしたディレクトリでコマンドラインから、

```
docker compose up
```

を実行すると、dockerイメージの構築とpro3simのビルドを行います。
ビルド後はpro3simのHTTPサーバーがport 8889で起動します。

webブラウザーで http://localhost:8889/ にアクセスすると、`volume/cts/index.html`を表示しますので、
コンテンツセットのファイルを`volume/cts/`に配置してください。

pro3simは、デフォルトでは以下のファイルとディレクトリを使用します。

- WebSQLのデータベースファイルは、`volume/db/` に保存します。
- `profileoperate.js` でのファイル操作は、`volume/fileOperateDir/` に行います。
- `providersetting.xml` は、`volume/providersetting.xml` を使用します。 

各種ディレクトリやポート番号は、docker-compose.yml で変更できます。


## カードのタッチをシミュレートする

サーバーのAPI`/pjf/api/eventTrigger`にアクセスすることで、`startCommunication()`のイベントを発生できます。
`/pjf/api/eventTrigger`へのアクセス方法は、`tools/touch.sh`を参照してください。

## ネットワーク状態の変化をシミュレートする

サーバーのAPI`/pjf/api/eventTrigger`にアクセスすることで、`startEventListen()`のイベントを発生できます。
`/pjf/api/eventTrigger`へのアクセス方法は、以下のスクリプトを参照してください。

- `tools/net_off.sh`
- `tools/net_lan.sh`
- `tools/net_mobile.sh`
- `tools/net_wlan.sh`



# 注意事項

- Pro3が提供するAPIのうちシミュレートしていないAPIは0を返すだけのモックです。中身は `pjf/prooperate.js` を参照してください。
- Pro3の動作を正確に再現しているわけではないため、実機での動作と異なる可能性があります。


# サポート

github上ではサポートは受け付けていません。バグ報告や機能追加のリクエストは弊社のサポート窓口までお願いいたします。


# ライセンス

GPLv3です。

