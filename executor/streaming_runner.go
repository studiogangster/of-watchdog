package executor

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/fluent/fluent-logger-golang/fluent"
)

// FunctionRunner runs a function
type FunctionRunner interface {
	Run(f FunctionRequest) error
}

// FunctionRequest stores request for function execution
type FunctionRequest struct {
	Process     string
	ProcessArgs []string
	Environment []string

	InputReader   io.ReadCloser
	OutputWriter  io.Writer
	ContentLength *int64
	TractID       string
}

// ForkFunctionRunner forks a process for each invocation
type ForkFunctionRunner struct {
	ExecTimeout time.Duration
}

var logger, _ = fluent.New(fluent.Config{
	FluentPort:   24224,
	FluentHost:   "localhost",
	TagPrefix:    "watchdog",
	MaxRetryWait: 4,
	Async:        true,
	MaxRetry:     0,
})

// Run run a fork for each invocation
func (f *ForkFunctionRunner) Run(req FunctionRequest) error {
	log.Printf("Running %s", req.Process)
	start := time.Now()
	cmd := exec.Command(req.Process, req.ProcessArgs...)
	cmd.Env = req.Environment

	var timer *time.Timer
	if f.ExecTimeout > time.Millisecond*0 {
		timer = time.NewTimer(f.ExecTimeout)

		go func() {
			<-timer.C

			log.Printf("Function was killed by ExecTimeout: %s\n", f.ExecTimeout.String())
			killErr := cmd.Process.Kill()
			if killErr != nil {
				fmt.Println("Error killing function due to ExecTimeout", killErr)
			}
		}()
	}

	if timer != nil {
		defer timer.Stop()
	}

	if req.InputReader != nil {
		defer req.InputReader.Close()
		cmd.Stdin = req.InputReader
	}
	// cmd.Stdout = req.OutputWriter

	// Prints stderr to console and is picked up by container logging driver.
	errPipe, _ := cmd.StderrPipe()
	stdoutPipe, _ := cmd.StdoutPipe()
	// log.Printf("TractId", req.TractID)

	var wg sync.WaitGroup
	bindFluentLoggingPipe(logger, "stderr", req.TractID, errPipe, &wg)
	bindFluentLoggingPipe(logger, "stdout", req.TractID, stdoutPipe, &wg)

	startErr := cmd.Start()
	wg.Wait()

	if startErr != nil {
		log.Println("Starting error", startErr)

		logger.Post(req.TractID, map[string]string{
			"pipe":    "stdend",
			"message": startErr.Error(),
		})
		return startErr
	}

	logger.Post(req.TractID, map[string]string{
		"pipe":    "stdend",
		"message": "Process completed successfully",
	})

	waitErr := cmd.Wait()
	done := time.Since(start)
	log.Printf("Took %f secs", done.Seconds())
	if timer != nil {
		timer.Stop()
	}

	req.InputReader.Close()

	req.OutputWriter.Write([]byte("Trace-ID: " + req.TractID))

	if waitErr != nil {
		return waitErr
	}

	return nil
}
