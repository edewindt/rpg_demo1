package main

import (
	"image"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type Game struct {
	spriteSheet  *ebiten.Image
	frameWidth   int
	frameHeight  int
	frameCount   int
	currentFrame int
	tickCount    int // Counter to track the number of updates
	x, y         float64
}

func (g *Game) Update() error {

	if ebiten.IsKeyPressed(ebiten.KeyLeft) {
		g.x -= 2 // Move left
	}
	if ebiten.IsKeyPressed(ebiten.KeyRight) {
		g.x += 2 // Move right
	}
	if ebiten.IsKeyPressed(ebiten.KeyUp) {
		g.y -= 2 // Move up
	}
	if ebiten.IsKeyPressed(ebiten.KeyDown) {
		g.y += 2 // Move down
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
	// Determine the x, y location of the current frame on the sprite sheet
	sx := (g.currentFrame % (g.spriteSheet.Bounds().Dx() / g.frameWidth)) * g.frameWidth
	sy := (g.currentFrame / (g.spriteSheet.Bounds().Dx() / g.frameWidth)) * g.frameHeight

	// Create a sub-image that represents the current frame
	frame := g.spriteSheet.SubImage(image.Rect(sx, sy, sx+g.frameWidth, sy+g.frameHeight)).(*ebiten.Image)

	// Draw the sub-image on the screen
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(g.x, g.y)
	screen.DrawImage(frame, opts)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 320, 240
}

func main() {
	// Load the sprite sheet
	spriteSheet, _, err := ebitenutil.NewImageFromFile("assets/playerDown.png")
	if err != nil {
		log.Fatal(err)
	}

	// Create an instance of the Game struct
	game := &Game{
		spriteSheet: spriteSheet,
		frameWidth:  16, // The width of a single frame
		frameHeight: 16, // The height of a single frame
		frameCount:  4,  // The total number of frames in the sprite sheet
	}

	// Configuration settings
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Sprite Animation")

	// Start the game
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
