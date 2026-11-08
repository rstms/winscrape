/*
Copyright © 2026 Matt Krueger <mkrueger@rstms.net>
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

 1. Redistributions of source code must retain the above copyright notice,
    this list of conditions and the following disclaimer.

 2. Redistributions in binary form must reproduce the above copyright notice,
    this list of conditions and the following disclaimer in the documentation
    and/or other materials provided with the distribution.

 3. Neither the name of the copyright holder nor the names of its contributors
    may be used to endorse or promote products derived from this software
    without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.
*/
package cmd

import (
	"errors"
	"fmt"
	"github.com/rstms/winscrape/tesseract"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Relay(filename, command string, args []string) error {
	debug := ViperGetBool("debug")
	verbose := ViperGetBool("verbose")

	tesseract, err := tesseract.NewTesseract()
	if err != nil {
		return Fatal(err)
	}
	if ViperGetBool("retain_assets") {
		defer fmt.Fprintf(os.Stderr, "Retained files: %s\n", filepath.Join(tesseract.TempDir, "assets"))
	} else {
		defer tesseract.Close()
	}

	if debug {
		args = append([]string{"-d"}, args...)
	}
	if verbose {
		args = append([]string{"-v"}, args...)
	}

	execFile := filepath.Join(tesseract.BinDir, filename)
	cmd := exec.Command(execFile, append([]string{command}, args...)...)
	/*
		cmd.Env = []string{}
		for _, line := range os.Environ() {
			name, value, _ := strings.Cut(line, "=")
			switch strings.ToUpper(name) {
			case "PATH":
			case "TESSDATA_PREFIX":
			default:
				cmd.Env = append(cmd.Env, name+"="+value)
			}
		}
		path, err := AddPath(binDir)
		if err != nil {
			return Fatal(err)
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", path))
		cmd.Env = append(cmd.Env, fmt.Sprintf("TESSDATA_PREFIX=%s", dataDir))
		cmd.Dir = tesseract.BinDir
	*/
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if debug {
		//log.Printf("binaryFile: %s\n", binaryFile)
		//log.Printf("binDir: %s\n", binDir)
		//log.Printf("dataDir: %s\n", dataDir)
		//log.Printf("tempDir: %s\n", tempDir)
		log.Printf("PATH=%s\n", os.Getenv("PATH"))
		log.Printf("cmd: %s\n", cmd)
		log.Printf("cmd.Env[\n%s\n]\n", strings.Join(cmd.Env, "\n"))
		//log.Printf("cmd.Dir: %s\n", cmd.Dir)
	}
	err = cmd.Run()
	if err != nil {
		if errors.Is(err, &exec.ExitError{}) {
			exitErr, _ := errors.AsType[*exec.ExitError](err)
			ExitCode = exitErr.ProcessState.ExitCode()
			if debug {
				fmt.Fprintf(os.Stderr, "%s\n", exitErr.ProcessState.String())
			}
			return Fatal(exitErr)
		}
		return Fatal(err)
	}
	ExitCode = cmd.ProcessState.ExitCode()
	return nil
}
