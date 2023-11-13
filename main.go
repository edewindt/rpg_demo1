package main

import (
	"ebi/player"
	"image"
	"image/color"
	"log"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

type GameState int

const (
	MenuState GameState = iota
	PlayState
	OptionsState
)

type Game struct {
	player          *player.Player
	background      *ebiten.Image
	Foreground      *ebiten.Image
	state           GameState
	menuOptions     []string
	selectedOption  int
	keyPressCounter map[ebiten.Key]int // Tracks duration of key presses
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
		// // Navigate through menu options
		// if ebiten.IsKeyPressed(ebiten.KeyDown) && g.selectedOption < len(g.menuOptions)-1 {
		// 	g.selectedOption++
		// } else if ebiten.IsKeyPressed(ebiten.KeyUp) && g.selectedOption > 0 {
		// 	g.selectedOption--
		// }

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
		movementKeyPressed := false

		// if ebiten.IsKeyPressed(ebiten.KeyLeft) {

		// 	g.player.Move("left", speed)
		// 	movementKeyPressed = true
		// }
		// if ebiten.IsKeyPressed(ebiten.KeyRight) {
		// 	g.player.Move("right", speed)
		// 	movementKeyPressed = true
		// }
		// if ebiten.IsKeyPressed(ebiten.KeyUp) {
		// 	g.player.Move("up", speed)
		// 	movementKeyPressed = true
		// }
		// if ebiten.IsKeyPressed(ebiten.KeyDown) {
		// 	g.player.Move("down", speed)
		// 	movementKeyPressed = true
		// }
		if ebiten.IsKeyPressed(ebiten.KeyLeft) {
			g.player.Move("left")
			movementKeyPressed = true
		}
		if ebiten.IsKeyPressed(ebiten.KeyRight) {
			g.player.Move("right")
			movementKeyPressed = true
		}
		if ebiten.IsKeyPressed(ebiten.KeyUp) {
			g.player.Move("up")
			movementKeyPressed = true
		}
		if ebiten.IsKeyPressed(ebiten.KeyDown) {
			g.player.Move("down")
			movementKeyPressed = true
		}
		if movementKeyPressed {
			// Increment the tick count
			g.player.TickCount++
		}

		// Update the current frame every 10 ticks
		if g.player.TickCount >= 10 {
			g.player.CurrentFrame = (g.player.CurrentFrame + 1) % g.player.FrameCount
			g.player.TickCount = 0 // Reset the tick count
		}
	}

	return nil
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

func (g *Game) Draw(screen *ebiten.Image) {
	if g.state == MenuState {
		fontFace, err := loadFontFace()
		if err != nil {
			log.Fatal(err)
		}
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
		bgOpts := &ebiten.DrawImageOptions{}
		bgOpts.GeoM.Translate(g.player.X, g.player.Y)
		bgScale := 0.5
		bgOpts.GeoM.Scale(bgScale, bgScale)
		screen.DrawImage(g.background, bgOpts)
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
		scale := 0.5
		opts.GeoM.Scale(scale, scale)
		//Draw Character at the center of the screen
		screenWidth := screen.Bounds().Dx()
		screenHeight := screen.Bounds().Dy()
		charWidth := frame.Bounds().Dx()
		charHeight := frame.Bounds().Dy()
		charX := float64(screenWidth)/2 - float64(charWidth)/2
		charY := float64(screenHeight)/2 - float64(charHeight)/2
		opts.GeoM.Translate(charX, charY)
		screen.DrawImage(frame, opts)
		screen.DrawImage(g.Foreground, bgOpts)
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
		path := "assets/player" + strings.Title(direction) + "Black" + ".png"

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
func loadBackground() (*ebiten.Image, *ebiten.Image) {
	bgImage, _, err := ebitenutil.NewImageFromFile("assets/myMap.png")
	if err != nil {
		log.Fatal(err)
	}
	bgImage2, _, err := ebitenutil.NewImageFromFile("assets/over.png")
	if err != nil {
		log.Fatal(err)
	}
	return bgImage, bgImage2
}
func main() {
	// Load the sprite sheet
	spriteSheets := loadSpriteSheets()
	background, Foreground := loadBackground()
	// Create an instance of the Game struct
	game := &Game{
		state:          MenuState,
		menuOptions:    []string{"Start Game", "Options", "Exit"},
		selectedOption: 0,
		background:     background,
		Foreground:     Foreground,
		player: &player.Player{
			X:            -500,
			Y:            -500,
			FrameWidth:   192 / 4, // The width of a single frame
			FrameHeight:  68,      // The height of a single frame
			FrameCount:   4,       // The total number of frames in the sprite sheet // Default direction
			SpriteSheets: spriteSheets,
			Direction:    "down",
		},
		// spriteSheets:   spriteSheets,
		// FrameWidth:     192 / 4, // The width of a single frame
		// FrameHeight:    68,      // The height of a single frame
		// FrameCount:     4,       // The total number of frames in the sprite sheet // Default direction
	}

	// Configuration settings
	ebiten.SetWindowSize(640*2, 480*2)
	ebiten.SetWindowTitle("Sprite Animation")

	// Start the game
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
