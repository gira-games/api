package models

// Game is the representation of a game
// in the database.
type Game struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}