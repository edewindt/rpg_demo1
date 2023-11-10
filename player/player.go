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
}

func (p *Player) Move(dir string, speed float64) {
	p.Direction = dir

	switch dir {
	case "left":
		p.X -= speed // Move left
	case "right":
		p.X += speed // Move right
	case "up":
		p.Y -= speed // Move up
	case "down":
		p.Y += speed // Move down
	}

}

func (p *Player) GetDirection() string {
	return p.Direction
}
