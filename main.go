// Copyright 2023 Alex
// Licensed under the MIT License

package main

import (
	"fmt"
	"time"
	"github.com/oldschoolsysadmin/netris/game"
	"github.com/oldschoolsysadmin/netris/game/piece"
)

func main() {
    // Initialize the game state
    state := game.New()
    if !state.SpawnPiece(piece.I) {
        fmt.Println("Failed to spawn initial piece!")
        return
    }

    // Main game loop
    tick := time.Tick(500 * time.Millisecond)
    for !state.GameOver {
        select {
        case <-tick:
            // Move the active piece down every tick (default drop)
            if !state.MoveDown() {
                // Piece has landed; lock it and spawn a new one
                cleared := state.LockActive()
                fmt.Printf("Locked! Cleared %d lines. Score: %d\n", cleared, state.Score)

                // Spawn a new piece (randomly or sequentially)
                if !state.SpawnPiece(piece.I) {
                    fmt.Println("Game Over!")
                    return
                }
            }
        default:
            // Wait for input (e.g., from terminal or another channel)
            // You can integrate a proper input handling mechanism here.
        }
    }

    fmt.Println("Final Score:", state.Score)
}

