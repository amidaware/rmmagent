### Tactical RMM Agent
https://github.com/amidaware/tacticalrmm

#### building the agent
```
env CGO_ENABLED=0 GOOS=<GOOS> GOARCH=<GOARCH> go build -ldflags "-s -w"
```

### tests
Navigate to agent directory
```
go test -vet=off
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