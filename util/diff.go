package util

import (
	"bytes"
	"fmt"
	"time"

	"syscall"

	"context"
	"os/exec"

	"github.com/segmentio/ksuid"
	"golang.org/x/sys/unix"
)

// Diff uses linux `diff` to generate a unified diff patch.package util
func Diff(data1, data2 []byte) ([]byte, error) {

	// Create the two memory-allocated files.
	fd1, err := createMemfile(data1)

	if err != nil {
		return nil, fmt.Errorf("unable to create memory-allocated file: %v", err)
	}

	defer syscall.Close(fd1)

	fd2, err := createMemfile(data2)

	if err != nil {
		return nil, fmt.Errorf("unable to create memory-allocated file: %v", err)
	}

	defer syscall.Close(fd2)

	//Run the following command to generate unified diff:
	// diff -au /proc/self/$fd1 /proc/self/$fd2
	// Timeout in 5 seconds.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"diff",
		"-au",
		fmt.Sprintf("/proc/self/fd/%d", fd1),
		fmt.Sprintf("/proc/self/fd/%d", fd2))

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	// Note that diff returns exit code 1 when the two files being compared are different,
	// but Go's exec.Command.Run interprets non-zero exit codes as an error.
	// Here we are untangling this case by failing on any non-zero exit code but 1.
	if err != nil {
		fail := true
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ProcessState != nil && exitError.ProcessState.ExitCode() == 1 {
				fail = false
			}
		}

		if fail {
			return nil, fmt.Errorf("unable to execute 'diff': %v : %v", err, stderr)
		}
	}

	return stdout.Bytes(), nil
}

// Creates an anonymous memory-allocated file populated with the given data and returns its file descriptor.
func createMemfile(contents []byte) (int, error) {
	var err error
	filename := ksuid.New().String()

	fd, err := unix.MemfdCreate(filename, 0)

	if err != nil {
		return 0, fmt.Errorf("MemfdCreate failed: %v", err)
	}

	err = unix.Ftruncate(fd, int64(len(contents)))

	if err != nil {
		return 0, fmt.Errorf("Ftruncate failed: %v", err)
	}

	data, err := unix.Mmap(fd, 0, len(contents), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)

	if err != nil {
		return 0, fmt.Errorf("Mmap failed: %v", err)
	}

	copy(data, contents)

	err = unix.Munmap(data)

	if err != nil {
		return 0, fmt.Errorf("Munmap failed: %v", err)
	}

	return fd, nil
}
