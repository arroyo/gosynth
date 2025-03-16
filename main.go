package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gosynth/pkg/synth"
	"gosynth/pkg/ui"

	tea "github.com/charmbracelet/bubbletea"
	"gitlab.com/gomidi/midi/v2"
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv" // autoregisters driver
)

func main() {
	// Initialize MIDI
	defer midi.CloseDriver()

	// Create a new synthesizer
	s := synth.NewSynth()

	// Start the synthesizer
	if err := s.Start(); err != nil {
		log.Fatal(err)
	}
	defer s.Stop()

	// Create and start the UI
	p := tea.NewProgram(ui.NewModel(s))

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		s.Stop()
		os.Exit(0)
	}()

	// Run the UI
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
