## build clash xcframework

1. Install and configure Golang, navigate to the project directory and initialize it:

```shell
go get -u -d -v golang.org/x/mobile/cmd/gomobile
go build -a -v golang.org/x/mobile/cmd/gomobile
go install -v golang.org/x/mobile/cmd/gomobile
gomobile init
```

2. build:

```shell
make ios
```