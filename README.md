# logs-ecstask

ECS タスクのログを CloudWatch Logs から簡単に閲覧するための CLI ツール

## 機能

- ECS クラスターとタスクの対話的な選択
- CloudWatch Logs からのログ取得と表示
- ECSサービスイベントの表示
- ページング機能付きのタイムライン表示

## インストール

```bash
brew install takayanagishinnosuke/tap/logs-ecstask
```

## 使い方

```bash
logs-ecstask [options]

Options:
  -profile AWS プロファイル名を指定 (指定しない場合はデフォルト)
  -cluster ECS クラスター名を指定 (指定し無い場合は選択)
  -task ECS タスク IDを指定 (指定し無い場合は選択)
```

## ライセンス
MIT
