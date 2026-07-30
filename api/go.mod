module github.com/a-digi/coco-iam

go 1.26.0

require (
	github.com/a-digi/coco-filer v0.1.0
	github.com/a-digi/coco-lift v0.0.1
	github.com/a-digi/coco-logger v0.1.0
	github.com/a-digi/coco-oauth v0.1.0
	github.com/a-digi/coco-observe v0.1.2
	github.com/a-digi/coco-orm v1.0.6
	github.com/a-digi/coco-queue v0.0.1
	github.com/a-digi/coco-server v0.1.10
	github.com/disintegration/imaging v1.6.2
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/mattn/go-sqlite3 v1.14.34
	github.com/parquet-go/parquet-go v0.28.0
	github.com/pquerna/otp v1.5.0
	github.com/swaggo/http-swagger v1.3.4
	github.com/swaggo/swag v1.16.6
	github.com/wneessen/go-mail v0.7.2
	golang.org/x/crypto v0.49.0
	golang.org/x/image v0.39.0
)

require (
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/boombuler/barcode v1.0.1-0.20190219062509-6c824513bacc // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/spec v0.20.6 // indirect
	github.com/go-openapi/swag v0.19.15 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/parquet-go/bitpack v1.0.0 // indirect
	github.com/parquet-go/jsonlite v1.0.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/swaggo/files v0.0.0-20220610200504-28940afbdbfe // indirect
	github.com/twpayne/go-geom v1.6.1 // indirect
	golang.org/x/mod v0.34.0 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	golang.org/x/tools v0.43.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/a-digi/coco-server => ../plugins/coco-server

replace github.com/a-digi/coco-lift => ../plugins/coco-lift

replace github.com/a-digi/coco-observe => ../plugins/coco-observe
