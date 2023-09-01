/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	goDebug "runtime/debug"
	"strconv"
	"strings"
	"time"

	ps "github.com/elastic/go-sysinfo"
	"github.com/go-ping/ping"
	"github.com/go-resty/resty/v2"
	"github.com/shirou/gopsutil/v3/process"
)

type PingResponse struct {
	Status string
	Output string
}

func DoPing(host string) (PingResponse, error) {
	var ret PingResponse
	pinger, err := ping.NewPinger(host)
	if err != nil {
		return ret, err
	}

	var buf bytes.Buffer
	pinger.OnRecv = func(pkt *ping.Packet) {
		fmt.Fprintf(&buf, "%d bytes from %s: icmp_seq=%d time=%v\n",
			pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt)
	}

	pinger.OnFinish = func(stats *ping.Statistics) {
		fmt.Fprintf(&buf, "\n--- %s ping statistics ---\n", stats.Addr)
		fmt.Fprintf(&buf, "%d packets transmitted, %d packets received, %v%% packet loss\n",
			stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
		fmt.Fprintf(&buf, "round-trip min/avg/max/stddev = %v/%v/%v/%v\n",
			stats.MinRtt, stats.AvgRtt, stats.MaxRtt, stats.StdDevRtt)
	}

	pinger.Count = 3
	pinger.Size = 548
	pinger.Interval = time.Second
	pinger.Timeout = 5 * time.Second
	pinger.SetPrivileged(true)

	err = pinger.Run()
	if err != nil {
		return ret, err
	}

	ret.Output = buf.String()

	stats := pinger.Statistics()

	if stats.PacketsRecv == stats.PacketsSent || stats.PacketLoss == 0 {
		ret.Status = "passing"
	} else {
		ret.Status = "failing"
	}

	return ret, nil
}

// PublicIP returns the agent's public ip
// Tries 3 times before giving up
func (a *Agent) PublicIP() string {
	a.Logger.Debugln("PublicIP start")
	client := resty.New()
	client.SetTimeout(4 * time.Second)
	if len(a.Proxy) > 0 {
		client.SetProxy(a.Proxy)
	}
	urls := []string{"https://icanhazip.tacticalrmm.io/", "https://icanhazip.com", "https://ifconfig.co/ip"}
	ip := "error"

	for _, url := range urls {
		r, err := client.R().Get(url)
		if err != nil {
			a.Logger.Debugln("PublicIP err", err)
			continue
		}
		ip = StripAll(r.String())
		if !IsValidIP(ip) {
			a.Logger.Debugln("PublicIP not valid", ip)
			continue
		}
		v4 := net.ParseIP(ip)
		if v4.To4() == nil {
			r1, err := client.R().Get("https://ifconfig.me/ip")
			if err != nil {
				return ip
			}
			ipv4 := StripAll(r1.String())
			if !IsValidIP(ipv4) {
				continue
			}
			a.Logger.Debugln("Forcing ipv4:", ipv4)
			return ipv4
		}
		a.Logger.Debugln("PublicIP return: ", ip)
		break
	}
	return ip
}

// GenerateAgentID creates and returns a unique agent id
func GenerateAgentID() string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, 40)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// ShowVersionInfo prints basic debugging info
func ShowVersionInfo(ver string) {
	fmt.Println("Tactical RMM Agent:", ver)
	fmt.Println("Arch:", runtime.GOARCH)
	if runtime.GOOS == "windows" {
		fmt.Println("Program Directory:", filepath.Join(os.Getenv("ProgramFiles"), progFilesName))
	}
	bi, ok := goDebug.ReadBuildInfo()
	if ok {
		fmt.Println(bi.String())
	}
}

// TotalRAM returns total RAM in GB
func (a *Agent) TotalRAM() float64 {
	host, err := ps.Host()
	if err != nil {
		return 8.0
	}
	mem, err := host.Memory()
	if err != nil {
		return 8.0
	}
	return math.Ceil(float64(mem.Total) / 1073741824.0)
}

// BootTime returns system boot time as a unix timestamp
func (a *Agent) BootTime() int64 {
	host, err := ps.Host()
	if err != nil {
		return 1000
	}
	info := host.Info()
	return info.BootTime.Unix()
}

// IsValidIP checks for a valid ipv4 or ipv6
func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// StripAll strips all whitespace and newline chars
func StripAll(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\n")
	s = strings.Trim(s, "\r")
	return s
}

// KillProc kills a process and its children
func KillProc(pid int32) error {
	p, err := process.NewProcess(pid)
	if err != nil {
		return err
	}

	children, err := p.Children()
	if err == nil {
		for _, child := range children {
			if err := child.Kill(); err != nil {
				continue
			}
		}
	}

	if err := p.Kill(); err != nil {
		return err
	}
	return nil
}

// DjangoStringResp removes double quotes from django rest api resp
func DjangoStringResp(resp string) string {
	return strings.Trim(resp, `"`)
}

func TestTCP(addr string) error {
	conn, err := net.Dial("tcp4", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}

// CleanString removes invalid utf-8 byte sequences
func CleanString(s string) string {
	r := strings.NewReplacer("\x00", "")
	s = r.Replace(s)
	return strings.ToValidUTF8(s, "")
}

// https://golangcode.com/unzip-files-in-go/
func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {

		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip. More Info: http://bit.ly/2MsjAWE
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("%s: illegal file path", fpath)
		}

		if f.FileInfo().IsDir() {
			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Make File
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		_, err = io.Copy(outFile, rc)

		// Close the file without defer to close before next iteration of loop
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

// https://yourbasic.org/golang/formatting-byte-size-to-human-readable-format/
func ByteCountSI(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

func randRange(min, max int) int {
	return rand.Intn(max-min) + min
}

func randomCheckDelay() {
	time.Sleep(time.Duration(randRange(300, 950)) * time.Millisecond)
}

func removeWinNewLines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func regRangeToInt(s string) int {
	split := strings.Split(s, ",")
	min, _ := strconv.Atoi(split[0])
	max, _ := strconv.Atoi(split[1])
	return randRange(min, max)
}

func getPowershellExe() string {
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil || powershell == "" {
		return filepath.Join(os.Getenv("WINDIR"), `System32\WindowsPowerShell\v1.0\powershell.exe`)
	}
	return powershell
}

func getCMDExe() string {
	cmdExe, err := exec.LookPath("cmd.exe")
	if err != nil || cmdExe == "" {
		return filepath.Join(os.Getenv("WINDIR"), `System32\cmd.exe`)
	}
	return cmdExe
}

// more accurate than os.Getwd()
func getCwd() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.Dir(self), nil
}

func createNixTmpFile() (*os.File, error) {
	var f *os.File
	cwd, err := getCwd()
	if err != nil {
		return f, err
	}

	f, err = os.CreateTemp(cwd, "trmm")
	if err != nil {
		return f, err
	}

	return f, nil
}
