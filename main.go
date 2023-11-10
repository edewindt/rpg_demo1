package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type Game struct {
	image *ebiten.Image
}

func (g *Game) Update() error {
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	opts := &ebiten.DrawImageOptions{}
	screen.DrawImage(g.image, opts)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 320, 240
}

func main() {
	img, _, err := ebitenutil.NewImageFromFile("assets/playerDown.png")
	if err != nil {
		log.Fatal(err)
	}

	// Create an instance of your game
	game := &Game{
		image: img,
	}

	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("My Game")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
