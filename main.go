package main

import (
	"ebi/npc"
	"ebi/player"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type GameState int

const (
	MenuState GameState = iota
	PlayState
	OptionsState
	TransitionState
	NewSceneState
	CutsceneState
)

type CutsceneActionType int

const (
	MovePlayer CutsceneActionType = iota
	MoveNPC
	ShowDialogue
	TeleportNPC
	TeleportPlayer
	TurnNPC
	TurnPlayer
	FadeIn
	FadeOut
	ChangeScene
)

type CutsceneAction struct {
	ActionType   CutsceneActionType
	Target       interface{}
	Data         interface{}
	WaitPrevious bool // Whether to wait for previous actions to complete
}
type Vector2D struct {
	X, Y float64
}

type Cutscene struct {
	Game          *Game `json:"-"`
	Actions       []CutsceneAction
	Current       int
	ActiveActions map[int]bool // Tracks active actions by their index
	IsPlaying     bool
	CleanUp       func(*Cutscene) `json:"-"`
}

type GameProgress struct {
	HasVisitedRedTown     bool
	HasMetNPCBryan        bool
	FirstCutSceneFinished bool
}

type Door struct {
	Rect        *image.Rectangle
	Id          string
	Destination string
	NewX, NewY  float64
}

type Scene struct {
	Name                   string
	Game                   *Game `json:"-"`
	obstacles              []*image.Rectangle
	doors                  []*Door
	Background, Foreground *ebiten.Image
	loadObsnDoors          func(*Game) `json:"-"`
	loadNPCs               func(*Game) `json:"-"`
	NPCs                   []*npc.NPC
}

type Dialogue struct {
	TextLines         []string
	CurrentLine       int
	CharIndex         int
	FramesPerChar     int // Number of frames to wait before showing the next character
	AccumulatedFrames int // Frame counter for the typewriter effect
	IsOpen            bool
	Finished          bool
}

type Game struct {
	player                  *player.Player
	state                   GameState
	alpha                   float64 // For the fade effect (0.0: fully transparent, 1.0: fully opaque)
	fadeSpeed               float64 // How fast the fade occurs
	menuOptions             []string
	Scenes                  map[string]*Scene
	Progress                GameProgress
	Cutscene                Cutscene
	CurrentScene, NextScene string
	CurrentDoor             *Door
	selectedOption          int
	keyPressCounter         map[ebiten.Key]int // Tracks duration of key presses
	keyKPressedLastFrame    bool
	keyZPressedLastFrame    bool
	// keyRPressedLastFrame    bool
	dialogue *Dialogue
	fface    font.Face
	Full     bool
}

type SaveState struct {
	PlayerDirection string
	PlayerPosition  Vector2D
	NPCPositions    []Vector2D
	CurrentScene    string
	GameProgress
}

func SaveGameState(state *SaveState, filename string) error {
	fmt.Println(state)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(state); err != nil {
		return err
	}

	return nil
}

func LoadGameState(filename string) (*SaveState, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var state SaveState
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&state); err != nil {
		return nil, err
	}

	return &state, nil
}

func (g *Game) Update() error {
	if g.keyPressCounter == nil {
		g.keyPressCounter = make(map[ebiten.Key]int)
	}
	if g.state == OptionsState {
		os.Exit(0)
	} else if g.state == MenuState {
		keys := []ebiten.Key{
			ebiten.KeyUp,
			ebiten.KeyDown,
		}
		for _, key := range keys {
			if ebiten.IsKeyPressed(key) {
				g.keyPressCounter[key]++
			} else {
				g.keyPressCounter[key] = 0
			}
		}

		// Change the selected option based on input
		if g.keyPressCounter[ebiten.KeyDown] == 1 { // Only move down if it's the first frame the key is pressed
			g.selectedOption = (g.selectedOption + 1) % len(g.menuOptions)
		} else if g.keyPressCounter[ebiten.KeyUp] == 1 { // Only move up if it's the first frame the key is pressed
			g.selectedOption--
			if g.selectedOption < 0 {
				g.selectedOption = len(g.menuOptions) - 1
			}
		}

		// Select an option
		if ebiten.IsKeyPressed(ebiten.KeyEnter) {
			switch g.selectedOption {
			case 0: // Start the game
				g.state = PlayState
			case 1: // Options (if you have any)
				g.state = OptionsState
			case 2: // Exit
				os.Exit(0)
			}
		}
	} else if g.state == PlayState {
		for _, cnpc := range g.Scenes[g.CurrentScene].NPCs {
			if nearNPC(minX(g.player.X, g), minY(g.player.Y, g), cnpc.X, cnpc.Y) {
				if ebiten.IsKeyPressed(ebiten.KeyZ) && !g.keyZPressedLastFrame {
					// Toggle NPC interaction state
					if cnpc.InteractionState == npc.NoInteraction {
						cnpc.InteractionState = npc.PlayerInteracted
						g.Progress.HasMetNPCBryan = true
						g.player.CanMove = false // Disallow player movement
					} else if cnpc.InteractionState == npc.WaitingForPlayerToResume && g.dialogue.Finished && g.dialogue.IsLastLine() {
						cnpc.InteractionState = npc.NoInteraction
						g.player.CanMove = true // Allow player movement
					}
				}
			}
			cnpc.Update(ebiten.KeyZ)
			if ebiten.IsKeyPressed(ebiten.KeyZ) && !g.keyZPressedLastFrame && nearNPC(minX(g.player.X, g), minY(g.player.Y, g), cnpc.X, cnpc.Y) {
				if !g.dialogue.IsOpen {
					g.dialogue.IsOpen = true
					g.dialogue.CurrentLine = 0
					g.dialogue.CharIndex = 0
					g.dialogue.Finished = false
					g.dialogue.TextLines = cnpc.DialogueText
				} else {
					if g.dialogue.Finished {
						g.dialogue.NextLine()
					} else {
						// Instantly display all characters in the current line
						g.dialogue.CharIndex = len(g.dialogue.TextLines[g.dialogue.CurrentLine])
						g.dialogue.Finished = true
					}
				}

			}

			g.dialogue.Update()

		}
		// fmt.Println("Player:", g.player.X, g.player.Y)
		// fmt.Println("NPC:", g.Scenes[g.CurrentScene].NPCs[0].X, g.Scenes[g.CurrentScene].NPCs[0].Y)
		g.keyZPressedLastFrame = ebiten.IsKeyPressed(ebiten.KeyZ)
		if g.Progress.HasMetNPCBryan && g.Progress.HasVisitedRedTown && !g.dialogue.IsOpen && !g.Progress.FirstCutSceneFinished && g.Scenes[g.CurrentScene] == g.Scenes["mainMapRed"] {
			g.Cutscene = createExampleCutscene(g)
			g.Cutscene.Start()
			g.state = CutsceneState
			// fmt.Println("Progress Attained")
			// n := g.Scenes[g.CurrentScene].NPCs[0]
			// p := g.player

			// g.player.X = -500
			// g.player.Y = -500
			// if nearNPC(minX(g.player.X, g), minY(g.player.Y, g), n.X, n.Y) {
			// 	n.IsStopped = true
			// 	n.StopTimer = 100
			// }
			// p.X += p.Speed
			// p.Y += p.Speed
			// n.X += n.Speed
			// n.Y += n.Speed
			// // n.X =
			// // n.Y =
			// // p.CanMove = false
			// p.Direction = "left"
			// p.CurrentFrame = 1
			// g.Progress.FirstCutSceneFinished = true
			// t := false
			// g.alpha += g.fadeSpeed
			// if g.alpha >= 1.0 {
			// 	g.alpha = 1.0
			// 	t = true
			// 	// g.player.X = g.CurrentDoor.NewX
			// 	// g.player.Y = g.CurrentDoor.NewY
			// 	// g.Scenes[g.CurrentScene].loadObsnDoors(g)
			// 	// g.Scenes[g.CurrentScene].loadNPCs(g)
			// } else if t {
			// 	fmt.Println("Went Black")
			// 	// Decrease the alpha for the fade in effect
			// 	g.alpha -= g.fadeSpeed
			// 	if g.alpha <= 0.0 {
			// 		g.alpha = 0.0
			// 		g.state = PlayState
			// 		// The new scene is fully visible now, and game continues as normal
			// 	}
			// }
		}
		colliding := false
		movementKeyPressed := false
		var moveX, moveY float64
		if g.player.CanMove {

			// Handle player movement
			if ebiten.IsKeyPressed(ebiten.KeyLeft) {
				X, Y := g.player.CheckMove("left")
				g.player.Direction = "left"
				moveX = X
				moveY = Y
				movementKeyPressed = true
			}
			if ebiten.IsKeyPressed(ebiten.KeyRight) {
				X, Y := g.player.CheckMove("right")
				g.player.Direction = "right"
				moveX = X
				moveY = Y
				movementKeyPressed = true
			}
			if ebiten.IsKeyPressed(ebiten.KeyUp) {
				X, Y := g.player.CheckMove("up")
				g.player.Direction = "up"
				moveX = X
				moveY = Y
				movementKeyPressed = true
			}
			if ebiten.IsKeyPressed(ebiten.KeyDown) {
				X, Y := g.player.CheckMove("down")
				g.player.Direction = "down"
				moveX = X
				moveY = Y
				movementKeyPressed = true
			}
		}

		// if moveX != 0 {
		// 	fmt.Println(minX(moveX, g), minY(moveY, g))
		// 	fmt.Println(*g.obstacles[0])
		// }

		for _, obstacle := range g.Scenes[g.CurrentScene].obstacles {
			obsMinX := float64(obstacle.Min.X)
			obsMaxX := float64(obstacle.Max.X)
			obsMinY := float64(obstacle.Min.Y)
			obsMaxY := float64(obstacle.Max.Y)
			if obsMinX < minX(moveX, g) && obsMaxX > maxX(moveX, g) && obsMinY < minY(moveY, g) && obsMaxY > maxY(moveY, g) {
				moveX = g.player.X
				moveY = g.player.Y
				colliding = true
			}
		}
		for _, door := range g.Scenes[g.CurrentScene].doors {
			obsMinX := float64(door.Rect.Min.X)
			obsMaxX := float64(door.Rect.Max.X)
			obsMinY := float64(door.Rect.Min.Y)
			obsMaxY := float64(door.Rect.Max.Y)
			if obsMinX < minX(moveX, g) && obsMaxX > maxX(moveX, g) && obsMinY < minY(moveY, g) && obsMaxY > maxY(moveY, g) {
				if colliding {
					g.state = TransitionState
					g.CurrentDoor = door
					g.Progress.HasVisitedRedTown = true
				}
			}
		}
		// Check for proximity and key press to interact with the NPC

		// if ebiten.IsKeyPressed(ebiten.KeyR) && !g.keyRPressedLastFrame {
		// 	if !g.dialogue.IsOpen {
		// 		g.dialogue.IsOpen = true
		// 		g.dialogue.CurrentLine = 0
		// 		g.dialogue.CharIndex = 0
		// 		g.dialogue.Finished = false
		// 	} else {
		// 		if g.dialogue.Finished {
		// 			g.dialogue.NextLine()
		// 		} else {
		// 			// Instantly display all characters in the current line
		// 			g.dialogue.CharIndex = len(g.dialogue.TextLines[g.dialogue.CurrentLine])
		// 			g.dialogue.Finished = true
		// 		}

		// 	}
		// 	g.dialogue.Update()
		// }
		// g.keyRPressedLastFrame = ebiten.IsKeyPressed(ebiten.KeyR)

		if ebiten.IsKeyPressed(ebiten.KeyK) && !g.keyKPressedLastFrame {
			g.Full = !g.Full
			ebiten.SetFullscreen(g.Full)
			s := &SaveState{
				PlayerPosition:  Vector2D{g.player.X, g.player.Y},
				PlayerDirection: g.player.Direction,
				CurrentScene:    g.CurrentScene,
				GameProgress:    g.Progress,
				NPCPositions:    []Vector2D{{X: g.Scenes[g.CurrentScene].NPCs[0].X, Y: g.Scenes[g.CurrentScene].NPCs[0].Y}},
			}
			err := SaveGameState(s, "savefile.json")
			if err != nil {
				log.Fatal(err)
			}
			// g.Progress.FirstCutSceneFinished = !g.Progress.FirstCutSceneFinished
			// g.Progress.HasMetNPCBryan = false
			// g.Progress.HasVisitedRedTown = false

			// if g.CurrentScene == g.Scenes["mainMap"] {
			// 	g.changeScene(g.Scenes["mainMap"], g.Scenes["mainMapRed"])
			// } else {
			// 	g.changeScene(g.Scenes["mainMapRed"], g.Scenes["mainMap"])
			// }

		}
		g.keyKPressedLastFrame = ebiten.IsKeyPressed(ebiten.KeyK)
		if g.player.CanMove {
			// Handle player movement
			if ebiten.IsKeyPressed(ebiten.KeyLeft) {
				g.player.X = moveX
				movementKeyPressed = true
			}
			if ebiten.IsKeyPressed(ebiten.KeyRight) {
				g.player.X = moveX
				movementKeyPressed = true
			}
			if ebiten.IsKeyPressed(ebiten.KeyUp) {
				g.player.Y = moveY
				movementKeyPressed = true
			}
			if ebiten.IsKeyPressed(ebiten.KeyDown) {
				g.player.Y = moveY
				movementKeyPressed = true
			}
			if movementKeyPressed {
				// Increment the tick count
				g.player.TickCount++
			}

		}

		// Update the current frame every 10 ticks
		if g.player.TickCount >= 10 {
			g.player.CurrentFrame = (g.player.CurrentFrame + 1) % g.player.FrameCount
			g.player.TickCount = 0 // Reset the tick count
		}
	} else if g.state == TransitionState {
		// Increase the alpha for the fade out effect
		g.alpha += g.fadeSpeed
		if g.alpha >= 1.0 {
			g.alpha = 1.0
			g.state = NewSceneState
			g.changeScene(g.CurrentScene, g.CurrentDoor.Destination)
			g.player.X = g.CurrentDoor.NewX
			g.player.Y = g.CurrentDoor.NewY
			g.Scenes[g.CurrentScene].loadObsnDoors(g)
			g.Scenes[g.CurrentScene].loadNPCs(g)
		}
	} else if g.state == NewSceneState {
		// Decrease the alpha for the fade in effect
		g.alpha -= g.fadeSpeed
		if g.alpha <= 0.0 {
			g.alpha = 0.0
			g.state = PlayState
			// The new scene is fully visible now, and game continues as normal
		}
	} else if g.state == CutsceneState {
		g.Cutscene.Update()

		g.keyZPressedLastFrame = ebiten.IsKeyPressed(ebiten.KeyZ)
	}
	return nil
}
func CleanUpCutScene1(c *Cutscene) {
	c.IsPlaying = false
	c.Game.Scenes[c.Game.CurrentScene].NPCs[0].X = -900
	c.Game.Scenes[c.Game.CurrentScene].NPCs[0].Y = -950
	c.Game.Scenes[c.Game.CurrentScene].NPCs[0].Direction = "left"
	c.Game.state = PlayState
	c.Game.Progress.FirstCutSceneFinished = true
	fmt.Println("finished")
}
func (c *Cutscene) Start() {
	c.Current = 0
	c.IsPlaying = true
	c.ActiveActions = make(map[int]bool)
}

func (c *Cutscene) Update() {
	if !c.IsPlaying {
		return
	}

	for i, action := range c.Actions {
		if i < c.Current && !c.ActiveActions[i] {
			// Skip completed actions
			continue
		}

		if action.WaitPrevious && i > c.Current {
			// If the action should wait for previous ones, don't start it yet
			continue
		}

		// Process the action
		completed := c.processAction(action)
		if completed {
			c.ActiveActions[i] = false // Mark action as completed
			if i == c.Current {
				c.Current++ // Move to the next action
			}
		} else {
			c.ActiveActions[i] = true // Mark action as active
		}
	}

	// Check if all actions are completed
	if c.Current >= len(c.Actions) {
		c.CleanUp(c)
		c.Game.state = PlayState
	}
}

func (c *Cutscene) processAction(action CutsceneAction) bool {
	switch action.ActionType {
	case MoveNPC:
		cnpc := action.Target.(*npc.NPC)
		destination := action.Data.(Vector2D)
		return moveTowards(cnpc, destination)
	case MovePlayer:
		p := action.Target.(*player.Player)
		destination := action.Data.(Vector2D)
		return moveTowards(p, destination)
	case FadeOut:
		g := c.Game
		g.alpha += action.Data.(float64)
		f := false
		if g.alpha >= 1.0 {
			g.alpha = 1.0
			f = true
		}
		return f
	case FadeIn:
		g := c.Game
		g.alpha -= action.Data.(float64)
		f := false
		if g.alpha <= 0.0 {
			g.alpha = 0.0
			f = true
			// The new scene is fully visible now, and game continues as normal
		}
		return f
	case TeleportPlayer:
		p := action.Target.(*player.Player)
		destination := action.Data.(Vector2D)
		p.X = destination.X
		p.Y = destination.Y
		return true
	case TeleportNPC:
		cnpc := action.Target.(*npc.NPC)
		destination := action.Data.(Vector2D)
		cnpc.X = destination.X
		cnpc.Y = destination.Y
		return true
	case TurnPlayer:
		p := action.Target.(*player.Player)
		dir := action.Data.(string)
		p.Direction = dir
		return true
	case TurnNPC:
		p := action.Target.(*npc.NPC)
		dir := action.Data.(string)
		p.Direction = dir
		return true
	case ShowDialogue:
		d := action.Target.(*Dialogue)
		if !d.IsOpen {
			d.IsOpen = true
			d.CurrentLine = 0
			d.CharIndex = 0
			d.Finished = false
			d.TextLines = action.Data.([]string)
		} else {
			g := c.Game
			if ebiten.IsKeyPressed(ebiten.KeyZ) && !g.keyZPressedLastFrame {
				if d.Finished {
					d.NextLine()
					if !d.IsOpen {
						return true
					}
				} else {
					// Instantly display all characters in the current line
					d.CharIndex = len(d.TextLines[d.CurrentLine])
					d.Finished = true
				}
				g.keyZPressedLastFrame = ebiten.IsKeyPressed(ebiten.KeyZ)

			}
			d.Update()
			// return d.Finished
		}
	}
	return false
}

func moveTowards(entity interface{}, target Vector2D) bool {
	const speed = 5.0
	res := false
	switch e := entity.(type) {
	case *player.Player:
		if e.X < target.X {
			e.Direction = "left"
			e.X += speed
		} else if e.X > target.X {
			e.Direction = "right"
			e.X -= speed
		} else if e.Y > target.Y {
			e.Direction = "down"
			e.Y -= speed
		} else if e.Y < target.Y {
			e.Direction = "up"
			e.Y += speed
		} else {
			e.CurrentFrame = 2
			res = true
		}
		e.TickCount++
		if e.TickCount >= 10 && !res {
			e.CurrentFrame = (e.CurrentFrame + 1) % e.FrameCount
			e.TickCount = 0 // Reset the tick count
		}
	case *npc.NPC:
		if e.X < target.X {
			e.Direction = "left"
			e.X += speed
		} else if e.X > target.X {
			e.Direction = "right"
			e.X -= speed
		} else if e.Y > target.Y {
			e.Direction = "down"
			e.Y -= speed
		} else if e.Y < target.Y {
			e.Direction = "up"
			e.Y += speed
		} else {
			// e.CurrentFrame = 2
			res = true
		}
		e.TickCount++
		if e.TickCount >= 10 && !res {
			e.CurrentFrame = (e.CurrentFrame + 1) % e.FrameCount
			e.TickCount = 0 // Reset the tick count
		}
	default:
		res = true
	}
	return res
}

func createExampleCutscene(g *Game) Cutscene {
	return Cutscene{
		CleanUp: CleanUpCutScene1,
		Game:    g,
		Actions: []CutsceneAction{
			{
				ActionType:   FadeOut,
				Data:         0.01,
				WaitPrevious: false,
			},
			{
				ActionType:   TeleportPlayer,
				Target:       g.player,
				Data:         Vector2D{X: 100, Y: 100},
				WaitPrevious: true,
			},
			{
				ActionType:   TeleportNPC,
				Target:       g.Scenes[g.CurrentScene].NPCs[0],
				Data:         Vector2D{X: 150 - 600, Y: 150 - 400},
				WaitPrevious: true,
			},
			{
				ActionType:   FadeIn,
				Data:         0.01,
				WaitPrevious: true,
			},
			{
				ActionType:   MovePlayer,
				Target:       g.player,
				Data:         Vector2D{X: -100, Y: -140}, // Target position for player
				WaitPrevious: true,
			},
			{
				ActionType:   MoveNPC,
				Target:       g.Scenes[g.CurrentScene].NPCs[0],
				Data:         Vector2D{X: -150 - 600, Y: -150 - 400}, // Target position for NPC
				WaitPrevious: false,
			},
			{
				ActionType:   TurnNPC,
				Target:       g.Scenes[g.CurrentScene].NPCs[0],
				Data:         "left",
				WaitPrevious: true,
			},
			{
				ActionType:   TurnPlayer,
				Target:       g.player,
				Data:         "right",
				WaitPrevious: true,
			},
			{
				ActionType:   ShowDialogue,
				Target:       g.dialogue,
				Data:         []string{"This is our first Scene.", "Pretty Cool huh?"},
				WaitPrevious: true,
			},
		},
	}
}
func (g *Game) StartCutscene() {

	g.Cutscene = createExampleCutscene(g)
	g.Cutscene.Start()

}

//	func (c *Cutscene) Draw(screen *ebiten.Image) {
//		// ... draw anything related to the cutscene ...
//	}
func (d *Dialogue) IsLastLine() bool {
	return d.CurrentLine == len(d.TextLines)-1
}
func nearNPC(playerX, playerY, npcX, npcY float64) bool {
	npcX *= -1
	npcY *= -1
	npcX += 192 / 4
	npcY += 68
	// Define what "near" means, e.g., within 50 pixels
	const proximityThreshold = 50.0
	return math.Abs(playerX-npcX) < proximityThreshold && math.Abs(playerY-npcY) < proximityThreshold
}
func minX(moveX float64, g *Game) float64 {
	screenWidth, _ := ebiten.WindowSize()
	return ((moveX - float64(screenWidth)) * -1)
}
func maxX(moveX float64, g *Game) float64 {
	screenWidth, _ := ebiten.WindowSize()
	return ((moveX - float64(screenWidth) + float64(g.player.FrameWidth)) * -1)
}
func minY(moveY float64, g *Game) float64 {
	_, screenHeight := ebiten.WindowSize()
	return ((moveY - float64(screenHeight)) * -1)
}
func maxY(moveY float64, g *Game) float64 {
	_, screenHeight := ebiten.WindowSize()
	return ((moveY - float64(screenHeight) + float64(g.player.FrameHeight)) * -1)
}

func loadFontFace() (font.Face, error) {
	// Read the font data
	fontBytes := fonts.MPlus1pRegular_ttf

	// Parse the font data
	fontParsed, err := opentype.Parse(fontBytes)
	if err != nil {
		log.Fatal(err)
	}

	// Specify the font size
	const dpi = 72
	face, err := opentype.NewFace(fontParsed, &opentype.FaceOptions{
		Size:    12,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}

	return face, nil
}

func (d *Dialogue) Update() {
	if !d.IsOpen || d.Finished {
		return
	}

	d.AccumulatedFrames++
	if d.AccumulatedFrames >= d.FramesPerChar {
		d.AccumulatedFrames = 0
		d.CharIndex++
		if d.CharIndex > len(d.TextLines[d.CurrentLine]) {
			d.CharIndex = len(d.TextLines[d.CurrentLine])
			d.Finished = true
		}
	}
}

func (d *Dialogue) NextLine() {
	if d.CurrentLine < len(d.TextLines)-1 {
		d.CurrentLine++
		d.CharIndex = 0
		d.Finished = false
	} else {
		// No more lines, close the dialogue
		d.IsOpen = false
	}
}
func (d *Dialogue) Draw(screen *ebiten.Image, g *Game) {
	if !d.IsOpen {
		return
	}

	// Set up the dialogue box dimensions
	boxWidth := screen.Bounds().Dx() - 20         // 10 pixels padding on each side
	boxHeight := 63                               // Fixed height for the dialogue box
	boxX := 10                                    // X position of the box
	boxY := screen.Bounds().Dy() - boxHeight - 10 // Y position of the box, 10 pixels above the bottom of the screen

	// Draw the dialogue box background
	dialogueBox := ebiten.NewImage(boxWidth, boxHeight)
	dialogueBox.Fill(color.Black)
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(float64(boxX), float64(boxY))
	screen.DrawImage(dialogueBox, opts)
	charImg, _, err := ebitenutil.NewImageFromFile("assets/animBoy.png")
	if err != nil {
		log.Fatal(err)
	}
	charOpts := &ebiten.DrawImageOptions{}
	charOpts.GeoM.Translate(float64(boxX), float64(boxY))
	screen.DrawImage(charImg, charOpts)
	fontFace := g.fface
	if err != nil {
		log.Fatal(err)
	}
	// Draw the text with the typewriter effect
	textToDisplay := d.TextLines[d.CurrentLine][:d.CharIndex]
	text.Draw(screen, wrapText(textToDisplay, 225, fontFace), fontFace, boxX+70, boxY+17, color.White) // +10 for text padding, +30 to vertically center
}
func (g *Game) Draw(screen *ebiten.Image) {
	if g.state == MenuState {
		fontFace := g.fface
		x := 4
		y := 20
		spacing := 20
		for i, option := range g.menuOptions {
			// Change color or style if option is selected
			col := color.White
			if i == g.selectedOption {
				col = color.Black // Highlighted color
			}
			text.Draw(screen, option, fontFace, x, y+i*spacing, col)
		}
	} else if g.state == PlayState {
		scale := 0.25
		bgOpts := &ebiten.DrawImageOptions{}
		bgOpts.GeoM.Translate(g.player.X, g.player.Y)
		bgOpts.GeoM.Scale(scale, scale)
		screen.DrawImage(g.Scenes[g.CurrentScene].Background, bgOpts)
		currentSpriteSheet := g.player.SpriteSheets[g.player.Direction]
		// 	// Determine the x, y location of the current frame on the sprite sheet
		sx := (g.player.CurrentFrame % (currentSpriteSheet.Bounds().Dx() / g.player.FrameWidth)) * g.player.FrameWidth
		sy := (g.player.CurrentFrame / (currentSpriteSheet.Bounds().Dx() / g.player.FrameWidth)) * g.player.FrameHeight

		// 	// Create a sub-image that represents the current frame
		frame := currentSpriteSheet.SubImage(image.Rect(sx, sy, sx+g.player.FrameWidth, sy+g.player.FrameHeight)).(*ebiten.Image)

		// Draw the sub-image on the screen
		opts := &ebiten.DrawImageOptions{}
		// If the direction is left, flip the image on the vertical axis
		if g.player.Direction == "left" {
			opts.GeoM.Scale(-1, 1)                               // Flip horizontally
			opts.GeoM.Translate(float64(g.player.FrameWidth), 0) // Adjust the position after flipping
		}

		opts.GeoM.Scale(scale, scale)
		//Draw Character at the center of the screen
		screenWidth := screen.Bounds().Dx()
		screenHeight := screen.Bounds().Dy()
		charWidth := frame.Bounds().Dx()
		charHeight := frame.Bounds().Dy()
		charX := float64(screenWidth)/2 - float64(charWidth)/4
		charY := float64(screenHeight)/2 - float64(charHeight)/4
		opts.GeoM.Translate(charX, charY)
		for _, cnpc := range g.Scenes[g.CurrentScene].NPCs {
			cnpc.Draw(screen, g.player.X, g.player.Y, scale)
		}
		screen.DrawImage(frame, opts)
		screen.DrawImage(g.Scenes[g.CurrentScene].Foreground, bgOpts)
		g.dialogue.Draw(screen, g)

	} else if g.state == TransitionState || g.state == NewSceneState {
		scale := 0.25
		bgOpts := &ebiten.DrawImageOptions{}
		bgOpts.GeoM.Translate(g.player.X, g.player.Y)
		bgOpts.GeoM.Scale(scale, scale)
		screen.DrawImage(g.Scenes[g.CurrentScene].Background, bgOpts)
		currentSpriteSheet := g.player.SpriteSheets[g.player.Direction]
		// 	// Determine the x, y location of the current frame on the sprite sheet
		sx := (g.player.CurrentFrame % (currentSpriteSheet.Bounds().Dx() / g.player.FrameWidth)) * g.player.FrameWidth
		sy := (g.player.CurrentFrame / (currentSpriteSheet.Bounds().Dx() / g.player.FrameWidth)) * g.player.FrameHeight

		// 	// Create a sub-image that represents the current frame
		frame := currentSpriteSheet.SubImage(image.Rect(sx, sy, sx+g.player.FrameWidth, sy+g.player.FrameHeight)).(*ebiten.Image)

		// Draw the sub-image on the screen
		opts := &ebiten.DrawImageOptions{}
		// If the direction is left, flip the image on the vertical axis
		if g.player.Direction == "left" {
			opts.GeoM.Scale(-1, 1)                               // Flip horizontally
			opts.GeoM.Translate(float64(g.player.FrameWidth), 0) // Adjust the position after flipping
		}

		opts.GeoM.Scale(scale, scale)
		//Draw Character at the center of the screen
		screenWidth := screen.Bounds().Dx()
		screenHeight := screen.Bounds().Dy()
		charWidth := frame.Bounds().Dx()
		charHeight := frame.Bounds().Dy()
		charX := float64(screenWidth)/2 - float64(charWidth)/4
		charY := float64(screenHeight)/2 - float64(charHeight)/4
		opts.GeoM.Translate(charX, charY)
		screen.DrawImage(frame, opts)
		screen.DrawImage(g.Scenes[g.CurrentScene].Foreground, bgOpts)

		// Draw the fade rectangle
		fadeImage := ebiten.NewImage(screen.Bounds().Dx(), screen.Bounds().Dy())
		fadeColor := color.RGBA{0, 0, 0, uint8(g.alpha * 0xff)} // Black with variable alpha
		fadeImage.Fill(fadeColor)
		screen.DrawImage(fadeImage, nil)
	} else if g.state == CutsceneState {
		scale := 0.25
		bgOpts := &ebiten.DrawImageOptions{}
		bgOpts.GeoM.Translate(g.player.X, g.player.Y)
		bgOpts.GeoM.Scale(scale, scale)
		screen.DrawImage(g.Scenes[g.CurrentScene].Background, bgOpts)
		currentSpriteSheet := g.player.SpriteSheets[g.player.Direction]
		// 	// Determine the x, y location of the current frame on the sprite sheet
		sx := (g.player.CurrentFrame % (currentSpriteSheet.Bounds().Dx() / g.player.FrameWidth)) * g.player.FrameWidth
		sy := (g.player.CurrentFrame / (currentSpriteSheet.Bounds().Dx() / g.player.FrameWidth)) * g.player.FrameHeight

		// 	// Create a sub-image that represents the current frame
		frame := currentSpriteSheet.SubImage(image.Rect(sx, sy, sx+g.player.FrameWidth, sy+g.player.FrameHeight)).(*ebiten.Image)

		// Draw the sub-image on the screen
		opts := &ebiten.DrawImageOptions{}
		// If the direction is left, flip the image on the vertical axis
		if g.player.Direction == "left" {
			opts.GeoM.Scale(-1, 1)                               // Flip horizontally
			opts.GeoM.Translate(float64(g.player.FrameWidth), 0) // Adjust the position after flipping
		}

		opts.GeoM.Scale(scale, scale)
		//Draw Character at the center of the screen
		screenWidth := screen.Bounds().Dx()
		screenHeight := screen.Bounds().Dy()
		charWidth := frame.Bounds().Dx()
		charHeight := frame.Bounds().Dy()
		charX := float64(screenWidth)/2 - float64(charWidth)/4
		charY := float64(screenHeight)/2 - float64(charHeight)/4
		opts.GeoM.Translate(charX, charY)
		for _, cnpc := range g.Scenes[g.CurrentScene].NPCs {
			cnpc.Draw(screen, g.player.X, g.player.Y, scale)
		}
		screen.DrawImage(frame, opts)
		screen.DrawImage(g.Scenes[g.CurrentScene].Foreground, bgOpts)
		g.dialogue.Draw(screen, g)
		fadeImage := ebiten.NewImage(screen.Bounds().Dx(), screen.Bounds().Dy())
		fadeColor := color.RGBA{0, 0, 0, uint8(g.alpha * 0xff)} // Black with variable alpha
		fadeImage.Fill(fadeColor)
		screen.DrawImage(fadeImage, nil)
	}

}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 320, 240
}

func loadSpriteSheets() map[string]*ebiten.Image {
	// Create a map to hold the sprite sheets
	spriteSheets := make(map[string]*ebiten.Image)

	// List of directions
	directions := []string{"up", "down", "right", "left"}

	// Loop over the directions and load the corresponding sprite sheet
	for _, direction := range directions {
		// Construct the file path for the sprite sheet
		// This assumes you have files named like "playerUp.png", "playerDown.png", etc.
		if direction == "left" {
			spriteSheets["left"] = spriteSheets["right"]
			break
		}
		c := cases.Upper(language.English)
		path := "assets/player" + c.String(direction) + "Black" + ".png"

		// Load the image
		img, _, err := ebitenutil.NewImageFromFile(path)
		if err != nil {
			log.Fatalf("failed to load '%s' sprite sheet: %v", direction, err)
		}

		// Store the loaded image in the map
		spriteSheets[direction] = img
	}

	return spriteSheets
}

func loadBackground(foregroundPath, backgroundPath string) (*ebiten.Image, *ebiten.Image) {
	bgImage, _, err := ebitenutil.NewImageFromFile(backgroundPath)
	if err != nil {
		log.Fatal(err)
	}
	bgImage2, _, err := ebitenutil.NewImageFromFile(foregroundPath)
	if err != nil {
		log.Fatal(err)
	}
	return bgImage, bgImage2
}

func (g *Game) AddObstacle(x1, y1, x2, y2 int) {
	i := image.Rect(x1, y1, x2, y2)
	g.Scenes[g.CurrentScene].obstacles = append(g.Scenes[g.CurrentScene].obstacles, &i)
}
func (g *Game) AddAirTightDiagonalObstacles(startX, startY, width, height, count int) {
	for i := 0; i < count; i++ {
		x1 := startX + (width * i)
		y1 := startY + (height * i)
		x2 := x1 + width
		y2 := y1 + height
		g.AddObstacle(x1, y1, x2, y2)
	}
}

func (g *Game) AddDoor(x1, y1, x2, y2 int, dest, id string, newX, newY float64) {
	i := image.Rect(x1, y1, x2, y2)
	d := &Door{
		Rect:        &i,
		Id:          id,
		Destination: dest,
		NewX:        newX,
		NewY:        newY,
	}
	g.Scenes[g.CurrentScene].doors = append(g.Scenes[g.CurrentScene].doors, d)
}
func (g *Game) AddNPC(spriteSheets map[string]*ebiten.Image, name string) {
	n := &npc.NPC{
		Name:             name,
		X:                -900,
		Y:                -950,
		FrameWidth:       192 / 4, // The width of a single frame
		FrameHeight:      68,      // The height of a single frame
		FrameCount:       4,       // The total number of frames in the sprite sheet
		SpriteSheets:     spriteSheets,
		Direction:        "left", // Default direction
		Speed:            7.0,
		MoveTimer:        60,  // 1 second at 60 FPS
		StopDuration:     120, // stops for 3 seconds
		IsStopped:        true,
		InteractionState: npc.NoInteraction,
		DialogueText:     []string{"Lets go on a trip together! How much dialogue do you need?", "Liten up fella, I really hate doing this, but you kind of smell like rotten eggs took a piss in a toilet."},
	}
	g.Scenes[g.CurrentScene].NPCs = append(g.Scenes[g.CurrentScene].NPCs, n)
}
func newScene(foreground *ebiten.Image, background *ebiten.Image, fn, fn2 func(*Game)) *Scene {
	return &Scene{
		Foreground:    foreground,
		Background:    background,
		loadObsnDoors: fn,
		loadNPCs:      fn2,
	}
}
func (g *Game) loadScenes() {
	m := make(map[string]*Scene)
	bg1, fg1 := loadBackground("assets/mainMap.png", "assets/over.png")
	bg2, fg2 := loadBackground("assets/mainMapRed.png", "assets/overRed.png")
	mainScene := newScene(bg1, fg1, loadObsnDoorss, loadNPCBryan)
	secondScene := newScene(bg2, fg2, loadObsnDoors2, loadNPCBryan)
	mainScene.Game = g
	secondScene.Game = g
	g.Scenes = m

	m["mainMap"] = mainScene
	m["mainMapRed"] = secondScene
}

func (g *Game) changeScene(from string, to string) {
	g.CurrentScene = to
}
func loadNPCBryan(g *Game) {
	if len(g.Scenes[g.CurrentScene].NPCs) == 0 {

		// Create a map to hold the sprite sheets
		spriteSheets := make(map[string]*ebiten.Image)

		// List of directions
		directions := []string{"up", "down", "right", "left"}

		// Loop over the directions and load the corresponding sprite sheet
		for _, direction := range directions {
			// Construct the file path for the sprite sheet
			// This assumes you have files named like "playerUp.png", "playerDown.png", etc.
			if direction == "left" {
				spriteSheets["left"] = spriteSheets["right"]
				break
			}
			c := cases.Upper(language.English)
			path := "assets/player" + c.String(direction) + "Blue" + ".png"

			// Load the image
			img, _, err := ebitenutil.NewImageFromFile(path)
			if err != nil {
				log.Fatalf("failed to load '%s' sprite sheet: %v", direction, err)
			}

			// Store the loaded image in the map
			spriteSheets[direction] = img
		}

		g.AddNPC(spriteSheets, "Bryan")
	}
}
func loadObsnDoorss(g *Game) {
	if len(g.Scenes[g.CurrentScene].obstacles) == 0 {
		g.AddObstacle(2075, 432, 1850, 604) // House Collision
		g.AddObstacle(956, 630, 1430, 855)
		g.AddObstacle(2340, 432, 2550, 604)
		g.AddObstacle(540, 715, 620, 800) // Tree Collision
		g.AddObstacle(580, 520, 660, 610)
		g.AddObstacle(680, 950, 755, 1040)
		g.AddObstacle(920, 430, 1000, 515)
		g.AddObstacle(1495, 430, 1575, 515)
		g.AddObstacle(875, 665, 955, 745)
		g.AddObstacle(1160, 815, 1235, 900)
		g.AddObstacle(1210, 485, 1290, 565)
		g.AddObstacle(1680, 485, 1765, 565)
		g.AddObstacle(1495, 435, 1575, 515)
		g.AddObstacle(1450, 670, 1530, 750)
		g.AddObstacle(2260, 465, 2350, 550)
		g.AddObstacle(2555, 465, 2640, 550)
		g.AddObstacle(225, 680, 270, 770)
		g.AddAirTightDiagonalObstacles(275, 780, 30, 30, 10)
		g.AddAirTightDiagonalObstacles(615, 1070, 30, 30, 7)
		g.AddAirTightDiagonalObstacles(250, 665, 30, -30, 10)
		g.AddAirTightDiagonalObstacles(2400, 1270, 30, -30, 13)
		g.AddAirTightDiagonalObstacles(2515, 355, 30, 30, 10)
		g.AddObstacle(815, 1240, 2410, 1295) //Land boundary Collision
		g.AddObstacle(575, 305, 2520, 335)
		g.AddObstacle(975, 1060, 2520, 1100) //Fence Collision
		g.AddObstacle(1415, 930, 1895, 955)
		g.AddObstacle(2030, 930, 2380, 955)
		g.AddObstacle(2830, 645, 3060, 670) // Port Collisions
		g.AddObstacle(2835, 815, 3060, 835)
		g.AddObstacle(3060, 670, 3085, 815)
		g.AddObstacle(2170, 705, 2300, 850)                           // Pond Collisions
		g.AddDoor(1000, 840, 1095, 945, "mainMapRed", "fd", -140, 20) // Door Collisions
		g.AddDoor(1290, 840, 1390, 945, "mainMapRed", "sd", -1000, -1000)
		g.AddDoor(1915, 600, 2015, 710, "mainMapRed", "td", -1500, -1500)
		g.AddDoor(2400, 600, 2495, 710, "mainMapRed", "ffd", -700, -700)
	}

}
func loadObsnDoors2(g *Game) {
	if len(g.Scenes[g.CurrentScene].obstacles) == 0 {
		g.AddObstacle(2075, 432, 1850, 604) // House Collision
		g.AddObstacle(956, 630, 1430, 855)
		g.AddObstacle(2340, 432, 2550, 604)
		g.AddObstacle(540, 715, 620, 800) // Tree Collision
		g.AddObstacle(580, 520, 660, 610)
		g.AddObstacle(680, 950, 755, 1040)
		g.AddObstacle(920, 430, 1000, 515)
		g.AddObstacle(1495, 430, 1575, 515)
		g.AddObstacle(875, 665, 955, 745)
		g.AddObstacle(1160, 815, 1235, 900)
		g.AddObstacle(1210, 485, 1290, 565)
		g.AddObstacle(1680, 485, 1765, 565)
		g.AddObstacle(1495, 435, 1575, 515)
		g.AddObstacle(1450, 670, 1530, 750)
		g.AddObstacle(2260, 465, 2350, 550)
		g.AddObstacle(2555, 465, 2640, 550)
		g.AddObstacle(225, 680, 270, 770)
		g.AddAirTightDiagonalObstacles(275, 780, 30, 30, 10)
		g.AddAirTightDiagonalObstacles(615, 1070, 30, 30, 7)
		g.AddAirTightDiagonalObstacles(250, 665, 30, -30, 10)
		g.AddAirTightDiagonalObstacles(2400, 1270, 30, -30, 13)
		g.AddAirTightDiagonalObstacles(2515, 355, 30, 30, 10)
		g.AddObstacle(815, 1240, 2410, 1295) //Land boundary Collision
		g.AddObstacle(575, 305, 2520, 335)
		g.AddObstacle(975, 1060, 2520, 1100) //Fence Collision
		g.AddObstacle(1415, 930, 1895, 955)
		g.AddObstacle(2030, 930, 2380, 955)
		g.AddObstacle(2830, 645, 3060, 670) // Port Collisions
		g.AddObstacle(2835, 815, 3060, 835)
		g.AddObstacle(3060, 670, 3085, 815)
		g.AddObstacle(2170, 705, 2300, 850)                          // Pond Collisions
		g.AddDoor(1000, 840, 1095, 945, "mainMap", "fd", -500, -500) // Door Collisions
		g.AddDoor(1290, 840, 1390, 945, "mainMap", "sd", -1000, -1000)
		g.AddDoor(1915, 600, 2015, 710, "mainMap", "td", -1500, -1500)
		g.AddDoor(2400, 600, 2495, 710, "mainMap", "ffd", -700, -700)
	}

}
func wrapText(text string, maxWidth int, face font.Face) string {
	var wrapped string
	var lineWidth fixed.Int26_6
	spaceWidth := font.MeasureString(face, " ")

	for _, word := range strings.Fields(text) {
		wordWidth := font.MeasureString(face, word)

		// If adding the new word exceeds the max width, then insert a new line
		if lineWidth > 0 && lineWidth+wordWidth+spaceWidth > fixed.I(maxWidth) {
			wrapped += "\n"
			lineWidth = 0
		}

		if lineWidth > 0 {
			wrapped += " "
			lineWidth += spaceWidth
		}

		wrapped += word
		lineWidth += wordWidth
	}

	return wrapped
}
func NewGame() *Game {
	// Load the sprite sheet
	spriteSheets := loadSpriteSheets()
	f, err := loadFontFace()
	if err != nil {
		log.Fatal(err)
	}
	// Create an instance of the Game struct
	g := &Game{
		state:          PlayState,
		fface:          f,
		menuOptions:    []string{"Start Game", "Options", "Exit"},
		selectedOption: 0,
		alpha:          0.0,
		fadeSpeed:      0.05,
		player: &player.Player{
			X:            0,
			Y:            0,
			FrameWidth:   192 / 4, // The width of a single frame
			FrameHeight:  68,      // The height of a single frame
			FrameCount:   4,       // The total number of frames in the sprite sheet
			SpriteSheets: spriteSheets,
			Direction:    "down", // Default direction
			Speed:        7.0,
			CanMove:      true,
		},
	}
	g.loadScenes()
	g.CurrentScene = "mainMap"
	g.Scenes[g.CurrentScene].loadObsnDoors(g)
	g.Scenes[g.CurrentScene].loadNPCs(g)
	g.dialogue = newDialogue()
	// g.AddObstacle(0, 0, 300, 300)       // Debug collision box

	// g.AddObstacle()
	// g.AddObstacle()
	return g
}
func newDialogue() *Dialogue {
	d := &Dialogue{
		TextLines:     []string{},
		FramesPerChar: 2, // For example, one character every 2 frames
		IsOpen:        false,
		CurrentLine:   0,
		CharIndex:     0,
	}
	return d

}
func main() {
	var game *Game
	if savedStateExists("savefile.json") {
		gameState, err := LoadGameState("savefile.json")
		if err != nil {
			log.Fatalf("Failed to load saved game: %v", err)
		}
		f, err := loadFontFace()
		spriteSheets := loadSpriteSheets()
		if err != nil {
			log.Fatal(err)
		}
		game =
			&Game{
				CurrentScene:   gameState.CurrentScene,
				Progress:       gameState.GameProgress,
				state:          PlayState,
				fface:          f,
				menuOptions:    []string{"Start Game", "Options", "Exit"},
				selectedOption: 0,
				alpha:          0.0,
				fadeSpeed:      0.05,
				player: &player.Player{
					X:            gameState.PlayerPosition.X,
					Y:            gameState.PlayerPosition.Y,
					FrameWidth:   192 / 4, // The width of a single frame
					FrameHeight:  68,      // The height of a single frame
					FrameCount:   4,       // The total number of frames in the sprite sheet
					SpriteSheets: spriteSheets,
					Direction:    gameState.PlayerDirection, // Default direction
					Speed:        7.0,
					CanMove:      true,
				},
			}
		game.loadScenes()
		game.Scenes[game.CurrentScene].loadObsnDoors(game)
		game.Scenes[game.CurrentScene].loadNPCs(game)
		game.dialogue = newDialogue()
	} else {
		game = NewGame()
	}

	// Configuration settings
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Sprite Animation")

	// Start the game
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

func savedStateExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}
