package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/eru1a/shogi-go"
	"github.com/eru1a/shogi-go/engine"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

type selectFrom struct {
	square shogi.Square
	piece  shogi.Piece
}

func newFrom() selectFrom {
	return selectFrom{square: shogi.NullSquare, piece: shogi.NO_PIECE}
}

func (f selectFrom) IsNull() bool {
	return f.square.IsNull() && f.piece == shogi.NO_PIECE
}

func (f *selectFrom) Cancel() {
	f.square = shogi.NullSquare
	f.piece = shogi.NO_PIECE
}

type PositionView struct {
	*tview.Grid
	App           *tview.Application
	View          *View
	Pages         *tview.Pages
	Game          *shogi.GameTree
	BoardView     *tview.Table
	BlackHandView *tview.Table
	WhiteHandView *tview.Table
	From          selectFrom
}

func NewPositionView(app *tview.Application, view *View, pages *tview.Pages, game *shogi.GameTree) *PositionView {
	boardView := tview.NewTable().SetBorders(false).SetSelectable(true, true)
	boardView.SetBorder(true).SetTitle("盤面")

	blackHandView := tview.NewTable().SetBorders(false)
	blackHandView.SetBorder(true).SetTitle("先手の持ち駒")

	whiteHandView := tview.NewTable().SetBorders(false)
	whiteHandView.SetBorder(true).SetTitle("後手の持ち駒")

	grid := tview.NewGrid().
		SetRows(3, 11, 3).
		AddItem(whiteHandView, 0, 0, 1, 1, 0, 0, false).
		AddItem(boardView, 1, 0, 1, 1, 0, 0, true).
		AddItem(blackHandView, 2, 0, 1, 1, 0, 0, false)

	p := &PositionView{
		Grid:          grid,
		View:          view,
		App:           app,
		Pages:         pages,
		Game:          game,
		BoardView:     boardView,
		BlackHandView: blackHandView,
		WhiteHandView: whiteHandView,
		From:          newFrom(),
	}

	p.setup()
	return p
}

func (p *PositionView) UpdateView() {
	// board
	for rank := 0; rank < 9; rank++ {
		for file := 0; file < 9; file++ {
			sq, _ := shogi.NewSquare(file, rank)
			piece := p.Game.Current.Position.Get(sq)
			kif := piece.PieceType().KIF()
			if piece.Color() == shogi.White {
				kif = "v" + kif
			} else {
				kif = " " + kif
			}
			cell := tview.NewTableCell(kif)
			if !p.From.IsNull() && p.From.square.File() == file && p.From.square.Rank() == rank {
				cell.SetBackgroundColor(tcell.ColorGreen)
			}
			p.BoardView.SetCell(rank, file, cell)
		}
	}

	// black hand
	for pt := shogi.FU; pt <= shogi.HI; pt++ {
		n, _ := p.Game.Current.Position.HandGet(pt, shogi.Black)
		cell := tview.NewTableCell(fmt.Sprintf("%s%d", pt.KIF(), n))
		if !p.From.IsNull() && p.From.piece.Color() == shogi.Black && p.From.piece.PieceType() == pt {
			cell.SetBackgroundColor(tcell.ColorGreen)
		}
		p.BlackHandView.SetCell(0, int(pt)-1, cell)
	}

	// white hand
	for pt := shogi.FU; pt <= shogi.HI; pt++ {
		n, _ := p.Game.Current.Position.HandGet(pt, shogi.White)
		cell := tview.NewTableCell(fmt.Sprintf("%s%d", pt.KIF(), n))
		if !p.From.IsNull() && p.From.piece.Color() == shogi.White && p.From.piece.PieceType() == pt {
			cell.SetBackgroundColor(tcell.ColorGreen)
		}
		p.WhiteHandView.SetCell(0, int(pt)-1, cell)
	}
}

func (p *PositionView) setup() {
	p.BoardView.SetSelectedFunc(func(rank, file int) {
		sq, _ := shogi.NewSquare(file, rank)

		if p.From.IsNull() {
			p.From.square = sq
			p.UpdateView()
			return
		}

		switch {
		case !p.From.square.IsNull():
			piece := p.Game.Current.Position.Get(p.From.square)
			if shogi.NeedForcePromotion(piece, sq.Rank()) {
				m := shogi.NewNormalMove(p.From.square, sq, true)
				p.View.Move(m)
			} else if shogi.CanPromote(piece, p.From.square, sq) {
				p.promotionModal(sq)
			} else {
				m := shogi.NewNormalMove(p.From.square, sq, false)
				p.View.Move(m)
			}
		case p.From.piece != shogi.NO_PIECE && p.From.piece.Color() == p.Game.Current.Position.Turn:
			m := shogi.NewDropMove(p.From.piece.PieceType(), sq)
			p.View.Move(m)
		default:
			p.From.Cancel()
			p.UpdateView()
		}
	})
	p.BoardView.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		rank, _ := p.BoardView.GetSelection()
		switch {
		case e.Key() == tcell.KeyDown, e.Rune() == 'j':
			if rank == 8 {
				p.BoardView.SetSelectable(false, false)
				p.BlackHandView.SetSelectable(true, true)
				p.App.SetFocus(p.BlackHandView)
				return nil
			}
		case e.Key() == tcell.KeyUp, e.Rune() == 'k':
			if rank == 0 {
				p.BoardView.SetSelectable(false, false)
				p.WhiteHandView.SetSelectable(true, true)
				p.App.SetFocus(p.WhiteHandView)
				return nil
			}
		}
		return e
	})

	p.BlackHandView.SetSelectedFunc(func(rank, file int) {
		if !p.From.IsNull() {
			p.From.Cancel()
			p.UpdateView()
			return
		}
		pt := shogi.PieceType(file + 1)
		p.From.piece = shogi.NewPiece(pt, shogi.Black)
		p.UpdateView()
	})
	p.BlackHandView.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		switch {
		case e.Key() == tcell.KeyUp, e.Rune() == 'k':
			p.BoardView.SetSelectable(true, true)
			p.BlackHandView.SetSelectable(false, false)
			p.App.SetFocus(p.BoardView)
			return nil
		}
		return e
	})

	p.WhiteHandView.SetSelectedFunc(func(rank, file int) {
		if !p.From.IsNull() {
			p.From.Cancel()
			p.UpdateView()
			return
		}
		pt := shogi.PieceType(file + 1)
		p.From.piece = shogi.NewPiece(pt, shogi.White)
		p.UpdateView()
	})
	p.WhiteHandView.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		switch {
		case e.Key() == tcell.KeyDown, e.Rune() == 'j':
			p.BoardView.SetSelectable(true, true)
			p.WhiteHandView.SetSelectable(false, false)
			p.App.SetFocus(p.BoardView)
			return nil
		}
		return e
	})
}

func (p *PositionView) promotionModal(toSq shogi.Square) {
	modal := tview.NewModal().
		SetText("成りますか？").
		AddButtons([]string{"はい", "いいえ"}).
		SetDoneFunc(func(idx int, label string) {
			var promotion bool
			if idx == 0 {
				promotion = true
			}
			move := shogi.NewNormalMove(p.From.square, toSq, promotion)
			p.View.Move(move)
			p.Pages.RemovePage("modal").ShowPage("main")
		})

	p.Pages.AddAndSwitchToPage("modal", modal, true).ShowPage("main")
}

type MovesView struct {
	*tview.Table
	View     *View
	Game     *shogi.GameTree
	MoveList []shogi.MoveData
}

func NewMovesView(view *View, game *shogi.GameTree) *MovesView {
	movesView := &MovesView{
		Table:    tview.NewTable(),
		View:     view,
		Game:     game,
		MoveList: []shogi.MoveData{},
	}
	movesView.SetBorder(true).SetTitle("棋譜")
	movesView.SetSelectable(true, false)
	movesView.Select(0, 0)
	movesView.SetSelectionChangedFunc(func(row, col int) {
		game.GotoNth(game.Root.Position.Ply + row)
		view.UpdateView()
	})
	return movesView
}

func (m *MovesView) UpdateView() {
	// 棋譜リストを毎回一から作り直すのは効率悪い
	moves := []shogi.MoveData{}
	node := m.Game.Root
	for node != nil {
		moves = append(moves, node.MoveData)
		node = node.Next
	}
	m.MoveList = moves

	m.Clear()
	for i, move := range m.MoveList {
		if move.IsInitialMove() {
			m.SetCell(i, 0, tview.NewTableCell("=== 開始局面 ==="))
			continue
		}
		pre := "☗"
		if move.Color == shogi.White {
			pre = "☖"
		}
		kif := fmt.Sprintf("%d %s%s", move.Ply, pre, move.KIF())
		m.SetCell(i, 0, tview.NewTableCell(kif))
	}
}

// KIF形式の読み筋を持つ
type USIInfoWithKIFPv struct {
	engine.USIInfo
	Pv []string
}

func USIMovesToKIFMoves(usiMoves []string, position *shogi.Position, beforeSq shogi.Square) ([]string, bool) {
	var pv []string
	p := position.Clone()
	before := beforeSq
	for _, usi := range usiMoves {
		move, _ := shogi.NewMoveFromUSI(usi)
		moveData := shogi.NewMoveData(move, p, before)
		pre := "☗"
		if moveData.Color == shogi.White {
			pre = "☖"
		}
		pv = append(pv, pre+moveData.KIF())
		before = move.To
		if err := p.Move(move); err != nil {
			// エンジン思考中に駒を動かすと前の局面の思考が送られて来てしまうことがある
			// log.Printf("illegal move: position %v, move %v\n", p, move)
			return nil, false
		}
	}
	return pv, true
}

func NewUSIInfoWithKIFPv(info engine.USIInfo, p *shogi.Position, before shogi.Square) (USIInfoWithKIFPv, bool) {
	kifmoves, ok := USIMovesToKIFMoves(info.Pv, p, before)
	if !ok {
		return USIInfoWithKIFPv{}, false
	}
	return USIInfoWithKIFPv{info, kifmoves}, true
}

type AnalysisView struct {
	*tview.Table
	Infos []USIInfoWithKIFPv
}

func NewAnalysisView() *AnalysisView {
	analysisView := &AnalysisView{
		Table: tview.NewTable(),
	}
	analysisView.SetBorder(true).SetTitle("検討")
	analysisView.SetCell(0, 0, tview.NewTableCell("R"))
	analysisView.SetCell(0, 1, tview.NewTableCell("時間"))
	analysisView.SetCell(0, 2, tview.NewTableCell("深さ"))
	analysisView.SetCell(0, 3, tview.NewTableCell("ノード数"))
	analysisView.SetCell(0, 4, tview.NewTableCell("評価値"))
	analysisView.SetCell(0, 5, tview.NewTableCell("読み筋"))

	analysisView.SetFixed(1, 0)
	return analysisView
}

func (a *AnalysisView) Clear() {
	a.Table.Clear()

	a.SetCell(0, 0, tview.NewTableCell("R"))
	a.SetCell(0, 1, tview.NewTableCell("時間"))
	a.SetCell(0, 2, tview.NewTableCell("深さ"))
	a.SetCell(0, 3, tview.NewTableCell("ノード数"))
	a.SetCell(0, 4, tview.NewTableCell("評価値"))
	a.SetCell(0, 5, tview.NewTableCell("読み筋"))
	a.SetFixed(1, 0)

	a.Infos = []USIInfoWithKIFPv{}
}

func (a *AnalysisView) AppendInfo(info engine.USIInfo, p *shogi.Position, before shogi.Square) {
	infoWithKIFPv, ok := NewUSIInfoWithKIFPv(info, p, before)
	if !ok {
		return
	}
	a.Infos = append(a.Infos, infoWithKIFPv)
	a.updateOneInfo(len(a.Infos), infoWithKIFPv)
}

func (a *AnalysisView) updateOneInfo(row int, info USIInfoWithKIFPv) {
	multipv := tview.NewTableCell(strconv.Itoa(info.MultiPv))
	a.SetCell(row, 0, multipv)

	time := tview.NewTableCell(strconv.Itoa(info.Time))
	a.SetCell(row, 1, time)

	depth := tview.NewTableCell(strconv.Itoa(info.Depth))
	a.SetCell(row, 2, depth)

	nodes := tview.NewTableCell(strconv.Itoa(info.Nodes))
	a.SetCell(row, 3, nodes)

	score := tview.NewTableCell(strconv.Itoa(info.ScoreCp))
	if info.IsMate {
		score = tview.NewTableCell(fmt.Sprintf("mate %d", info.ScoreMate))
	}
	a.SetCell(row, 4, score)

	pv := tview.NewTableCell(strings.Join(info.Pv, " "))
	a.SetCell(row, 5, pv)
}

type View struct {
	PositionView    *PositionView
	MovesView       *MovesView
	AnalysisView    *AnalysisView
	EngineStateView *tview.TextView
	Game            *shogi.GameTree
	Engine          *engine.Engine
	EngineInfoC     chan engine.USIInfo
	App             *tview.Application
	Pages           *tview.Pages
	Panels
	Config *Config
}

func (v *View) UpdateView() {
	v.PositionView.UpdateView()
	v.MovesView.UpdateView()
	v.AnalysisView.Clear()

	// 局面が更新された時に自動的に新しい局面でエンジンに思考させる
	if v.Engine != nil && v.Engine.State == engine.Thinking {
		v.analyze()
	}
}

func (v *View) engineStateViewUpdateLoop() {
	ticker := time.NewTicker(time.Millisecond * 300)
	for range ticker.C {
		if v.Engine != nil {
			text := v.Engine.Name + ": "
			switch v.Engine.State {
			case engine.Initialized:
				text += "初期化完了"
			case engine.WaitingUSIOk:
				text += "waiting usiok"
			case engine.WaitingReadyOk:
				text += "waiting readyok"
			case engine.Idling:
				text += "待機中"
			case engine.Thinking:
				text += "検討中"
			}
			v.App.QueueUpdateDraw(func() {
				v.EngineStateView.SetText(text)
			})
		}
	}
}

type Panels struct {
	Current int
	Panels  []tview.Primitive
}

func NewView(config *Config) *View {
	analysisView := NewAnalysisView()

	app := tview.NewApplication()
	game := shogi.NewGameTree()
	pages := tview.NewPages()

	v := &View{
		AnalysisView:    analysisView,
		EngineStateView: tview.NewTextView(),
		Game:            game,
		App:             app,
		Pages:           pages,
		Config:          config,
	}
	positionView := NewPositionView(app, v, pages, game)
	movesView := NewMovesView(v, game)

	v.PositionView = positionView
	v.MovesView = movesView

	v.Panels = Panels{
		Panels: []tview.Primitive{
			positionView,
			movesView,
			analysisView,
		},
	}

	v.EngineInfoC = make(chan engine.USIInfo, 128)

	v.setup()

	v.EngineStateView.SetText("エンジンなし")
	go v.engineStateViewUpdateLoop()

	return v
}

func (v *View) setup() {
	v.App.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		switch v.App.GetFocus() {
		case v.PositionView.BoardView, v.PositionView.BlackHandView, v.PositionView.WhiteHandView:
			if e.Rune() == ' ' {
				return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
			}
		}
		switch e.Key() {
		case tcell.KeyTab:
			v.nextPanel()
			return nil
		case tcell.KeyBacktab:
			v.prevPanel()
			return nil
		}
		switch e.Rune() {
		case 'v':
			// エンジンの起動・思考開始/停止をvキー1つで担う
			if v.Engine == nil {
				var err error
				v.Engine, err = engine.NewEngine(v.Config.EnginePath)
				if err != nil {
					log.Println(err)
					return nil
				}
				v.Engine.InfoC = v.EngineInfoC
				go v.readEngineInfo()
				v.Engine.SendUSI()
				for _, option := range v.Config.EngineOptions {
					v.Engine.SendSetOption(option[0], option[1])
				}
				v.Engine.SendIsReady()
				log.Println("engine initialized")
				v.analyze()
				return nil
			}
			switch v.Engine.State {
			case engine.Idling:
				v.analyze()
			case engine.Thinking:
				v.Engine.SendStop()
				log.Println("> stop")
			}
			return nil
		}
		return e
	})
}

func (v *View) analyze() {
	if v.Engine == nil {
		return
	}
	if v.Engine.State == engine.Thinking {
		v.Engine.SendStop()
		log.Println("> stop")
	}
	go func() {
		// Idlingになるまで待つ
		ticker := time.NewTicker(time.Millisecond * 100)
		defer ticker.Stop()
		for range ticker.C {
			if v.Engine.State == engine.Idling && v.Engine.ReadyOk {
				v.Engine.SendSFEN(v.Game.Current.Position.SFEN(), nil)
				v.Engine.GoInfinite()
				log.Println(">", v.Game.Current.Position.SFEN())
				break
			}
		}
	}()
}

func (v *View) readEngineInfo() {
	for {
		for info := range v.EngineInfoC {
			v.App.QueueUpdateDraw(func() {
				p := v.Game.Current.Position
				before := shogi.NullSquare
				if v.Game.Current.Prev != nil {
					before = v.Game.Current.Prev.MoveData.To
				}
				v.AnalysisView.AppendInfo(info, p, before)
			})
		}
	}
}

func (v *View) nextPanel() {
	v.Panels.Current++
	if v.Panels.Current >= len(v.Panels.Panels) {
		v.Panels.Current = 0
	}
	v.App.SetFocus(v.Panels.Panels[v.Panels.Current])
}

func (v *View) prevPanel() {
	v.Panels.Current--
	if v.Panels.Current < 0 {
		v.Panels.Current = len(v.Panels.Panels) - 1
	}
	v.App.SetFocus(v.Panels.Panels[v.Panels.Current])
}

func (v *View) Move(m shogi.Move) {
	moveErr := v.PositionView.Game.Move(m)
	v.PositionView.From.Cancel()
	if moveErr == nil {
		v.MovesView.Select(v.Game.Current.Position.Ply-v.Game.Root.Position.Ply, 0)
	} else {
		// selectFromのキャンセルを反映させる
		v.PositionView.UpdateView()
	}
}

func (v *View) Run() error {
	grid := tview.NewGrid().
		SetRows(17, 0).
		SetColumns(38, 0).
		AddItem(v.PositionView, 0, 0, 1, 1, 0, 0, true).
		AddItem(v.MovesView, 0, 1, 1, 1, 0, 0, false).
		AddItem(v.AnalysisView, 1, 0, 1, 2, 0, 0, false)

	grid2 := tview.NewGrid().
		SetRows(0, 1).
		AddItem(grid, 0, 0, 1, 1, 0, 0, true).
		AddItem(v.EngineStateView, 1, 0, 1, 1, 0, 0, false)

	v.Pages.AddAndSwitchToPage("main", grid2, true)
	v.UpdateView()

	if err := v.App.SetRoot(v.Pages, true).Run(); err != nil {
		v.App.Stop()
		log.Println(err)
		return err
	}
	return nil
}

type Config struct {
	EnginePath    string     `json:"engine_path"`
	EngineOptions [][]string `json:"engine_options"`
}

const defaultConfigJSON = `{
  "engine_path": "/path/to/yaneuraou",
  "engine_options": [
    ["Threads", "1"],
    ["MultiPv", "3"]
  ]
}
`

func main() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	configFile := filepath.Join(configDir, "termshogi", "config.json")

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if err := os.Mkdir(filepath.Dir(configFile), 0777); err != nil {
			panic(err)
		}
		if err := ioutil.WriteFile(configFile, []byte(defaultConfigJSON), 0644); err != nil {
			panic(err)
		}
	}

	bytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		panic(err)
	}

	var config Config
	if err := json.Unmarshal(bytes, &config); err != nil {
		panic(err)
	}

	logWriter, err := os.OpenFile("termshogi.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	log.SetOutput(logWriter)

	view := NewView(&config)
	if err := view.Run(); err != nil {
		panic(err)
	}
	if view.Engine != nil {
		view.Engine.Close()
	}
}
