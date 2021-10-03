# MYGAME

## HTTP-PORT:  8080

## LOCAL CONFIGURATION

### REQUIREMENTS
- go 1.16
- postgresql

Create **local-config.yaml** file in config directory:
```yaml
app:
  port: 8080

db:
  host:     "0.0.0.0"
  port:     "5432"
  user:     "user"
  password: "password"
  db_name:  "mygame"
  ssl_mode: "disable"

jwt:
  secret_key:      "1234"
  expiration_time: "24h"
```

### Build
```shell
go build -o fibonacci-service cmd/main.go
```
### Run
```shell
./fibonacci-service
```

### OR

```shell
go run cmd/main.go
```
