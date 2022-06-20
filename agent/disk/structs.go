package disk

type Disk struct {
	Device  string `json:"device"`
	Fstype  string `json:"fstype"`
	Total   string `json:"total"`
	Used    string `json:"used"`
	Free    string `json:"free"`
	Percent int    `json:"percent"`
}