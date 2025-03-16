# gosynth
A simple synth TUI

![Screenshot of the Text User Interface](screenshot.png)

A terminal-based FM synthesizer written in Go, featuring MIDI input support and real-time waveform visualization.

## Features

- Frequency Modulation (FM) synthesis
- MIDI input support
- Real-time waveform visualization with color gradients
- Interactive TUI controls for:
  - Carrier frequency
  - Modulator frequency range
  - Modulation sweep time
  - Modulation index
  - Volume control
  - Real-time display toggle

## Prerequisites

- Go 1.21 or later
- PortAudio development libraries
- RtMidi development libraries

### Installing Dependencies

#### macOS
```bash
brew install portaudio rtmidi
```

#### Ubuntu/Debian
```bash
sudo apt-get install libasound2-dev libportaudio2 libportmididev
sudo apt-get install librtmidi-dev
```

#### Windows
Install MinGW and the required development libraries:
```bash
pacman -S mingw-w64-x86_64-portaudio mingw-w64-x86_64-rtmidi
```

## Installation

1. Clone the repository:
```bash
git clone https://github.com/arroyo/gosynth.git
cd gosynth
```

2. Install Go dependencies:
```bash
go mod download
```

3. Build the project:
```bash
go build
```

## Usage

1. Connect a MIDI device (optional)

2. Run the synthesizer:
```bash
./gosynth
```

3. Controls:
- Use ↑/↓ arrows to select parameters
- Use ←/→ arrows to adjust values
- Press 'q' to quit

## Project Structure

- `main.go`: Application entry point and initialization
- `pkg/synth/`: Synthesizer core functionality
  - Audio processing
  - MIDI handling
  - Parameter management
- `pkg/ui/`: Terminal user interface
  - Interactive controls
  - Waveform visualization
  - Color rendering

## License

Apache License - see LICENSE file for details
