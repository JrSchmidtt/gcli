//go:build windows
// +build windows

package run

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/vinirossado/gcli/internal/pkg/helper"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

var quit = make(chan os.Signal, 1)

type Run struct{}

var RunCmd = &cobra.Command{
	Use:     "run",
	Short:   "gcli run [main.go path]",
	Long:    "gcli run [main.go path]",
	Example: "gcli run source/cmd",
	Run: func(cmd *cobra.Command, args []string) {
		cmdArgs, programArgs := helper.SplitArgs(cmd, args)
		var dir string
		if len(cmdArgs) > 0 {
			dir = cmdArgs[0]
		}
		base, err := os.Getwd()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "\033[31mERROR: %s\033[m\n", err)
			return
		}
		if dir == "" {
			cmdPath, err := helper.FindMain(base)

			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "\033[31mERROR: %s\033[m\n", err)
				return
			}

			switch len(cmdPath) {
			case 0:
				_, _ = fmt.Fprintf(os.Stderr, "\033[31mERROR: %s\033[m\n", "The cmd directory cannot be found in the current working directory")
				return
			case 1:
				for _, v := range cmdPath {
					dir = v
				}
			default:
				var cmdPaths []string
				for k := range cmdPath {
					cmdPaths = append(cmdPaths, k)
				}
				prompt := &survey.Select{
					Message:  "Which directory do you want to run?",
					Options:  cmdPaths,
					PageSize: 10,
				}
				e := survey.AskOne(prompt, &dir)
				if e != nil || dir == "" {
					return
				}
				dir = cmdPath[dir]
			}
		}
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		fmt.Printf("\033[35mGcli run %s.\033[0m\n", dir)
		watch(dir, programArgs)
	},
}

func watch(dir string, programArgs []string) {
	watchPath := "./"
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("Error creating watcher", err)
		return
	}
	defer func(watcher *fsnotify.Watcher) {
		err := watcher.Close()
		if err != nil {

		}
	}(watcher)

	err = filepath.Walk(watchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			ext := filepath.Ext(info.Name())
			if ext == ".go" || ext == ".yml" || ext == ".yaml" || ext == ".html" {
				err = watcher.Add(path)
				if err != nil {
					fmt.Println("Error:", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	cmd := start(dir, programArgs)

	for {
		select {
		case <-quit:
			err = killProcess(cmd)

			if err != nil {
				fmt.Printf("\033[31mserver exiting...\033[0m\n")
				return
			}
			fmt.Printf("\033[31mserver exiting...\033[0m\n")
			os.Exit(0)

		case event := <-watcher.Events:
			if event.Op&fsnotify.Create == fsnotify.Create ||
				event.Op&fsnotify.Write == fsnotify.Write ||
				event.Op&fsnotify.Remove == fsnotify.Remove {
				fmt.Printf("\033[36mfile modified: %s\033[0m\n]", event.Name)
				_ = killProcess(cmd)

				cmd = start(dir, programArgs)
			}
		case err := <-watcher.Errors:
			fmt.Println("Error:", err)
		}
	}
}

func killProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	taskkill := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	err := taskkill.Run()
	if err != nil {
		return err
	}
	return nil
}

func start(dir string, programArgs []string) *exec.Cmd {
	cmd := exec.Command("go", append([]string{"run", dir}, programArgs...)...)
	// Set a new process group to kill all child processes when the program exists

	fmt.Printf("Starting... %s\n", cmd)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		log.Fatalf("\033[33;1mcmd run failed\u001B[0m")
	}
	time.Sleep(time.Second)
	fmt.Printf("\033[33;1mrunning...\033[0m\n]")
	return cmd
}
