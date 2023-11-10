package main

import (
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
)

type Game struct {
	frameWidth      int
	frameHeight     int
	frameCount      int
	currentFrame    int
	tickCount       int // Counter to track the number of updates
	x, y            float64
	spriteSheets    map[string]*ebiten.Image // Map of sprite sheets for each direction
	direction       string
	state           GameState
	menuOptions     []string
	selectedOption  int
	keyPressCounter map[ebiten.Key]int // Tracks duration of key presses
}

func (g *Game) Update() error {
	if g.keyPressCounter == nil {
		g.keyPressCounter = make(map[ebiten.Key]int)
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
				os.Exit(0)
			case 2: // Exit
				os.Exit(0)
			}
		}
	} else if g.state == PlayState {
		speed := 2.0
		movementKeyPressed := false

		if ebiten.IsKeyPressed(ebiten.KeyLeft) {
			g.x -= speed // Move left
			g.direction = "left"
			movementKeyPressed = true
		}
		if ebiten.IsKeyPressed(ebiten.KeyRight) {
			g.x += speed // Move right
			g.direction = "right"
			movementKeyPressed = true
		}
		if ebiten.IsKeyPressed(ebiten.KeyUp) {
			g.y -= speed // Move up
			g.direction = "up"
			movementKeyPressed = true
		}
		if ebiten.IsKeyPressed(ebiten.KeyDown) {
			g.y += speed // Move down
			g.direction = "down"
			movementKeyPressed = true
		}
		if movementKeyPressed {
			// Increment the tick count
			g.tickCount++
		}

		// Update the current frame every 10 ticks
		if g.tickCount >= 10 {
			g.currentFrame = (g.currentFrame + 1) % g.frameCount
			g.tickCount = 0 // Reset the tick count
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
		currentSpriteSheet := g.spriteSheets[g.direction]
		// Determine the x, y location of the current frame on the sprite sheet
		sx := (g.currentFrame % (currentSpriteSheet.Bounds().Dx() / g.frameWidth)) * g.frameWidth
		sy := (g.currentFrame / (currentSpriteSheet.Bounds().Dx() / g.frameWidth)) * g.frameHeight

		// Create a sub-image that represents the current frame
		frame := currentSpriteSheet.SubImage(image.Rect(sx, sy, sx+g.frameWidth, sy+g.frameHeight)).(*ebiten.Image)

		// Draw the sub-image on the screen
		opts := &ebiten.DrawImageOptions{}
		// If the direction is left, flip the image on the vertical axis
		if g.direction == "left" {
			opts.GeoM.Scale(-1, 1)                        // Flip horizontally
			opts.GeoM.Translate(float64(g.frameWidth), 0) // Adjust the position after flipping
		}
		opts.GeoM.Translate(g.x, g.y)
		screen.DrawImage(frame, opts)
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
		path := "assets/player" + strings.Title(direction) + ".png"

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
func main() {
	// Load the sprite sheet
	spriteSheets := loadSpriteSheets()

	// Create an instance of the Game struct
	game := &Game{
		state:          MenuState,
		menuOptions:    []string{"Start Game", "Options", "Exit"},
		selectedOption: 0,
		spriteSheets:   spriteSheets,
		frameWidth:     192 / 4, // The width of a single frame
		frameHeight:    68,      // The height of a single frame
		frameCount:     4,       // The total number of frames in the sprite sheet
		direction:      "down",  // Default direction
	}

	// Configuration settings
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Sprite Animation")

	// Start the game
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
