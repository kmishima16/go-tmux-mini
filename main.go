package main

import (
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/gdamore/tcell/v2"
	"golang.org/x/term"
)

// イベントタイプの定義
type EventType int

const (
	KeyInput EventType = iota
	PTYOutput
	WindowResize
)

// アプリケーションイベント
type AppEvent struct {
	Type   EventType
	PaneID int
	Data   interface{}
}

// グローバルなイベントチャネル
var eventChan = make(chan AppEvent, 100)

type Pane struct {
	id       int
	x, y     int
	w, h     int
	ptmx     *os.File
	cmd      *exec.Cmd
	buffer   []string
	isActive bool
	mu       sync.Mutex
}

func NewPane(id, x, y, w, h int) (*Pane, error) {
	cmd := exec.Command("bash", "--norc", "-i")  // .bashrcを読み込まず、対話モードで起動
	cmd.Env = append(os.Environ(), 
		"TERM=dumb",  // シンプルなターミナルタイプ
		"PS1=~ $ ",   // シンプルなプロンプト
	)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	
	// PTYのウィンドウサイズを設定（枠線分を考慮）
	rows := uint16(h - 2)
	cols := uint16(w - 2)
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	if err := pty.Setsize(ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	}); err != nil {
		log.Printf("Failed to set PTY size: %v", err)
	}

	pane := &Pane{
		id:       id,
		x:        x,
		y:        y,
		w:        w,
		h:        h,
		ptmx:     ptmx,
		cmd:      cmd,
		buffer:   make([]string, 0),
		isActive: false,
	}

	go pane.readOutput()
	return pane, nil
}

func (p *Pane) readOutput() {
	buf := make([]byte, 4096) // バッファサイズを拡大
	for {
		n, err := p.ptmx.Read(buf)
		if err != nil {
			log.Printf("PTY read error for pane %d: %v", p.id, err)
			return
		}
		
		p.mu.Lock()
		output := string(buf[:n])
		p.buffer = append(p.buffer, output)
		if len(p.buffer) > 1000 {
			p.buffer = p.buffer[100:]
		}
		p.mu.Unlock()
		
		// イベントを送信
		eventChan <- AppEvent{
			Type:   PTYOutput,
			PaneID: p.id,
			Data:   output,
		}
	}
}

func (p *Pane) Write(data []byte) {
	p.ptmx.Write(data)
}

func (p *Pane) Close() {
	p.ptmx.Close()
	if p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
}

func (p *Pane) Draw(screen tcell.Screen) {
	log.Printf("Drawing pane %d at (%d,%d) size %dx%d", p.id, p.x, p.y, p.w, p.h)
	
	// 境界チェック
	screenW, screenH := screen.Size()
	if p.x >= screenW || p.y >= screenH || p.w <= 0 || p.h <= 0 {
		log.Printf("Pane %d is out of bounds or invalid size", p.id)
		return
	}
	
	// スタイル設定
	borderStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
	if p.isActive {
		borderStyle = tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorBlack)
	}

	// 簡単な枠線を描画（ASCII文字を使用）
	maxX := p.x + p.w - 1
	maxY := p.y + p.h - 1
	
	// 境界を再チェック
	if maxX >= screenW {
		maxX = screenW - 1
	}
	if maxY >= screenH {
		maxY = screenH - 1
	}
	
	// 水平線
	for x := p.x; x <= maxX; x++ {
		if p.y < screenH {
			screen.SetContent(x, p.y, '-', nil, borderStyle)
		}
		if maxY < screenH {
			screen.SetContent(x, maxY, '-', nil, borderStyle)
		}
	}
	
	// 垂直線
	for y := p.y; y <= maxY; y++ {
		if p.x < screenW {
			screen.SetContent(p.x, y, '|', nil, borderStyle)
		}
		if maxX < screenW {
			screen.SetContent(maxX, y, '|', nil, borderStyle)
		}
	}
	
	// 角
	if p.x < screenW && p.y < screenH {
		screen.SetContent(p.x, p.y, '+', nil, borderStyle)
	}
	if maxX < screenW && p.y < screenH {
		screen.SetContent(maxX, p.y, '+', nil, borderStyle)
	}
	if p.x < screenW && maxY < screenH {
		screen.SetContent(p.x, maxY, '+', nil, borderStyle)
	}
	if maxX < screenW && maxY < screenH {
		screen.SetContent(maxX, maxY, '+', nil, borderStyle)
	}

	// 簡単なテキスト描画（バッファから最新の数行）
	p.mu.Lock()
	bufferText := ""
	for _, line := range p.buffer {
		bufferText += line
	}
	p.mu.Unlock()

	// 内容の描画エリア
	contentX := p.x + 1
	contentY := p.y + 1
	contentW := p.w - 2
	contentH := p.h - 2
	
	if contentW > 0 && contentH > 0 {
		// 簡単な文字描画
		lines := []rune(bufferText)
		row, col := contentY, contentX
		for _, r := range lines {
			if row >= contentY+contentH {
				break
			}
			if r == '\n' || r == '\r' {
				row++
				col = contentX
			} else if col < contentX+contentW && row < contentY+contentH {
				if col < screenW && row < screenH {
					screen.SetContent(col, row, r, nil, tcell.StyleDefault)
				}
				col++
				if col >= contentX+contentW {
					row++
					col = contentX
				}
			}
		}
	}
	
	log.Printf("Pane %d drawn successfully", p.id)
}

func main() {
	// デバッグ用ログファイル
	logFile, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	
	screen, err := tcell.NewScreen()
	if err != nil {
		log.Fatalf("Error creating screen: %v", err)
	}
	if err := screen.Init(); err != nil {
		log.Fatalf("Error initializing screen: %v", err)
	}
	defer screen.Fini()
	
	// 強制的に画面をクリアして、基本設定を確認
	screen.Clear()
	screen.SetStyle(tcell.StyleDefault)
	log.Printf("Screen initialized successfully")

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("Error setting raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	w, h := screen.Size()
	
	var panes []*Pane
	activePaneIndex := 0
	prefixMode := false
	paneIDCounter := 0

	initialPane, err := NewPane(paneIDCounter, 0, 0, w, h)
	if err != nil {
		log.Fatalf("Error creating initial pane: %v", err)
	}
	initialPane.isActive = true
	panes = append(panes, initialPane)
	paneIDCounter++

	defer func() {
		for _, pane := range panes {
			pane.Close()
		}
	}()

	draw := func() {
		log.Printf("Draw function called")
		screen.Clear()
		
		// デバッグ: 画面サイズとペイン情報をログ出力
		w, h := screen.Size()
		log.Printf("Screen size: %dx%d, Panes: %d", w, h, len(panes))
		for i, pane := range panes {
			log.Printf("Pane %d: x=%d, y=%d, w=%d, h=%d, active=%v", i, pane.x, pane.y, pane.w, pane.h, pane.isActive)
		}
		
		log.Printf("Drawing panes")
		for _, pane := range panes {
			pane.Draw(screen)
		}
		
		log.Printf("Calling screen.Show()")
		screen.Show()
		log.Printf("Draw completed")
	}

	draw()

	// tcellイベントをAppEventに変換するgoroutine
	go func() {
		for {
			ev := screen.PollEvent()
			if ev != nil {
				switch ev := ev.(type) {
				case *tcell.EventKey:
					eventChan <- AppEvent{
						Type: KeyInput,
						Data: ev,
					}
				case *tcell.EventResize:
					eventChan <- AppEvent{
						Type: WindowResize,
						Data: ev,
					}
				}
			}
		}
	}()

	// メインイベントループ
	for {
		select {
		case event := <-eventChan:
			switch event.Type {
			case KeyInput:
				// キー入力処理
				ev := event.Data.(*tcell.EventKey)
				log.Printf("Key event received: Key=%v, Rune=%c, Modifiers=%v", 
					ev.Key(), ev.Rune(), ev.Modifiers())
				
				if ev.Key() == tcell.KeyCtrlC {
					return
				}

				if prefixMode {
					prefixMode = false
					switch ev.Key() {
					case tcell.KeyRune:
						switch ev.Rune() {
						case '%':
							activePane := panes[activePaneIndex]
							if activePane.w > 4 {
								newW := activePane.w / 2
								newPane, err := NewPane(paneIDCounter, activePane.x+newW, activePane.y, activePane.w-newW, activePane.h)
								if err == nil {
									activePane.w = newW
									panes = append(panes, newPane)
									paneIDCounter++
									draw()
								}
							}
						case '"':
							activePane := panes[activePaneIndex]
							if activePane.h > 4 {
								newH := activePane.h / 2
								newPane, err := NewPane(paneIDCounter, activePane.x, activePane.y+newH, activePane.w, activePane.h-newH)
								if err == nil {
									activePane.h = newH
									panes = append(panes, newPane)
									paneIDCounter++
									draw()
								}
							}
						}
					case tcell.KeyLeft:
						if activePaneIndex > 0 {
							panes[activePaneIndex].isActive = false
							activePaneIndex--
							panes[activePaneIndex].isActive = true
							draw()
						}
					case tcell.KeyRight:
						if activePaneIndex < len(panes)-1 {
							panes[activePaneIndex].isActive = false
							activePaneIndex++
							panes[activePaneIndex].isActive = true
							draw()
						}
					case tcell.KeyUp:
						if activePaneIndex > 0 {
							panes[activePaneIndex].isActive = false
							activePaneIndex = (activePaneIndex - 1 + len(panes)) % len(panes)
							panes[activePaneIndex].isActive = true
							draw()
						}
					case tcell.KeyDown:
						if len(panes) > 1 {
							panes[activePaneIndex].isActive = false
							activePaneIndex = (activePaneIndex + 1) % len(panes)
							panes[activePaneIndex].isActive = true
							draw()
						}
					}
				} else {
					if ev.Key() == tcell.KeyCtrlB {
						prefixMode = true
					} else {
						activePane := panes[activePaneIndex]
						if ev.Key() == tcell.KeyRune {
							activePane.Write([]byte(string(ev.Rune())))
						} else {
							switch ev.Key() {
							case tcell.KeyEnter:
								activePane.Write([]byte("\r"))
							case tcell.KeyBackspace, tcell.KeyBackspace2:
								activePane.Write([]byte("\b"))
							case tcell.KeyTab:
								activePane.Write([]byte("\t"))
							case tcell.KeyEscape:
								activePane.Write([]byte("\x1b"))
							}
						}
					}
				}
			
			case PTYOutput:
				// PTY出力イベント
				log.Printf("PTY output from pane %d: %d bytes", event.PaneID, len(event.Data.(string)))
				draw()
			
			case WindowResize:
				// リサイズイベント
				w, h = screen.Size()
				if len(panes) == 1 {
					panes[0].x, panes[0].y = 0, 0
					panes[0].w, panes[0].h = w, h
				}
				log.Printf("Window resized to %dx%d", w, h)
				draw()
			}
		}
	}
}