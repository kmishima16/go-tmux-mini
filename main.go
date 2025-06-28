package main

import (
	"log"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/gdamore/tcell/v2"
	"golang.org/x/term"
)

func main() {
	screen, err := tcell.NewScreen()
	if err != nil {
		log.Fatalf("Error creating screen: %v", err)
	}
	if err := screen.Init(); err != nil {
		log.Fatalf("Error initializing screen: %v", err)
	}
	defer screen.Fini()

	cmd := exec.Command("bash")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Fatalf("Error starting PTY: %v", err)
	}
	defer ptmx.Close()

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("Error setting raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				return
			}
			log.Printf("PTY output: %s", string(buf[:n]))
		}
	}()

	for {
		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyCtrlC {
				return
			}
			if ev.Key() == tcell.KeyRune {
				ptmx.Write([]byte(string(ev.Rune())))
			} else {
				switch ev.Key() {
				case tcell.KeyEnter:
					ptmx.Write([]byte("\r"))
				case tcell.KeyBackspace, tcell.KeyBackspace2:
					ptmx.Write([]byte("\b"))
				case tcell.KeyTab:
					ptmx.Write([]byte("\t"))
				}
			}
		case *tcell.EventResize:
			screen.Sync()
		}
	}
}