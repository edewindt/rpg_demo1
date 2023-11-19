package main

import (
	"ebi/npc"
	"ebi/player"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type GameState int

const (
	MenuState GameState = iota
	PlayState
	OptionsState
	TransitionState
	NewSceneState
)

type Door struct {
	Rect        *image.Rectangle
	Id          string
	Destination *Scene
	NewX, NewY  float64
}

type Scene struct {
	Game                   *Game
	obstacles              []*image.Rectangle
	doors                  []*Door
	background, Foreground *ebiten.Image
	loadObsnDoors          func(*Game)
	loadNPCs               func(*Game)
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
	Scenes                  []*Scene
	CurrentScene, NextScene *Scene
	CurrentDoor             *Door
	selectedOption          int
	keyPressCounter         map[ebiten.Key]int // Tracks duration of key presses
	keyKPressedLastFrame    bool
	keyZPressedLastFrame    bool
	// keyRPressedLastFrame    bool
	dialogue *Dialogue
	fface    font.Face
}

func (g *Game) Update() error {
	if g.keyPressCounter == nil {
		g.keyPressCounter = make(map[ebiten.Key]int)
	}
	if g.state == OptionsState {
		os.Exit(0)
	}
	if g.state == MenuState {
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
		for _, cnpc := range g.CurrentScene.NPCs {
			if nearNPC(minX(g.player.X, g), minY(g.player.Y, g), cnpc.X, cnpc.Y) {
				if ebiten.IsKeyPressed(ebiten.KeyZ) && !g.keyZPressedLastFrame {
					// Toggle NPC interaction state
					if cnpc.InteractionState == npc.NoInteraction {
						cnpc.InteractionState = npc.PlayerInteracted
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

		g.keyZPressedLastFrame = ebiten.IsKeyPressed(ebiten.KeyZ)
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

		for _, obstacle := range g.CurrentScene.obstacles {
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
		for _, door := range g.CurrentScene.doors {
			obsMinX := float64(door.Rect.Min.X)
			obsMaxX := float64(door.Rect.Max.X)
			obsMinY := float64(door.Rect.Min.Y)
			obsMaxY := float64(door.Rect.Max.Y)
			if obsMinX < minX(moveX, g) && obsMaxX > maxX(moveX, g) && obsMinY < minY(moveY, g) && obsMaxY > maxY(moveY, g) {
				if colliding {
					g.state = TransitionState
					g.CurrentDoor = door
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
			if g.CurrentScene == g.Scenes[0] {
				g.changeScene(g.Scenes[0], g.Scenes[1])
			} else {
				g.changeScene(g.Scenes[1], g.Scenes[0])
			}

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
			g.CurrentScene.loadObsnDoors(g)
		}
	} else if g.state == NewSceneState {
		// Decrease the alpha for the fade in effect
		g.alpha -= g.fadeSpeed
		if g.alpha <= 0.0 {
			g.alpha = 0.0
			g.state = PlayState
			// The new scene is fully visible now, and game continues as normal
		}
	}
	return nil
}
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
		screen.DrawImage(g.CurrentScene.background, bgOpts)
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
		for _, cnpc := range g.CurrentScene.NPCs {
			cnpc.Draw(screen, g.player.X, g.player.Y)
		}
		screen.DrawImage(frame, opts)
		screen.DrawImage(g.CurrentScene.Foreground, bgOpts)
		g.dialogue.Draw(screen, g)

	} else if g.state == TransitionState || g.state == NewSceneState {
		scale := 0.25
		bgOpts := &ebiten.DrawImageOptions{}
		bgOpts.GeoM.Translate(g.player.X, g.player.Y)
		bgOpts.GeoM.Scale(scale, scale)
		screen.DrawImage(g.CurrentScene.background, bgOpts)
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
		screen.DrawImage(g.CurrentScene.Foreground, bgOpts)

		// Draw the fade rectangle
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
	g.CurrentScene.obstacles = append(g.CurrentScene.obstacles, &i)
}
func (g *Game) AddDoor(x1, y1, x2, y2 int, dest *Scene, id string, newX, newY float64) {
	i := image.Rect(x1, y1, x2, y2)
	d := &Door{
		Rect:        &i,
		Id:          id,
		Destination: dest,
		NewX:        newX,
		NewY:        newY,
	}
	g.CurrentScene.doors = append(g.CurrentScene.doors, d)
}
func (g *Game) AddNPC(spriteSheets map[string]*ebiten.Image) {
	n := &npc.NPC{
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
	}
	g.CurrentScene.NPCs = append(g.CurrentScene.NPCs, n)
}
func newScene(foreground *ebiten.Image, background *ebiten.Image, fn, fn2 func(*Game)) *Scene {
	return &Scene{
		Foreground:    foreground,
		background:    background,
		loadObsnDoors: fn,
		loadNPCs:      fn2,
	}
}
func (g *Game) loadScenes() {
	bg1, fg1 := loadBackground("assets/mainMap.png", "assets/over.png")
	bg2, fg2 := loadBackground("assets/mainMapRed.png", "assets/overRed.png")
	mainScene := newScene(bg1, fg1, loadObsnDoorss, loadNPCBryan)
	secondScene := newScene(bg2, fg2, loadObsnDoors2, loadNPCBryan)
	mainScene.Game = g
	secondScene.Game = g
	g.Scenes = append(g.Scenes, mainScene)
	g.Scenes = append(g.Scenes, secondScene)
	g.CurrentScene = mainScene
}

func (g *Game) changeScene(from *Scene, to *Scene) {
	g.CurrentScene = to
}
func loadNPCBryan(g *Game) {
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

	g.AddNPC(spriteSheets)
}
func loadObsnDoorss(g *Game) {
	if len(g.CurrentScene.obstacles) == 0 {
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
		g.AddObstacle(815, 1240, 2410, 1295) //Land boundary Collision
		g.AddObstacle(575, 305, 2520, 335)
		g.AddObstacle(975, 1060, 2520, 1100) //Fence Collision
		g.AddObstacle(1415, 930, 1895, 955)
		g.AddObstacle(2030, 930, 2380, 955)
		g.AddObstacle(2830, 645, 3060, 670) // Port Collisions
		g.AddObstacle(2835, 815, 3060, 835)
		g.AddObstacle(3060, 670, 3085, 815)
		g.AddObstacle(2170, 705, 2300, 850)                          // Pond Collisions
		g.AddDoor(1000, 840, 1095, 945, g.Scenes[1], "fd", -140, 20) // Door Collisions
		g.AddDoor(1290, 840, 1390, 945, g.Scenes[1], "sd", -1000, -1000)
		g.AddDoor(1915, 600, 2015, 710, g.Scenes[1], "td", -1500, -1500)
		g.AddDoor(2400, 600, 2495, 710, g.Scenes[1], "ffd", -700, -700)
	}

}
func loadObsnDoors2(g *Game) {
	if len(g.CurrentScene.obstacles) == 0 {
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
		g.AddObstacle(815, 1240, 2410, 1295) //Land boundary Collision
		g.AddObstacle(575, 305, 2520, 335)
		g.AddObstacle(975, 1060, 2520, 1100) //Fence Collision
		g.AddObstacle(1415, 930, 1895, 955)
		g.AddObstacle(2030, 930, 2380, 955)
		g.AddObstacle(2830, 645, 3060, 670) // Port Collisions
		g.AddObstacle(2835, 815, 3060, 835)
		g.AddObstacle(3060, 670, 3085, 815)
		g.AddObstacle(2170, 705, 2300, 850)                            // Pond Collisions
		g.AddDoor(1000, 840, 1095, 945, g.Scenes[0], "fd", -500, -500) // Door Collisions
		g.AddDoor(1290, 840, 1390, 945, g.Scenes[0], "sd", -1000, -1000)
		g.AddDoor(1915, 600, 2015, 710, g.Scenes[0], "td", -1500, -1500)
		g.AddDoor(2400, 600, 2495, 710, g.Scenes[0], "ffd", -700, -700)
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
	g.CurrentScene.loadObsnDoors(g)
	g.CurrentScene.loadNPCs(g)
	g.dialogue = newDialogue()
	// g.AddObstacle(0, 0, 300, 300)       // Debug collision box

	// g.AddObstacle()
	// g.AddObstacle()
	return g
}
func newDialogue() *Dialogue {
	d := &Dialogue{
		TextLines:     []string{"Lets go on a trip together! How much dialogue do you need?", "Liten up fella, I really hate doing this, but you kind of smell like rotten eggs took a piss in a toilet."},
		FramesPerChar: 2, // For example, one character every 2 frames
		IsOpen:        false,
		CurrentLine:   0,
		CharIndex:     0,
	}
	return d

}
func main() {
	game := NewGame()
	// Configuration settings
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Sprite Animation")

	// Start the game
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
