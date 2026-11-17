# Smart Go-YAML Engine

[![Go CI/CD Pipeline](https://github.com/cybervask/yaml/actions/workflows/go.yml/badge.svg)](https://github.com/cybervask/yaml/actions/workflows/go.yml)

その他の言語: [English (EN)](README.md) | [Русский (RU)](README.ru.md) | [中文 (ZH)](README.zh.md) | [Español (ES)](README.es.md)

`go.yaml.in/yaml/v4` の堅牢で高性能なドロップイン代替（drop-in replacement）です。元の関数シグネチャと主要オプション（`Option`）をそのまま保持しながら、深い再帰的なデフォルト値の適用（`default:"value"`）、OS 環境変数のネイティブ注入（`env:"VAR"`）、CLI ヘルプの自動生成（`Help()`）、タグベースの厳格な検証（`validate:"..."`）を解析レイヤーに自動的に統合します。

---

## 利用可能な検証ルールと機能

`validate` タグ内では、カンマ区切りで複数の検証ルールを組み合わせられます（例：`validate:"not_empty,endpoint"`）。

> **循環構造に関する注意：** プログラム的に構築された循環ポインタ構造（例：自分自身を参照するノード）は `Validate` をハングさせません — 現在のパスに沿った再帰は、参照元フィールドに対して明示的な `cyclic structure detected` エラーを返します。複数のフィールドから参照される共有（非循環）サブオブジェクトは有効なままです。`Help` 出力では、自己参照型はパスごとに 1 回だけ記録され、再帰分岐は黙って省略されます。

> **境界リミットに関する注意：** 解析できないリミット値（例：`validate:"minlen=abc,maxlen=5"`）は、ルール名を明示した設定エラー（`invalid validator configuration: minlen "abc": ...`）として報告されます — 無効なリミットが自分のチェックを黙って無効化することはありません。数値型以外（例：`bool`）に適用された境界ルール（`min`、`max`、`gt`、`lt`）も同様です：チェックを適用できないため、黙ってスキップするのではなく設定エラーとして報告されます。



| ルール / 機能             | 構文例                             | 説明                                                                                                                                       | 対応型                                       |
|:--------------------------|:-----------------------------------|:-------------------------------------------------------------------------------------------------------------------------------------------|:--------------------------------------------|
| **環境変数注入**          | `env:"APP_PORT"`                   | **12-Factor App:** OS の環境変数を動的に注入します。デフォルトでは環境変数が YAML 値とタグのデフォルト値に対して絶対的な優先度を持ちます。ソースの仲裁は `priority` タグで利用できます（**ソース優先度**を参照）。`default` と `env` タグはスカラーフィールドに適用されます。`time.Duration` 値に対応します。ロードパス（`LoadFile`、`Load`、`Loader.Load`、`Decoder.Decode`）では無効な env 値はエラーとして報告されます。単体の `SetDefaults` 呼び出しは旧来のサイレント動作を保ちます — 無効な env 値はエラーなしでフィールドをゼロのままにし、`default` タグは適用されません（唯一の例外：無効な `time.Duration` リテラルはエラーとして報告されます）。 | `string`, `bool`, 基本数値型, `time.Duration` |
| **ソース優先度**          | `priority:"value"`                 | **env 仲裁タグ:** `env` タグも持つフィールドに適用されます。取れる値は `value` または `env` です。デフォルト（タグなしまたは `priority:"env"`）では、環境変数が YAML 値と `default` タグに対して絶対的な優先度を持ちます。`priority:"value"` を設定すると、デフォルト適用とデコードの後に非ゼロになるフィールド — デコードされた YAML 値、適用された非ゼロの `default` タグ、プログラム的に事前入力された値 — が空でない環境変数に勝ります。ソースが 1 つしか存在しない場合は、タグに関係なくそのソースが適用されます。デフォルト適用とデコードの後もゼロのままであるフィールド（YAML の明示的ゼロやゼロ値の `default` タグ、例：`default:"0"`、`default:"false"`、`default:""`）は仲裁の値として扱われません — 環境変数が適用されます。未知のタグ値と `env` を持たない `priority` タグは、値や環境変数の有無にかかわらず設定エラーとして報告されます。`default` や `env` と同様に、`priority` タグはスカラーフィールドに適用されます。非スカラーフィールド（struct、slice、map）に適用すると設定エラーとして報告されます。 | `string`, `bool`, 基本数値型, `time.Duration` |
| **必須フィールド**        | `validate:"required"`              | フィールドに非ゼロ値が割り当てられていることを保証します。`default` タグと相互排他です。`not_empty` は旧来のエイリアスです（後方互換のために保持、非推奨化が予定されています）。 | `string`, `struct`, `slice`, `map`, 数値型 |
| **条件付き必須**          | `validate:"required_if=role!=srv"` | **クロスフィールド検証：** 別のフィールドの値に応じて現在のフィールドが必須になります。正規演算子：`==`（等しい）、`!=`（等しくない）— 比較は大文字小文字を区別しません。`~=`（正規表現マッチ）と `^=`（マッチしない）。正規演算子のオペランドは引用符（`'...'` / `"..."`）で囲むことができ、引用符は剥がされます。単純なターゲット名はまず自身を囲む構造体内で解決され、次にルートへフォールバックします。ドットパス（`required_if=server.mode==prod`）はルートからアドレス指定します。ポインターターゲットは自動的にデリファレンスされます。存在しないターゲット、エクスポートされていないターゲット、無効なパターンは設定エラーとして報告されます。認識可能な演算子を 1 つも含まない `required_if` 値は設定エラーとして報告されます。旧来のコロン構文（`key:value`、`key:!value`、マクロ `:empty`/`:not_empty`）は非推奨ですが、引き続きサポートされます。旧来のアトムと正規のアトムは同じ式内で混在できます。複数の条件をサポートしています：条件を `&&`（AND）と `\|\|`（OR）で組み合わせ、括弧でグループ化できます — `&&` の優先度は `\|\|` より高くなります。条件は、`&&` または `\|\|` が引用符の外に存在する場合、または値が引用符なしの `(` で始まる場合にのみ式として解析されます —— 括弧だけで単一条件を囲むことができます。それ以外の場合は、単一条件の構文とメッセージがそのまま維持されます。`&&`、`\|\|`、括弧の周囲の空白は無視されます。アトム内の比較演算子（`==`、`!=`、`~=`、`^=`）の周囲に空白を入れることはできません — 空白を含むターゲット名は解決されず、`target field "..." not found` として報告されます。`&&` または `\|\|` を含むオペランドは引用符で囲む必要があります：`webhook.url~='(a\|b)'`。引用符で囲まれたセグメントはリテラルとして扱われます。オペランド内の括弧（比較演算子の後ろ、例：`tag~=(a\|b)`）には引用符は不要です。同じフィールドに `required_if` を 2 回宣言するのは設定エラーです —— 条件は `&&` や `\|\|` で 1 つの式にまとめてください。`&&` や `\|\|` を含む非推奨の legacy 値は式として解析されます（例：`key:a&&b` は `key:a` と不正なアトムの AND になります）。単一のリテラル比較としては解析されません。同じフィールド上の数量ルール（`min`、`max`、`minlen`、`maxlen`、`mincount`、`maxcount`）は式の結果に関係なくチェックされます —— 式が false でも空フィールドの `minlen` は報告されます。式の評価は網羅的です：形状エラー（括弧の不一致、空のアトム）とアトムの設定エラーは、フィールド値や最終結果に関係なく報告され、誤りのあるアトムは false として扱われます。同一のアトムエラーの重複は 1 回だけ報告されます。違反は `field <名前>: is required when <式>` として報告されます（例：`field key_file: is required when mode==webhook&&(url~=^https:\|\|cert^=^$)`）。 | `string`, 数値, `bool`, ポインタ型など       |
| **ホワイトリスト**        | `validate:"choice=dev,prod"`       | **ホワイトリストモード：** 文字列値はカンマ区切りトークンの 1 つと完全一致しなければなりません。スライス要素を再帰的にサポートします。 | `string`, `[]string`                        |
| **ブラックリスト**        | `validate:"choice=!red,!black"`    | **ブラックリストモード：** 先頭に `!` が付いた特定の除外トークンを除く任意の文字列値を許可します。                                     | `string`, `[]string`                        |
| **数値と時間の範囲**      | `validate:"min=1s,max=10m"`        | **含む境界**の下限（`>=`）と上限（`<=`）をアトミックに強制します。数値と `time.Duration` の時間間隔をネイティブにサポートします。       | `int`, `uint`, `float`, `time.Duration`     |
| **厳格比較**              | `validate:"gt=5,lt=10"`            | **含まない**厳格な「より大きい」（`>`）と「より小さい」（`<`）の境界を強制します。`time.Duration` もサポートします。                    | `int`, `uint`, `float`, `time.Duration`     |
| **文字列長（ルーン）**    | `validate:"minlen=3,maxlen=20"`    | 文字列長の下限と上限を強制します。生のバイト数ではなく **Unicode ルーン（文字数）** をプロフェッショナルに数えます。                        | `string`                                    |
| **コレクション容量**      | `validate:"mincount=1,maxcount=5"` | 動的なコレクション内の要素数の最小値と最大値を強制します。                                                                                  | `slice`, `map`                              |
| **ネットワーク形式**      | `validate:"format=ipv4"`           | 特定の IP ネットワークレイアウトを検証します。`format=ip`、`format=ipv4`、`format=ipv6` に対応します。`endpoint` と `url` と相互排他です。 | `string`                                    |
| **16進エンコーディング**  | `validate:"format=hex"`            | 16 進文字列を検証します：偶数桁で `hex.DecodeString` によりデコード可能であること。奇数長と非 16 進文字は拒否されます。             | `string`                                    |
| **UUID 一意識別**         | `validate:"format=uuid"`           | UUID の正規形式（ハイフン付き 16 進 8-4-4-4-12）を検証します。バージョンとバリアント（RFC 4122）は検証しません。                        | `string`                                    |
| **正規表現**              | `validate:"regexp=^[a-z]{2,4}$"`   | 文字列が正規表現パターンに一致するかを検証します。埋め込まれたカンマに対して安全です。外側のクォート（`'...'` または `"..."`）はパターンから剥がされます。リテラルのクォートにマッチさせるには、反対の種類のクォートを使います（例：`regexp="^'q'$"`）。 | `string`, `[]string`                        |
| **エンドポイント**        | `validate:"endpoint"`              | 標準的なネットワークエンドポイント（`host:port`）を強制します。**IPv6** 構文とポート境界（1-65535）をネイティブにチェックします。           | `string`                                    |
| **URL 検証**              | `validate:"url"`                   | URI を検証します。プロトコルスキームの明示的な区切り（例：`http://`、`grpc://`）と空でないホストを厳格に要求します。スキームのみの値（`http://`）は拒否されます。 | `string`                                    |
| **インスタンス単位のパスルール** | `validate:"min_len.min=0,max_len.max=64"` | **共有ネスト型：** `<yamlパス>.<ルール>=<値>` 構文で、このインスタンスのネストされたフィールドにルールを適用します。親インスタンスごとに独自の境界を持ちます。存在しないパスはエラーとして報告されます。パスルールの値にはドットを含められます。 | ネストされた struct フィールド |

---

## ロードとダンプ

- **オプションはエンドツーエンドで有効：** `Load` / `LoadFile` に渡されたすべてのオプション（例：`WithKnownFields`、`WithUniqueKeys`）は AST 解析段階とターゲット構造への最終デコード段階の両方に適用されます — 厳格チェックは upstream の `yaml4.Load` とまったく同じように動作します。フォーマット専用のオプション（`WithIndent` など）はロードに影響しません。
- **`!include` の書き戻しはオブジェクトに紐付きます：** `DumpWithInclude` は、まさにそのオブジェクトのロード時に記録された include データから書き戻しパスを取得します — `Load` / `LoadFile` に渡したのと同じポインターを `DumpWithInclude` に渡してください。このようなロードを経ていない構造体（またはロード済み値のコピー）はセーフモードでダンプされます：値はインライン出力され、`!include` マーカーは合成されず、ファイルは一切変更されません。
- **`!include` はすべてのロードパスで解決されます：** `LoadFile` は `!include` ディレクティブを設定ファイルのディレクトリ基準で解決し、`Load`、`Loader.Load`、`Decoder.Decode` は現在の作業ディレクトリ基準で解決します。ストリームパスはターゲットに include セッションをバインドせず、以前 `Load`/`LoadFile` でロードされたポインターのストリームロードが成功すると、以前の紐付けは解除されます：オブジェクトは inline safe mode でダンプされます（上記の書き戻しの箇条書きを参照）。
- **include ターゲットは 1 ドキュメントのみ：** `!include` のターゲットファイルは正確に 1 つの YAML ドキュメントを含む必要があります — 末尾の `---` ドキュメントマーカーがあるとロードは `included file ... contains multiple documents` で失敗します。メインの設定は upstream yaml4 のセマンティクスに従います：ワンショットパス（`LoadFile`、`Load`、`Unmarshal`）は末尾の `---` マーカーも拒否し（`expected single document, found multiple`）、ストリームパス（`Loader.Load`、`Decoder.Decode`）は最初のドキュメントのみを読み、残りを黙って破棄します。

---

## 設定プロファイル例（有効と無効のシナリオ）

### 例 1：CLI ヘルプ出力を統合した標準アプリケーション設定（有効シナリオ）
**入力 YAML スペック（`config.yaml`）：**
```yaml
env: "prod"
api_url: "https://cybervask.net"
api_endpoint: "127.0.0.1:443"
server:
  logging:
    colors: true
    level: "warn"
  allowed_ips:
    - "192.168.1.1"
```

**Go アプリケーション設定モデル：**
```go
package main

import (
	"fmt"
	"time"

	"github.com/cybervask/yaml"
)

type TLS struct {
	MinVersion string   `yaml:"min_version" default:"tls1.3" validate:"choice=tls1.2,tls1.3" env:"TLS_MIN_VERSION" description:"サポートされる最小 TLS バージョン"`
	Alpn       []string `yaml:"alpn" default:"h2,http/1.1" validate:"choice=h2,http/1.1" description:"アプリケーション層プロトコルネゴシエーション ALPN リスト"`
}

type Logging struct {
	Level         string           `yaml:"level" default:"info" validate:"choice=debug,info,warn" description:"ログ階層の実行レベル"`
	Colors        bool             `yaml:"colors" description:"ANSI カラー付きログ出力の有効化"`
	Timeout       string           `yaml:"timeout" default:"5s" description:"内部の下流処理タイムアウト"`
}

type Config struct {
	Env          string        `yaml:"env" default:"dev" validate:"choice=dev,stage,prod" description:"アプリケーション実行環境"`
	AppName      string        `yaml:"app_name" default:"api" validate:"minlen=3,maxlen=10" description:"プログラム内部識別エイリアス"`
	APIUrl       string        `yaml:"api_url" validate:"url,required_if=env==prod" env:"API_URL" description:"対象 API のベースリンク"`
	APIEndpoint  string        `yaml:"api_endpoint" validate:"endpoint" description:"ローカルバインドネットワークソケット"`
	Timeout      time.Duration `yaml:"timeout" default:"5s" validate:"min=1s,max=10m" description:"グローバルリクエスト境界追跡パラメータ"`
	Crypto       TLS           `yaml:"crypto" description:"TLS セキュリティ構造設定レイヤー"`
	Server       struct {
		Logging    Logging  `yaml:"logging" description:"サーバーロギング設定パラメータ"`
		AllowedIPs []string `yaml:"allowed_ips" validate:"mincount=1,maxcount=10" description:"信頼するリモート接続のホワイトリスト"`
	} `yaml:"server"`
}

func main() {
	var cfg Config
	yaml.Help(cfg)
}
```

**自動生成される対話型 CLI ヘルプ出力 (`yaml.Help(cfg)`):**

自己参照の構造体型はパスごとに 1 回だけ記録されます — 再帰分岐は黙って省略されるため、`Help` は常に終了します。
```text
yaml configuration schema documentation:

env:             アプリケーション実行環境 (default: dev, validate: [choice=dev,stage,prod])
app_name:        プログラム内部識別エイリアス (default: api, validate: [minlen=3,maxlen=10])
api_url:         対象 API のベースリンク (env: API_URL, validate: [url,required_if=env==prod])
api_endpoint:    ローカルバインドネットワークソケット (validate: [endpoint])
timeout:         グローバルリクエスト境界追跡パラメータ (default: 5s, validate: [min=1s,max=10m])
crypto:          TLS セキュリティ構造設定レイヤー
  min_version:   サポートされる最小 TLS バージョン (env: TLS_MIN_VERSION, default: tls1.3, validate: [choice=tls1.2,tls1.3])
  alpn:          アプリケーション層プロトコルネゴシエーション ALPN リスト (default: h2,http/1.1, validate: [choice=h2,http/1.1])
server:
  logging:       サーバーロギング設定パラメータ
    level:       ログ階層の実行レベル (default: info, validate: [choice=debug,info,warn])
    colors:      ANSI カラー付きログ出力の有効化
    timeout:     内部の下流処理タイムアウト (default: 5s)
  allowed_ips:   信頼するリモート接続のホワイトリスト (validate: [mincount=1,maxcount=10])
```

---

### 例 2：ホワイトリスト制約違反（無効シナリオ）
**入力 YAML スペック（`config.yaml`）：**
```yaml
env: "testing" # エラー: "testing" はホワイトリスト [dev, stage, prod] の範囲外です
```
**アプリケーションランタイムが返すエラー文字列 (`err.Error()`):**
```text
field env: value "testing" is invalid; allowed choices are [dev, stage, prod]
```

---

### 例 3：複数の異常、文字列長、コレクションサイズ、時間制約の同時検出（無効シナリオ）
**入力 YAML スペック（`config.yaml`）：**
```yaml
env: "prod"
app_name: "go"         # エラー 1: minlen=3 に違反
api_url: ""            # エラー 2: env==prod のとき required_if 制約に違反
timeout: "500ms"       # エラー 3: 厳格な時間リミット min=1s に違反
server:
  logging:
    level: "debug"     # (有効)
  allowed_ips: []      # エラー 4: コレクションの mincount=1 に違反
```
**アプリケーションランタイムが返すエラー文字列の集積：**
```text
field app_name: string length 2 is less than minlen 3
field api_url: is required when field env==prod
field timeout: value 500ms < min 1s
field server.allowed_ips: collection size 0 is less than mincount 1
```

---

## 例

`examples/` ディレクトリには、すべての検証ルールと機能を網羅する実行可能なプログラムが含まれています。リポジトリのルートから実行してください：

```bash
go run ./examples/01_choice
```

| 例 | デモ内容 |
|---------|--------------|
| `01_choice` | ホワイトリスト/ブラックリストの `choice` ルール、カンマを含むクォート値、有効と無効のプロファイル |
| `02_help` | `description` タグから `yaml.Help()` により CLI ヘルプを自動生成 |
| `03_min-max` | 数値と `time.Duration` の包含境界 `min`/`max`、有効と範囲外のプロファイル |
| `04_mincount-maxcount` | スライスとマップのコレクションサイズ制限、有効と容量違反のプロファイル |
| `05_required_if` | `==`/`!=` 演算子、`~=`/`^=` 正規表現マッチャー、および `endpoint` と `url` ルールによるクロスフィールド `required_if` |
| `06_with-include` | ロード時の `!include` ディレクティブ解決、ファイル欠落エラーを含む |
| `07_with_env_overwrite` | タグのデフォルト値を上書きする環境変数注入、ホワイトリスト違反を含む |
| `08_write-to-includes` | `!include` 構造を保持するアトミックな書き戻し（`DumpWithInclude`） |
| `10_test` | include パストラッカー（`FindIncludeFile`）とスカラー入力に対する `DumpWithInclude` の拒否パス |
| `11_not_empty-required` | `required` ルールとその旧来のエイリアス `not_empty`（後方互換のために保持） |
| `12_gt-lt` | int、float、`time.Duration` の厳格境界 `gt`/`lt` |
| `13_minlen-maxlen` | Unicode ルーン数で数える文字列長リミット |
| `14_format` | `format=ip`、`format=ipv4`、`format=ipv6`、`format=uuid`、`format=hex` |
| `15_regexp` | `regexp` ルール：通常のパターン、埋め込まれたカンマ、スライス要素 |
| `16_validation-errors` | 複数のルールにまたがる複数違反の集約レポート |
| `17_embed` | 共有ネスト型上のインスタンス単位のパスルール（`min_len.min=0` と `min_len.min=4`） |
| `18_required_if-operators` | `required_if` の正規演算子：`==`、`!=`、`~=`、`^=` |
| `19_priority` | `priority` タグによる YAML/default 値と環境変数の間のソース仲裁：両ソース存在、単一ソース、タグなしのデフォルト動作、ゼロ default の境界 |
| `20_required_if-logic` | `required_if` 内のブール式：`&&`、`\|\|`、括弧と優先順位（`&&` は `\|\|` より強く結合） |
