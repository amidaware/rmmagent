### Tactical RMM Agent
https://github.com/amidaware/tacticalrmm

#### building the agent - linux
```
env CGO_ENABLED=0 GOOS=<GOOS> GOARCH=<GOARCH> go build -ldflags "-s -w -X 'main.Version=v2.0.4"
```

#### building the agent - windows
```
$env:CGO_ENABLED="0";$env:GOOS="windows";$env:GOARCH="amd64"; go build -ldflags "-s -w -X 'main.Version=v2.0.4"
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