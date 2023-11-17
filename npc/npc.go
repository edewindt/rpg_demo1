package npc

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

type InteractionState int

const (
	NoInteraction InteractionState = iota
	PlayerInteracted
	WaitingForPlayerToResume
)

type NPC struct {
	FrameWidth       int
	FrameHeight      int
	FrameCount       int
	CurrentFrame     int
	MoveTimer        int
	StopTimer        int
	IsStopped        bool
	StopDuration     int
	TickCount        int // Counter to track the number of updates
	X, Y             float64
	SpriteSheets     map[string]*ebiten.Image // Map of sprite sheets for each direction
	Direction        string
	Speed            float64
	InteractionState InteractionState
}

func (npc *NPC) Move(dir string) {

	switch dir {
	case "left":
		npc.X += npc.Speed // Move left
	case "right":
		npc.X -= npc.Speed // Move right
	case "up":
		npc.Y += npc.Speed // Move up
	case "down":
		npc.Y -= npc.Speed // Move down
	}

}

func (npc *NPC) Update(interactionKey ebiten.Key) {
	// Check for interaction key press to change the NPC's state
	if ebiten.IsKeyPressed(interactionKey) {
		if npc.InteractionState == PlayerInteracted {
			npc.InteractionState = WaitingForPlayerToResume
		} else if npc.InteractionState == WaitingForPlayerToResume {
			// Allow the NPC to move again
			npc.InteractionState = NoInteraction
		}
	}

	// NPC movement logic
	if npc.InteractionState == NoInteraction {
		if npc.IsStopped {
			// NPC is stopped, so we might count down the stop timer
			npc.StopTimer--
			if npc.StopTimer <= 0 {
				// Time to move again
				npc.IsStopped = false
				// Reset the move timer to some value
				npc.MoveTimer = 60
				// Change direction
				if npc.Direction == "right" {
					npc.Direction = "left"
				} else {
					npc.Direction = "right"
				}
			}
		} else {
			npc.MoveTimer--
			npc.Move(npc.Direction)
			if npc.MoveTimer <= 0 {
				// Time to stop
				npc.IsStopped = true
				// Reset the stop timer to the duration of the stop
				npc.StopTimer = npc.StopDuration

			}
		}
	} else {
		// NPC is stopped and waiting for player to resume
	}
}

func (npc *NPC) Draw(screen *ebiten.Image, pX, pY float64) {
	scale := 0.25
	currentSpriteSheet := npc.SpriteSheets[npc.Direction]

	// 	// Determine the x, y location of the current frame on the sprite sheet
	sx := (npc.CurrentFrame % (currentSpriteSheet.Bounds().Dx() / npc.FrameWidth)) * npc.FrameWidth
	sy := (npc.CurrentFrame / (currentSpriteSheet.Bounds().Dx() / npc.FrameWidth)) * npc.FrameHeight

	// 	// Create a sub-image that represents the current frame
	frame := currentSpriteSheet.SubImage(image.Rect(sx, sy, sx+npc.FrameWidth, sy+npc.FrameHeight)).(*ebiten.Image)

	// Draw the sub-image on the screen
	opts := &ebiten.DrawImageOptions{}
	// If the direction is left, flip the image on the vertical axis
	if npc.Direction == "left" {
		opts.GeoM.Scale(-1, 1)                          // Flip horizontally
		opts.GeoM.Translate(float64(npc.FrameWidth), 0) // Adjust the position after flipping
	}
	fX := pX - npc.X
	fY := pY - npc.Y
	opts.GeoM.Translate(fX, fY)
	opts.GeoM.Scale(scale, scale)

	screen.DrawImage(frame, opts)
}
