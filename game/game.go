package game

import (
	"encoding/json"
	"github.com/qaisjp/studenthackv-go-gameserver/mapgen"
	// . "github.com/qaisjp/studenthackv-go-gameserver/structs"
	"log"
	// "math/rand"
	"math"
	"time"
)

type Game struct {
	players map[*Player]bool

	Monster  *Player
	King     *Player
	Servants []*Player
	Map      *mapgen.Map

	// Inbound messages from players.
	broadcast chan MessageIn

	// Register requests from players.
	register chan *Player

	// Unregister requests from players.
	unregister chan *Player

	alive bool
}

func NewGame() *Game {
	log.Println("New game created")

	g := &Game{
		alive: true,

		broadcast:  make(chan MessageIn),
		register:   make(chan *Player),
		unregister: make(chan *Player),
		players:    make(map[*Player]bool),
		Map:        mapgen.NewMap(99, 99), // must be odd row/column
	}

	return g
}

func (g *Game) IsAlive() bool {
	return g.alive
}

func (g *Game) Run() {
	c := time.Tick(500 * time.Millisecond)

	for {
		select {
		case player := <-g.register:
			g.onPlayerConnect(player)
		case player := <-g.unregister:
			if _, ok := g.players[player]; ok {
				player.onLeave()

				delete(g.players, player)
				close(player.send)

				if player.Character == MonsterCharacter {
					g.Monster = nil
				} else if player.Character == KingCharacter {
					g.King = nil
				} else {
					i := -1
					for k, v := range g.Servants {
						if v == player {
							i = k
							break
						}
					}

					if i != -1 {
						copy(g.Servants[i:], g.Servants[i+1:])
						g.Servants[len(g.Servants)-1] = nil
						g.Servants = g.Servants[:len(g.Servants)-1]
					}
				}
			}
		case message := <-g.broadcast:
			log.Printf("Received (%s) from (%s): %s\n", message.Type, message.Player.ID, message.Payload)
			g.onMessageReceive(message)
		case <-c:
			if g.Monster != nil {
				for p := range g.players {
					if p.Character == MonsterCharacter {
						continue
					}

					if p.Dead {
						continue
					}

					x := (g.Monster.Position.X) - (p.Position.X)
					y := (g.Monster.Position.Z) - (p.Position.Z)

					if size(x, y) < 1 {
						log.Printf("Player(%s) died", p.ID)
						p.Dead = true
						for pl := range g.players {
							pl.Send(MessageOut{
								Type:    "dead",
								Payload: p,
							})
						}
					}
				}
			}
		}
	}
}

func size(x float64, y float64) float64 {
	return math.Sqrt(math.Pow(x, 2) + math.Pow(y, 2))
}

func (g *Game) onPlayerConnect(p *Player) {
	// g.onPlayerJoin(p)
	g.players[p] = true

	// Send them the map
	p.SendMap()

	log.Printf("New player(%s) joined the game!\n", p.ID)
}

func (g *Game) onMessageReceive(m MessageIn) {

	switch m.Type {
	case "ident":
		var payload string
		json.Unmarshal(m.Payload, &payload)

		m.Player.onIdentify(payload)
	case "pos":
		if !m.Player.Dead {
			json.Unmarshal(m.Payload, &m.Player.Position)

			for p := range g.players {
				if p.ID != m.Player.ID {
					p.Send(MessageOut{
						Type:    "player",
						Payload: m.Player,
					})
				}
			}
		}
	default:
		var payload string
		json.Unmarshal(m.Payload, &payload)
		log.Printf("Payload: %s\n", payload)

		for player := range g.players {
			select {
			case player.send <- append([]byte("Message: "), payload[:]...):
			default:
				log.Println("Connection lost perhaps?")
				close(player.send)
				delete(g.players, player)
			}
		}
	}
}
