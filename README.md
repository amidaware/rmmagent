### Tactical RMM Agent
https://github.com/amidaware/tacticalrmm

#### building the agent - linux
```
env CGO_ENABLED=0 GOOS=<GOOS> GOARCH=<GOARCH> go build -ldflags "-s -w -X 'main.version=v2.0.4'"
example: env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X 'main.version=v2.0.4'" -o build/output/rmmagent
```

#### building the agent - macos
```
env GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X 'main.version=v2.0.4'" -o build/output/rmmagent
```

#### building the agent - windows
```
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo
go generate
$env:CGO_ENABLED="0";$env:GOOS="windows";$env:GOARCH="amd64"; go build -ldflags "-s -w -X 'main.version=v2.0.4'" -o build/output/tacticalrmm.exe
```

### tests
Navigate to repo directory
```
go test ./... -vet=off
```

Add to settings.json
```
"gopls": {
    "build.buildFlags": [
      "-tags=DEBUG"
    ]
  },
  "go.testFlags": [
    "-vet=off"
  ],
  "go.testTags": "TEST",
```