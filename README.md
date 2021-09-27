# MYGAME

## HTTP-PORT:  8080

## LOCAL CONFIGURATION

### REQUIREMENTS
- go 1.16
- postgresql

Create **local-config.yaml** file in config directory:
```yaml
appconfig:
  ip: "0.0.0.0"
  grpc_port: "7001"
  http_port: "8080"
redis:
  db: 0
  host: "0.0.0.0"
  port: "6379"
  password: ""
```

### Build
```shell
go build -o fibonacci-service cmd/main.go
```
### Run
```shell
./fibonacci-service -config-path ./config/config.yaml
```

### OR

```shell
go run cmd/main.go -config-path ./config/config.yaml
```
