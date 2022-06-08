### Tactical RMM Agent
https://github.com/amidaware/tacticalrmm

#### building the agent
```
env CGO_ENABLED=0 GOOS=<GOOS> GOARCH=<GOARCH> go build -ldflags "-s -w"
```

#### building the mac agent
```
env GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w"
```