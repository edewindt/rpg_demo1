package player

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

type Player struct {
	FrameWidth        int
	FrameHeight       int
	FrameCount        int
	CurrentFrame      int
	TickCount         int // Counter to track the number of updates
	X, Y              float64
	SpriteSheets      map[string]*ebiten.Image // Map of sprite sheets for each direction
	Direction         string
	Speed             float64
	CanMove           bool
	GhostMode         bool
	GhostModeMeter    float64 // Time remaining in ghost mode
	GhostModeCooldown float64 // Cooldown time before it can be activated again

	KeyBeingPressed  bool // Whether the key is currently being pressed
	IsRunning        bool
	TickCounter      uint64 // Manually track the tick count
	LastKeyPressTick uint64 // Tick count of the last key press
}

func (p Player) CheckMove(dir string) (float64, float64) {
	if p.IsRunning {
		p.Speed = p.Speed * 2
	}
	switch dir {
	case "left":
		p.X += p.Speed // Move left
	case "right":
		p.X -= p.Speed // Move right
	case "up":
		p.Y += p.Speed // Move up
	case "down":
		p.Y -= p.Speed // Move down
	}
	return p.X, p.Y

}

func (p *Player) DrawGhostModeMeter(screen *ebiten.Image) {
	// Define meter dimensions and position
	const meterWidth = 300 // Adjust as needed
	const meterHeight = 20 // Adjust as needed
	const meterX = 50      // Position of the meter on screen
	const meterY = 50

	// Calculate the width of the filled part based on the current meter value

	var filledWidth float64
	if p.GhostModeMeter > 0 {
		fmt.Println(p.GhostModeMeter)
		filledWidth = (p.GhostModeMeter / 600) * meterWidth
	} else {
		fmt.Println("reached")
		filledWidth = ((600 - p.GhostModeCooldown) / 600) * meterWidth
	}
	if filledWidth < 1 {
		fmt.Println("Wow")
		filledWidth = 1
	}
	// Create a background bar (empty meter)
	emptyBar := ebiten.NewImage(meterWidth, meterHeight)
	emptyBar.Fill(color.RGBA{R: 0, G: 0, B: 0, A: 255}) // Grey color

	// Create a filled bar (current meter value)
	filledBar := ebiten.NewImage(int(filledWidth), meterHeight)
	if filledWidth > 150 {
		filledBar.Fill(color.RGBA{R: 0, G: 255, B: 0, A: 255}) // Green color
	} else if filledWidth > 75 {
		filledBar.Fill(color.RGBA{R: 255, G: 255, B: 0, A: 255}) // Yellow Color
	} else {
		filledBar.Fill(color.RGBA{R: 255, G: 0, B: 0, A: 255}) // Red Color
	}

	// Draw the empty bar
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(meterX, meterY)
	opts.GeoM.Scale(0.25, 0.25)
	screen.DrawImage(emptyBar, opts)

	// Draw the filled bar on top of the empty bar
	screen.DrawImage(filledBar, opts)
}
