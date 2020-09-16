package main

import (
	"fmt"
	"github.com/spf13/pflag"
	"os"
	"os/exec"
	"syscall"
)

var ( // flags
	chroot, chdir string
	loud          bool
)

var args []string

// Flag and argument parsing
func init() {
	pflag.StringVarP(&chroot, "chrt","" ,"", "Where to chroot to. Should contain a linux filesystem. Alpine is recommended.")
	pflag.StringVarP(&chdir, "chdr", "","/usr", "Initial chdir executed when running container.")
	pflag.BoolVar(&loud, "loud", false, "Suppresses not container output. Debugging purposes")
	pflag.Parse()
	args = pflag.Args()
	if chroot == "" {
		fatalf("chroot (--chrt flag) is required. got args: %v", args)
	}
	if len(args) < 2 {
		fatalf("too few arguments. got: %v", args)
	}
}

func main() {
	switch args[0] {
	case "run":
		run()
	case "child":
		child()
	default:
		panic("bad command")
	}
}

func run() {
	infof("run as [%d] : running %v", os.Getpid(), args[1:])
	lst := append([]string{"--chrt", chroot, "--chdr", chdir, "child"}, args[1:]...)
	cmd := exec.Command("/proc/self/exe", lst...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
	}
	must(cmd.Run())
}

// This child function runs a command in a containerized
// linux filesystem so it can't hurt you.
func child() {
	infof("child as [%d]: chrt: %s,  chdir:%s", os.Getpid(), chroot, chdir)
	infof("running %v", args[1:])
	must(syscall.Sethostname([]byte("container")))
	must(syscall.Chroot(chroot), "error in chroot:", chroot)
	syscall.Mkdir(chdir, 0600)

	// initial chdir is necessary so dir pointer is in chroot dir when proc mount is called
	must(syscall.Chdir("/"), "error in chdir /")
	must(syscall.Mount("proc", "proc", "proc", 0, ""), "error in proc mount")
	must(syscall.Chdir(chdir), "error in chdir:", chdir)
	cmd := exec.Command(args[1], args[2:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	must(cmd.Run())

	syscall.Unmount("/proc", 0)
}

func must(err error, s ...string) {
	if err != nil {
		errorf("%s : %v", err, s)
		os.Exit(1)
	}
}

func infof(format string, args ...interface{})  { logf("inf", format, args) }
//func printf(format string, args ...interface{}) { logf("prn", format, args) }
func errorf(format string, args ...interface{}) { logf("err", format, args) }
func fatalf(format string, args ...interface{}) { logf("fat", format, args); os.Exit(1) }
func logf(tag, format string, args []interface{}) {
	if loud {
		msg := fmt.Sprintf(format, args...)
		if args == nil {
			msg = fmt.Sprintf(format)
		}
		fmt.Println(fmt.Sprintf("[%s] %s", tag, msg))
	}
}
