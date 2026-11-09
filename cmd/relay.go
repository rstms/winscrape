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
	"github.com/rstms/winscrape/assets"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Relay(filename, command string, args []string) error {
	debug := ViperGetBool("debug")
	verbose := ViperGetBool("verbose")

	cache, err := assets.NewAssets()
	if err != nil {
		return Fatal(err)
	}
	defer cache.Close()

	if debug {
		args = append([]string{"-d"}, args...)
	}
	if verbose {
		args = append([]string{"-v"}, args...)
	}

	execFile := filepath.Join(cache.BinDir, filename)
	cmd := exec.Command(execFile, append([]string{command}, args...)...)
	cmd.Env = os.Environ()
	for key, value := range cache.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = cache.BinDir
	if debug {
		log.Printf("cache: %s\n", FormatJSON(cache))
		log.Printf("cmd.Env[\n%s\n]\n", strings.Join(cmd.Env, "\n"))
		log.Printf("cmd.Dir: %s\n", cmd.Dir)
		log.Printf("cmd: %s\n", cmd)
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
