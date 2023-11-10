package main

import (
	"image"
	"log"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type Game struct {
	frameWidth   int
	frameHeight  int
	frameCount   int
	currentFrame int
	tickCount    int // Counter to track the number of updates
	x, y         float64
	spriteSheets map[string]*ebiten.Image // Map of sprite sheets for each direction
	direction    string
}

func (g *Game) Update() error {

	if ebiten.IsKeyPressed(ebiten.KeyLeft) {
		g.x -= 2 // Move left
		g.direction = "left"
	}
	if ebiten.IsKeyPressed(ebiten.KeyRight) {
		g.x += 2 // Move right
		g.direction = "right"
	}
	if ebiten.IsKeyPressed(ebiten.KeyUp) {
		g.y -= 2 // Move up
		g.direction = "up"
	}
	if ebiten.IsKeyPressed(ebiten.KeyDown) {
		g.y += 2 // Move down
		g.direction = "down"
	}
	// Increment the tick count
	g.tickCount++

	// Update the current frame every 10 ticks
	if g.tickCount >= 10 {
		g.currentFrame = (g.currentFrame + 1) % g.frameCount
		g.tickCount = 0 // Reset the tick count
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
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
		spriteSheets: spriteSheets,
		frameWidth:   192 / 4, // The width of a single frame
		frameHeight:  68,      // The height of a single frame
		frameCount:   4,       // The total number of frames in the sprite sheet
		direction:    "down",  // Default direction
	}

	// Configuration settings
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Sprite Animation")

	// Start the game
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
