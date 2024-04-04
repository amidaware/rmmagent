package support

import (
	"encoding/json"
	"fmt"
	"golang.org/x/sys/windows"
	"syscall"
)

var hPipe windows.Handle
var currentTitle string
var currentConfig []MenuItemConfig

func startPipe() {
	const pipeName = `\\.\pipe\TRMM`
	const bufferSize = 512

	for {
		// Create a new named pipe instance
		hPipe, err := windows.CreateNamedPipe(
			syscall.StringToUTF16Ptr(pipeName),
			windows.PIPE_ACCESS_DUPLEX,
			windows.PIPE_TYPE_MESSAGE|windows.PIPE_READMODE_MESSAGE|windows.PIPE_WAIT,
			1, // Single instance
			bufferSize,
			bufferSize,
			0,
			nil,
		)
		if err != nil {
			fmt.Printf("Failed to create named pipe: %v\n", err)
			break
		}

		fmt.Println("Named pipe created, waiting for client connections...")

		// Wait for a client to connect
		err = windows.ConnectNamedPipe(hPipe, nil)
		if err != nil {
			fmt.Printf("Failed to connect to named pipe: %v\n", err)
			windows.CloseHandle(hPipe)
			continue // Attempt to create a new pipe in the next iteration
		}

		fmt.Println("Client connected.")

		handleClient(hPipe, bufferSize)

		// Close the handle before creating a new named pipe instance
		windows.CloseHandle(hPipe)
	}
}

func handleClient(hPipe windows.Handle, bufferSize uint32) {
    var totalData []byte

    for {
        buf := make([]byte, bufferSize)
        var read uint32
        err := windows.ReadFile(hPipe, buf, &read, nil)
        if err != nil {
            if err == windows.ERROR_MORE_DATA {
                // Append the read data to totalData and continue reading
                totalData = append(totalData, buf[:read]...)
                continue
            } else {
                fmt.Printf("Failed to read from named pipe: %v\n", err)
                break
            }
        }
        // Append the last chunk of data and break the loop
        totalData = append(totalData, buf[:read]...)
        break
    }

    // Process the received message
    processMessage(totalData)
}

func processMessage(data []byte) {
    var msg struct {
        Name   string          `json:"name"`
        Config []MenuItemConfig `json:"config"`
    }
    if err := json.Unmarshal(data, &msg); err != nil {
        fmt.Printf("Failed to unmarshal message: %v\n", err)
    } else {
        currentTitle = msg.Name
        currentConfig = msg.Config
        fmt.Printf("Received message: %+v\n", msg)
	updateSystray()

        // Check if the message is a shutdown signal
        if msg.Name == "systray-shutdown" {
            fmt.Println("Shutdown signal received. Initiating shutdown...")
            close(shutdownChan)
        }
    }
}

func closePipe() {
    windows.CloseHandle(hPipe)
}
