module github.com/a-digi/coco-iam

go 1.26.0

require (
	github.com/a-digi/coco-filer v0.1.0
	github.com/a-digi/coco-lift v0.0.1
	github.com/a-digi/coco-logger v0.1.0
	github.com/a-digi/coco-oauth v0.1.0
	github.com/a-digi/coco-observe v0.1.2
	github.com/a-digi/coco-orm v0.1.5
	github.com/a-digi/coco-queue v0.0.1
	github.com/a-digi/coco-server v0.1.9
	github.com/disintegration/imaging v1.6.2
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/mattn/go-sqlite3 v1.14.34
	github.com/parquet-go/parquet-go v0.28.0
	github.com/wneessen/go-mail v0.7.2
	golang.org/x/crypto v0.48.0
	golang.org/x/image v0.39.0
)

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/parquet-go/bitpack v1.0.0 // indirect
	github.com/parquet-go/jsonlite v1.0.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/twpayne/go-geom v1.6.1 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/a-digi/coco-orm => ../plugins/coco-orm

// replace github.com/a-digi/coco-server => ../plugins/coco-server

// replace github.com/a-digi/coco-lift => ../plugins/coco-lift

// replace github.com/a-digi/coco-observe => ../plugins/coco-observe
