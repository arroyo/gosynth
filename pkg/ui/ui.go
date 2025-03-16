package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"gosynth/pkg/synth"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	waveformWidth  = 100 // Width of the waveform display
	waveformHeight = 20  // Height of the waveform display
)

// Model represents the application UI state
type Model struct {
	spinner  spinner.Model
	synth    *synth.Synth
	realTime bool
	selected int
}

// NewModel creates a new UI model
func NewModel(s *synth.Synth) Model {
	return Model{
		spinner:  spinner.New(),
		synth:    s,
		realTime: false,
		selected: 0,
	}
}

// Init initializes the application
func (m Model) Init() tea.Cmd {
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
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Handle window resize
		return m, nil
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
						m.synth.CarrierFreq.Set(math.Max(20, m.synth.CarrierFreq.Get()-10))
					case 1:
						m.synth.MinModFreq.Set(math.Max(20, m.synth.MinModFreq.Get()-10))
					case 2:
						m.synth.MaxModFreq.Set(math.Max(m.synth.MinModFreq.Get()+10, m.synth.MaxModFreq.Get()-10))
					case 3:
						m.synth.SweepTime.Set(math.Max(0.01, m.synth.SweepTime.Get()-0.01))
					case 4:
						m.synth.ModIndex.Set(math.Max(0, m.synth.ModIndex.Get()-0.05))
					case 5:
						m.synth.Volume.Set(math.Max(0, m.synth.Volume.Get()-0.05))
					}
				case "right":
					switch m.selected {
					case 0:
						m.synth.CarrierFreq.Set(math.Min(2000, m.synth.CarrierFreq.Get()+10))
					case 1:
						m.synth.MinModFreq.Set(math.Min(m.synth.MaxModFreq.Get()-10, m.synth.MinModFreq.Get()+10))
					case 2:
						m.synth.MaxModFreq.Set(math.Min(2000, m.synth.MaxModFreq.Get()+10))
					case 3:
						m.synth.SweepTime.Set(math.Min(1.0, m.synth.SweepTime.Get()+0.01))
					case 4:
						m.synth.ModIndex.Set(math.Min(1.0, m.synth.ModIndex.Get()+0.05))
					case 5:
						m.synth.Volume.Set(math.Min(1.0, m.synth.Volume.Get()+0.05))
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

	borderStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#000000")).
			Foreground(lipgloss.Color("#004400"))

	spaceStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#000000"))
)

// hslToRGB converts HSL color values to RGB
func hslToRGB(h, s, l float64) (r, g, b float64) {
	if s == 0 {
		r, g, b = l, l, l
		return
	}

	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q

	r = hueToRGB(p, q, h+1.0/3.0)
	g = hueToRGB(p, q, h)
	b = hueToRGB(p, q, h-1.0/3.0)
	return
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	if t < 1.0/6.0 {
		return p + (q-p)*6*t
	}
	if t < 1.0/2.0 {
		return q
	}
	if t < 2.0/3.0 {
		return p + (q-p)*(2.0/3.0-t)*6
	}
	return p
}

// getRainbowColor returns a color string based on time and intensity
func getRainbowColor(intensity float64, timeOffset float64) string {
	// Use time to shift the hue
	hue := math.Mod(timeOffset, 1.0)
	// Fixed saturation for vibrant colors
	saturation := 1.0
	// Use intensity to control lightness
	lightness := 0.3 + intensity*0.4

	r, g, b := hslToRGB(hue, saturation, lightness)

	// Convert to hex color string
	return fmt.Sprintf("#%02x%02x%02x",
		int(r*255),
		int(g*255),
		int(b*255))
}

// drawWaveform creates an ASCII art representation of the waveform
func (m Model) drawWaveform() string {
	// Create a buffer for the waveform with double vertical resolution
	buffer := make([][]rune, waveformHeight)
	intensities := make([][]float64, waveformHeight)
	for i := range buffer {
		buffer[i] = make([]rune, waveformWidth)
		intensities[i] = make([]float64, waveformWidth)
		for j := range buffer[i] {
			buffer[i][j] = ' '
			intensities[i][j] = 0.0
		}
	}

	// Draw the center line
	centerY := waveformHeight / 2
	for x := 0; x < waveformWidth; x++ {
		buffer[centerY][x] = '─'
		intensities[centerY][x] = 0.2
	}

	// Calculate and draw the waveforms with interpolation
	points := waveformWidth * 4 // Calculate more points for smoother rendering
	lastCarrierY := -1
	lastFinalY := -1

	// Use current timeIndex for animated display if realTime is true
	displayTime := 0.0
	if m.realTime {
		displayTime = synth.GetTimeIndex()
	}

	for i := 0; i < points; i++ {
		x := i * waveformWidth / points
		t := displayTime + float64(i)/float64(points)*0.02 // Show 0.02 seconds of waveform

		// Generate carrier signal
		carrier := math.Sin(2 * math.Pi * m.synth.CarrierFreq.Get() * t)

		// Calculate modulator wave
		modFreq := m.synth.CalculateModulatorFreq(t)
		modulator := math.Sin(2 * math.Pi * modFreq * t)

		// Apply amplitude modulation
		final := carrier * (1 + m.synth.ModIndex.Get()*modulator)

		// Map the waves to y coordinates with higher resolution
		carrierY := int(carrier*float64(waveformHeight/3)) + centerY
		finalY := int(final*float64(waveformHeight/3)) + centerY

		// Ensure y coordinates are within bounds
		carrierY = clamp(carrierY, 0, waveformHeight-1)
		finalY = clamp(finalY, 0, waveformHeight-1)

		// Calculate intensities
		carrierIntensity := math.Abs(carrier) * 0.7
		finalIntensity := math.Abs(final)

		// Interpolate between last points if they exist
		if lastCarrierY != -1 && x > 0 {
			interpolatePointsWithIntensity(buffer, intensities, x-1, lastCarrierY, x, carrierY, '·', carrierIntensity)
		}
		if lastFinalY != -1 && x > 0 {
			// Use intensity-based characters for the modulated wave
			intensity := math.Abs(final)
			char := getWaveformChar(intensity)
			interpolatePointsWithIntensity(buffer, intensities, x-1, lastFinalY, x, finalY, char, finalIntensity)
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

	// Calculate time-based hue offset
	timeHueOffset := math.Mod(synth.GetTimeIndex()*0.2, 1.0) // Adjust speed of color change here

	// Waveform content
	for y, line := range buffer {
		result.WriteString(borderStyle.Render("║"))
		for x, char := range line {
			intensity := intensities[y][x]
			if char != ' ' {
				color := getRainbowColor(intensity, timeHueOffset+float64(x)/float64(waveformWidth)*0.5)
				style := lipgloss.NewStyle().
					Background(lipgloss.Color("#000000")).
					Foreground(lipgloss.Color(color))
				result.WriteString(style.Render(string(char)))
			} else {
				result.WriteString(spaceStyle.Render(" "))
			}
		}
		result.WriteString(borderStyle.Render("║") + "\n")
	}

	// Bottom border
	result.WriteString(borderStyle.Render("╚" + strings.Repeat("═", waveformWidth) + "╝\n"))

	return result.String()
}

// interpolatePointsWithIntensity draws a line between two points using Bresenham's line algorithm
func interpolatePointsWithIntensity(buffer [][]rune, intensities [][]float64, x1, y1, x2, y2 int, char rune, intensity float64) {
	dx := x2 - x1
	dy := y2 - y1

	if dx == 0 && dy == 0 {
		if y1 >= 0 && y1 < len(buffer) && x1 >= 0 && x1 < len(buffer[0]) {
			buffer[y1][x1] = char
			intensities[y1][x1] = intensity
		}
		return
	}

	if dx != 0 {
		for x := x1; x <= x2; x++ {
			t := float64(x-x1) / float64(dx)
			y := int(float64(y1) + t*float64(dy))
			if y >= 0 && y < len(buffer) && x >= 0 && x < len(buffer[0]) {
				buffer[y][x] = char
				intensities[y][x] = intensity
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
				intensities[y][x1] = intensity
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
func (m Model) View() string {
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
		MarginLeft(2).
		MarginRight(2)

	var s strings.Builder

	s.WriteString(baseStyle.Render("Synthesizer Controls") + "\n\n")

	// Carrier Frequency
	if m.selected == 0 {
		s.WriteString(selectedStyle.Render(fmt.Sprintf("> Carrier Frequency: %.1f Hz", m.synth.CarrierFreq.Get())) + "\n")
	} else {
		s.WriteString(baseStyle.Render(fmt.Sprintf("  Carrier Frequency: %.1f Hz", m.synth.CarrierFreq.Get())) + "\n")
	}

	// Min Modulator Frequency
	if m.selected == 1 {
		s.WriteString(selectedStyle.Render("> Min Modulator Frequency: "))
	} else {
		s.WriteString(baseStyle.Render("  Min Modulator Frequency: "))
	}
	s.WriteString(baseStyle.Render(fmt.Sprintf("%.1f Hz", m.synth.MinModFreq.Get())) + "\n")

	// Max Modulator Frequency
	if m.selected == 2 {
		s.WriteString(selectedStyle.Render("> Max Modulator Frequency: "))
	} else {
		s.WriteString(baseStyle.Render("  Max Modulator Frequency: "))
	}
	s.WriteString(baseStyle.Render(fmt.Sprintf("%.1f Hz", m.synth.MaxModFreq.Get())) + "\n")

	// Sweep Time
	if m.selected == 3 {
		s.WriteString(selectedStyle.Render("> Sweep Time: "))
	} else {
		s.WriteString(baseStyle.Render("  Sweep Time: "))
	}
	s.WriteString(baseStyle.Render(fmt.Sprintf("%.2f s", m.synth.SweepTime.Get())) + "\n")

	// Modulation Index
	if m.selected == 4 {
		s.WriteString(selectedStyle.Render("> Modulation Index: "))
	} else {
		s.WriteString(baseStyle.Render("  Modulation Index: "))
	}
	s.WriteString(baseStyle.Render(fmt.Sprintf("%.2f", m.synth.ModIndex.Get())) + "\n")

	// Volume
	if m.selected == 5 {
		s.WriteString(selectedStyle.Render("> Volume: "))
	} else {
		s.WriteString(baseStyle.Render("  Volume: "))
	}
	s.WriteString(baseStyle.Render(fmt.Sprintf("%.2f", m.synth.Volume.Get())) + "\n")

	// Real-time toggle
	if m.selected == 6 {
		s.WriteString(selectedStyle.Render("> Real-time display: "))
	} else {
		s.WriteString(baseStyle.Render("  Real-time display: "))
	}
	s.WriteString(baseStyle.Render(fmt.Sprintf("%v", m.realTime)) + "\n\n")

	// Add instructions with base style
	s.WriteString(baseStyle.Render("\nUse ↑↓ to select, ←→ to adjust, q to quit\n"))

	// Add waveform visualization
	s.WriteString(m.drawWaveform())

	// Apply container style to the entire output
	return containerStyle.Render(
		lipgloss.NewStyle().
			Background(lipgloss.Color("#000000")).
			Render(s.String()),
	)
}
