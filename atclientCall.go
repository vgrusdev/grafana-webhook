package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
	"log/slog"
	"strconv"
)

type atClient_t struct {
	javaPath string
	javaParam []string
	jarPath string
	botServer string
	port string
	timeout time.Duration
}
// JavaProcess represents a running Java process
type JavaProcess struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	timeout time.Duration
}

// NewJavaProcess creates and starts a new Java process
func (a *App) NewJavaProcess(args []string) (*JavaProcess, error) {

	atClient := a.atClient
	className := ""				// Depriciated parameter of the function, restore it if needed in the future.

	// Prepare Java command arguments
	javaAgrs := atClient.javaParam
	if className == "" {
		javaArgs = append(javaArgs, "-jar", atClient.jarPath)
	} else {
		javaArgs = append(javaArgs, "-cp", atClient.jarPath, className)
	}
	javaSrgs = append(javaArgs, "\"" + atClient.botServer + "\"", "\"" + atClient.port + "\"")
	javaArgs = append(javaArgs, args...)

	// Create the command
	cmd := exec.Command(atClient.javaPath, javaArgs...)

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("error creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("error creating stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("error starting Java process: %w", err)
	}

	return &JavaProcess{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		timeout: atClient.timeout,
	}, nil
}

// Execute sends input to Java and returns the output
func (jp *JavaProcess) Execute(input string) (string, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), jp.timeout)
	defer cancel()

	// Channel to collect results
	resultChan := make(chan struct {
		output string
		err    error
	})

	// Goroutine to handle the execution
	go func() {
		// Send input to Java process
		if input != "" {
			if _, err := io.WriteString(jp.stdin, input+"\n"); err != nil {
				resultChan <- struct {
					output string
					err    error
				}{"", fmt.Errorf("error writing to stdin: %w", err)}
				return
			}
		}

		// Read output
		var outputBuf bytes.Buffer
		if _, err := io.Copy(&outputBuf, jp.stdout); err != nil {
			resultChan <- struct {
				output string
				err    error
			}{"", fmt.Errorf("error reading stdout: %w", err)}
			return
		}

		// Read error output
		var errorBuf bytes.Buffer
		if _, err := io.Copy(&errorBuf, jp.stderr); err != nil {
			resultChan <- struct {
				output string
				err    error
			}{"", fmt.Errorf("error reading stderr: %w", err)}
			return
		}

		if errorBuf.Len() > 0 {
			resultChan <- struct {
				output string
				err    error
			}{"", fmt.Errorf("Java error: %s", errorBuf.String())}
			return
		}

		resultChan <- struct {
			output string
			err    error
		}{outputBuf.String(), nil}
	}()

	// Wait for results or timeout
	select {
	case <-ctx.Done():
		jp.terminate()
		return "", errors.New("Java execution timed out")
	case result := <-resultChan:
		return result.output, result.err
	}
}

// Close terminates the Java process
func (jp *JavaProcess) Close() error {
	jp.stdin.Close()
	jp.stdout.Close()
	jp.stderr.Close()
	return jp.cmd.Wait()
}

// terminate kills the process if it's still running
func (jp *JavaProcess) terminate() {
	jp.stdin.Close()
	jp.stdout.Close()
	jp.stderr.Close()
	jp.cmd.Process.Kill()
}


func (a *App) atClientTelegram(chatID int64, msg string, fileName string) (error) {

	var err error

	// func (a *App) NewJavaProcess(args []string) (*JavaProcess, error), where args:
	//	<ChatID>  [<MessageId: <MID>>] [<ParseMode: <PM>>] <Body> [<FIle>]

	javaArgs := []string { "\"" + strconv.FormatInt(chatID, 10) + "\"", "\"" + msg + "\"" }
	if len(filename) > 0
	javaArgs = append(javaArgs, "\"" + filename + "\"")

	javaProcess, err := a.NewJavaProcess(javaArgs)
	if err != nil {
		fmt.Printf("Failed to start Java process: %v\n", err)
		return
	}
	defer javaProcess.Close()

	// Execute with input
	output, err := javaProcess.Execute("")

	//if err != nil {
	//	fmt.Printf("Error executing Java: %v\n", err)
	//	return
	//}

	slog.Info("Java output:\n%s\n", putput)

	return err

}
