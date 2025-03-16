package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gordonklaus/portaudio"
	"gitlab.com/gomidi/midi/v2"
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv" // autoregisters driver
)

const (
	sampleRate      = 44100
	minModFreq      = 100.0 // Minimum modulation frequency in Hz
	maxModFreq      = 600.0 // Maximum modulation frequency in Hz
	freqSweepTime   = .100  // Time to finish 10Hz of sweep
	modulationIndex = 0.5   // Modulation intensity
	waveformWidth   = 100   // Width of the waveform display
	waveformHeight  = 20    // Height of the waveform display
	clipThreshold   = 0.5   // Threshold where soft clipping begins
	clipHardLimit   = 0.75  // Maximum amplitude after clipping
	initialVolume   = 0.8   // Initial volume level
)

var timeIndex float64 = 0

// SmoothValue represents a parameter value
type SmoothValue struct {
	value float64
}

func (sv *SmoothValue) update() {
	// No smoothing needed
}

func (sv *SmoothValue) set(value float64) {
	sv.value = value
}

func (sv *SmoothValue) get() float64 {
	return sv.value
}

// Convert MIDI note to frequency
func midiNoteToFreq(note uint8) float64 {
	return 440.0 * math.Pow(2, (float64(note)-69.0)/12.0)
}

// MIDIMsg represents a MIDI message
type MIDIMsg struct {
	Note uint8
}

// SynthModel represents the application state
type SynthModel struct {
	spinner     spinner.Model
	carrierFreq SmoothValue
	minModFreq  SmoothValue
	maxModFreq  SmoothValue
	sweepTime   SmoothValue
	modIndex    SmoothValue
	volume      SmoothValue
	realTime    bool
	selected    int
}

// Init initializes the application
func (m *SynthModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.Every(time.Second/60, func(time.Time) tea.Msg {
			return frameMsg{}
		}),
	)
}

// Custom message type for frame updates
type frameMsg struct{}

// Update handles application updates
func (m *SynthModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case frameMsg:
		// Request next frame
		return m, tea.Batch(
			m.spinner.Tick,
			tea.Every(time.Second/60, func(time.Time) tea.Msg {
				return frameMsg{}
			}),
		)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up":
			if m.selected > 0 {
				m.selected--
			}
		case "down":
			if m.selected < 6 {
				m.selected++
			}
		case "left", "right":
			if m.selected == 6 {
				m.realTime = !m.realTime
			} else {
				switch msg.String() {
				case "left":
					switch m.selected {
					case 0:
						m.carrierFreq.set(math.Max(20, m.carrierFreq.value-10))
					case 1:
						m.minModFreq.set(math.Max(20, m.minModFreq.value-10))
					case 2:
						m.maxModFreq.set(math.Max(m.minModFreq.value+10, m.maxModFreq.value-10))
					case 3:
						m.sweepTime.set(math.Max(0.01, m.sweepTime.value-0.01))
					case 4:
						m.modIndex.set(math.Max(0, m.modIndex.value-0.1))
					case 5:
						m.volume.set(math.Max(0, m.volume.value-0.1))
					}
				case "right":
					switch m.selected {
					case 0:
						m.carrierFreq.set(math.Min(2000, m.carrierFreq.value+10))
					case 1:
						m.minModFreq.set(math.Min(m.maxModFreq.value-10, m.minModFreq.value+10))
					case 2:
						m.maxModFreq.set(math.Min(2000, m.maxModFreq.value+10))
					case 3:
						m.sweepTime.set(math.Min(1.0, m.sweepTime.value+0.01))
					case 4:
						m.modIndex.set(math.Min(1.0, m.modIndex.value+0.1))
					case 5:
						m.volume.set(math.Min(1.0, m.volume.value+0.1))
					}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// getWaveformChar returns an appropriate character based on intensity
func getWaveformChar(value float64) rune {
	switch {
	case value >= 0.8:
		return '█'
	case value >= 0.6:
		return '▓'
	case value >= 0.4:
		return '▒'
	case value >= 0.2:
		return '░'
	case value > 0:
		return '·'
	default:
		return ' '
	}
}

// Create styles for different waveform elements
var (
	waveformStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#000000")).
			Foreground(lipgloss.Color("#00ff00"))

	carrierStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#000000")).
			Foreground(lipgloss.Color("#00ff00"))

	// Different shades of green for intensity levels
	intensityStyles = map[rune]lipgloss.Style{
		'█': lipgloss.NewStyle().
			Background(lipgloss.Color("#000000")).
			Foreground(lipgloss.Color("#00ff00")),
		'▓': lipgloss.NewStyle().
			Background(lipgloss.Color("#000000")).
			Foreground(lipgloss.Color("#00dd00")),
		'▒': lipgloss.NewStyle().
			Background(lipgloss.Color("#000000")).
			Foreground(lipgloss.Color("#00bb00")),
		'░': lipgloss.NewStyle().
			Background(lipgloss.Color("#000000")).
			Foreground(lipgloss.Color("#009900")),
		'·': lipgloss.NewStyle().
			Background(lipgloss.Color("#000000")).
			Foreground(lipgloss.Color("#007700")),
		'─': lipgloss.NewStyle().
			Background(lipgloss.Color("#000000")).
			Foreground(lipgloss.Color("#004400")),
		' ': lipgloss.NewStyle().
			Background(lipgloss.Color("#000000")),
	}

	borderStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#000000")).
			Foreground(lipgloss.Color("#004400"))
)

// drawWaveform creates an ASCII art representation of the waveform
func drawWaveform(model *SynthModel) string {
	// Create a buffer for the waveform with double vertical resolution
	buffer := make([][]rune, waveformHeight)
	for i := range buffer {
		buffer[i] = make([]rune, waveformWidth)
		for j := range buffer[i] {
			buffer[i][j] = ' '
		}
	}

	// Draw the center line
	centerY := waveformHeight / 2
	for x := 0; x < waveformWidth; x++ {
		buffer[centerY][x] = '─'
	}

	// Calculate and draw the waveforms with interpolation
	points := waveformWidth * 4 // Calculate more points for smoother rendering
	lastCarrierY := -1
	lastFinalY := -1

	// Use current timeIndex for animated display if realTime is true
	displayTime := 0.0
	if model.realTime {
		displayTime = timeIndex
	}

	for i := 0; i < points; i++ {
		x := i * waveformWidth / points
		t := displayTime + float64(i)/float64(points)*0.02 // Show 0.02 seconds of waveform

		// Generate carrier signal
		carrier := math.Sin(2 * math.Pi * model.carrierFreq.get() * t)

		// Calculate modulator wave
		modFreq := calculateModulatorFreq(t, model)
		modulator := math.Sin(2 * math.Pi * modFreq * t)

		// Apply amplitude modulation
		final := carrier * (1 + model.modIndex.get()*modulator)

		// Map the waves to y coordinates with higher resolution
		carrierY := int(carrier*float64(waveformHeight/3)) + centerY
		finalY := int(final*float64(waveformHeight/3)) + centerY

		// Ensure y coordinates are within bounds
		carrierY = clamp(carrierY, 0, waveformHeight-1)
		finalY = clamp(finalY, 0, waveformHeight-1)

		// Interpolate between last points if they exist
		if lastCarrierY != -1 && x > 0 {
			interpolatePoints(buffer, x-1, lastCarrierY, x, carrierY, '·')
		}
		if lastFinalY != -1 && x > 0 {
			// Use intensity-based characters for the modulated wave
			intensity := math.Abs(final)
			char := getWaveformChar(intensity)
			interpolatePoints(buffer, x-1, lastFinalY, x, finalY, char)
		}

		lastCarrierY = carrierY
		lastFinalY = finalY
	}

	// Convert buffer to string with a fancier border and colors
	var result strings.Builder
	result.WriteString("\n")
	result.WriteString(waveformStyle.Render("Waveform Display") + " ")
	result.WriteString(carrierStyle.Render("(carrier: ·)") + " ")
	result.WriteString(waveformStyle.Render("(modulated: ░▒▓█)") + "\n")

	// Top border
	result.WriteString(borderStyle.Render("╔" + strings.Repeat("═", waveformWidth) + "╗\n"))

	// Waveform content
	for _, line := range buffer {
		result.WriteString(borderStyle.Render("║"))
		for _, char := range line {
			result.WriteString(intensityStyles[char].Render(string(char)))
		}
		result.WriteString(borderStyle.Render("║") + "\n")
	}

	// Bottom border
	result.WriteString(borderStyle.Render("╚" + strings.Repeat("═", waveformWidth) + "╝\n"))

	return result.String()
}

// interpolatePoints draws a line between two points using Bresenham's line algorithm
func interpolatePoints(buffer [][]rune, x1, y1, x2, y2 int, char rune) {
	dx := x2 - x1
	dy := y2 - y1

	if dx == 0 && dy == 0 {
		if y1 >= 0 && y1 < len(buffer) && x1 >= 0 && x1 < len(buffer[0]) {
			buffer[y1][x1] = char
		}
		return
	}

	if dx != 0 {
		for x := x1; x <= x2; x++ {
			t := float64(x-x1) / float64(dx)
			y := int(float64(y1) + t*float64(dy))
			if y >= 0 && y < len(buffer) && x >= 0 && x < len(buffer[0]) {
				buffer[y][x] = char
			}
		}
	} else {
		step := 1
		if dy < 0 {
			step = -1
		}
		for y := y1; y != y2+step; y += step {
			if y >= 0 && y < len(buffer) && x1 >= 0 && x1 < len(buffer[0]) {
				buffer[y][x1] = char
			}
		}
	}
}

// clamp ensures a value is within the given range
func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// View renders the application UI
func (m *SynthModel) View() string {
	// Create base styles for menu items
	baseStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#000000"))

	selectedStyle := baseStyle.
		Foreground(lipgloss.Color("#00ff00"))

	// Create container style for the entire app
	containerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Padding(1)

	var s string
	s += baseStyle.Render("Synthesizer Controls") + "\n\n"

	// Carrier Frequency
	if m.selected == 0 {
		s += selectedStyle.Render("> Carrier Frequency: ")
	} else {
		s += baseStyle.Render("  Carrier Frequency: ")
	}
	s += baseStyle.Render(fmt.Sprintf("%.1f Hz", m.carrierFreq.value)) + "\n"

	// Min Modulator Frequency
	if m.selected == 1 {
		s += selectedStyle.Render("> Min Modulator Frequency: ")
	} else {
		s += baseStyle.Render("  Min Modulator Frequency: ")
	}
	s += baseStyle.Render(fmt.Sprintf("%.1f Hz", m.minModFreq.value)) + "\n"

	// Max Modulator Frequency
	if m.selected == 2 {
		s += selectedStyle.Render("> Max Modulator Frequency: ")
	} else {
		s += baseStyle.Render("  Max Modulator Frequency: ")
	}
	s += baseStyle.Render(fmt.Sprintf("%.1f Hz", m.maxModFreq.value)) + "\n"

	// Sweep Time
	if m.selected == 3 {
		s += selectedStyle.Render("> Sweep Time: ")
	} else {
		s += baseStyle.Render("  Sweep Time: ")
	}
	s += baseStyle.Render(fmt.Sprintf("%.2f s", m.sweepTime.value)) + "\n"

	// Modulation Index
	if m.selected == 4 {
		s += selectedStyle.Render("> Modulation Index: ")
	} else {
		s += baseStyle.Render("  Modulation Index: ")
	}
	s += baseStyle.Render(fmt.Sprintf("%.2f", m.modIndex.value)) + "\n"

	// Volume
	if m.selected == 5 {
		s += selectedStyle.Render("> Volume: ")
	} else {
		s += baseStyle.Render("  Volume: ")
	}
	s += baseStyle.Render(fmt.Sprintf("%.2f", m.volume.value)) + "\n"

	// Real-time toggle
	if m.selected == 6 {
		s += selectedStyle.Render("> Real-time display: ")
	} else {
		s += baseStyle.Render("  Real-time display: ")
	}
	s += baseStyle.Render(fmt.Sprintf("%v", m.realTime)) + "\n\n"

	// Add instructions with base style
	s += baseStyle.Render("\nUse ↑↓ to select, ←→ to adjust, q to quit\n")

	// Add waveform visualization
	s += drawWaveform(m)

	// Apply container style to the entire output
	return containerStyle.Render(s)
}

// calculateModulatorFreq returns the current modulator frequency based on time
func calculateModulatorFreq(t float64, model *SynthModel) float64 {
	// Calculate how many periods have passed
	periods := t / model.sweepTime.get()

	// Calculate the frequency range
	freqRange := model.maxModFreq.get() - model.minModFreq.get()

	// Calculate the frequency increase (wrap around using modulo)
	freqIncrease := math.Mod(periods*freqRange, freqRange)

	// Calculate current frequency
	return model.minModFreq.get() + freqIncrease
}

// softClip applies soft clipping to prevent harsh distortion
func softClip(sample float64) float64 {
	// Apply a hyperbolic tangent-based soft clipper
	if math.Abs(sample) > clipThreshold {
		// Calculate how much the signal exceeds the threshold
		excess := math.Abs(sample) - clipThreshold

		// Apply progressively stronger compression as the signal gets louder
		compressionFactor := 1.0 - math.Min(1.0, excess/(clipHardLimit-clipThreshold))

		// Determine the sign of the original sample
		sign := 1.0
		if sample < 0 {
			sign = -1.0
		}

		// Apply the compression
		return sign * (clipThreshold + excess*compressionFactor)
	}
	return sample
}

func audioCallback(out []float32, model *SynthModel) {
	// Process audio
	for i := range out {
		t := timeIndex + float64(i)/sampleRate

		// Generate carrier signal
		carrier := math.Sin(2 * math.Pi * model.carrierFreq.get() * t)

		// Calculate modulator wave
		modFreq := calculateModulatorFreq(t, model)
		modulator := math.Sin(2 * math.Pi * modFreq * t)

		// Apply amplitude modulation
		sample := carrier * (1 + model.modIndex.get()*modulator)

		// Apply soft clipping to prevent distortion
		sample = softClip(sample)

		// Apply volume control
		out[i] = float32(sample * model.volume.get())
	}

	timeIndex += float64(len(out)) / sampleRate
}

func main() {
	// Initialize PortAudio
	err := portaudio.Initialize()
	if err != nil {
		panic(err)
	}
	defer portaudio.Terminate()

	// Find the first available MIDI input device
	ports := midi.GetInPorts()
	if len(ports) == 0 {
		fmt.Println("No MIDI input devices available")
		return
	}

	inPort, err := midi.InPort(0)
	if err != nil {
		fmt.Printf("Error opening MIDI input: %v\n", err)
		return
	}

	// Create initial model
	model := &SynthModel{
		spinner:     spinner.New(),
		carrierFreq: SmoothValue{value: 440.0}, // Start with A4 note
		minModFreq:  SmoothValue{value: minModFreq},
		maxModFreq:  SmoothValue{value: maxModFreq},
		sweepTime:   SmoothValue{value: freqSweepTime},
		modIndex:    SmoothValue{value: modulationIndex},
		volume:      SmoothValue{value: initialVolume},
		realTime:    false,
		selected:    0,
	}

	// Set up MIDI message handling
	stopListening, err := midi.ListenTo(inPort, func(msg midi.Message, timestampms int32) {
		var channel, key, velocity uint8
		if msg.GetNoteStart(&channel, &key, &velocity) {
			model.carrierFreq.set(midiNoteToFreq(key))
		}
	})
	if err != nil {
		fmt.Printf("Error setting up MIDI listener: %v\n", err)
		return
	}
	defer stopListening()

	// Open default audio output stream with callback
	stream, err := portaudio.OpenDefaultStream(0, 1, float64(sampleRate), 1024, func(out []float32) {
		audioCallback(out, model)
	})
	if err != nil {
		panic(err)
	}
	defer stream.Close()

	err = stream.Start()
	if err != nil {
		panic(err)
	}

	// Run the Bubble Tea program
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
