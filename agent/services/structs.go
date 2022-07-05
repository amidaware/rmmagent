package services

type Service struct {
	Name             string `json:"name"`
	Status           string `json:"status"`
	DisplayName      string `json:"display_name"`
	BinPath          string `json:"binpath"`
	Description      string `json:"description"`
	Username         string `json:"username"`
	PID              uint32 `json:"pid"`
	StartType        string `json:"start_type"`
	DelayedAutoStart bool   `json:"autodelay"`
}

type WinSvcResp struct {
	Success  bool   `json:"success"`
	ErrorMsg string `json:"errormsg"`
}
