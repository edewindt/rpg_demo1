package player

import "github.com/hajimehoshi/ebiten/v2"

type Player struct {
	FrameWidth   int
	FrameHeight  int
	FrameCount   int
	CurrentFrame int
	TickCount    int // Counter to track the number of updates
	X, Y         float64
	SpriteSheets map[string]*ebiten.Image // Map of sprite sheets for each direction
	Direction    string
	Speed        float64
}

func (p Player) CheckMove(dir string) (float64, float64) {

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
