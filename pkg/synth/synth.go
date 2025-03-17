package synth

import (
	"math"

	"github.com/gordonklaus/portaudio"
	"gitlab.com/gomidi/midi/v2"
)

const (
	SampleRate      = 44100
	MinModFreq      = 100.0 // Minimum modulation frequency in Hz
	MaxModFreq      = 600.0 // Maximum modulation frequency in Hz
	FreqSweepTime   = .300  // Time to finish 10Hz of sweep
	ModulationIndex = 0.5   // Modulation intensity
	ClipThreshold   = 0.6   // Threshold where soft clipping begins
	ClipHardLimit   = 0.85  // Maximum amplitude after clipping
	InitialVolume   = 0.75  // Initial volume level
	AudioBufferSize = 2048  // Increased buffer size for more stability
)

// SmoothValue represents a parameter value
type SmoothValue struct {
	value float64
}

func (sv *SmoothValue) Update() {
	// No smoothing needed
}

func (sv *SmoothValue) Set(value float64) {
	sv.value = value
}

func (sv *SmoothValue) Get() float64 {
	return sv.value
}

// Synth represents the synthesizer state
type Synth struct {
	CarrierFreq SmoothValue
	MinModFreq  SmoothValue
	MaxModFreq  SmoothValue
	SweepTime   SmoothValue
	ModIndex    SmoothValue
	Volume      SmoothValue
	stream      *portaudio.Stream
	stopMIDI    func()
	buffer      []float32 // Add audio buffer
	timeIndex   float64   // Move timeIndex into the struct
}

// NewSynth creates a new synthesizer instance
func NewSynth() *Synth {
	return &Synth{
		CarrierFreq: SmoothValue{value: 440.0}, // Start with A4 note
		MinModFreq:  SmoothValue{value: MinModFreq},
		MaxModFreq:  SmoothValue{value: MaxModFreq},
		SweepTime:   SmoothValue{value: FreqSweepTime},
		ModIndex:    SmoothValue{value: ModulationIndex},
		Volume:      SmoothValue{value: InitialVolume},
		buffer:      make([]float32, AudioBufferSize),
		timeIndex:   0,
	}
}

// MIDINoteToFreq converts a MIDI note number to frequency
func MIDINoteToFreq(note uint8) float64 {
	return 440.0 * math.Pow(2, (float64(note)-69.0)/12.0)
}

// CalculateModulatorFreq returns the current modulator frequency based on time
func (s *Synth) CalculateModulatorFreq(t float64) float64 {
	// Calculate how many periods have passed
	periods := t / s.SweepTime.Get()

	// Calculate the frequency range
	freqRange := s.MaxModFreq.Get() - s.MinModFreq.Get()

	// Calculate the frequency increase (wrap around using modulo)
	freqIncrease := math.Mod(periods*freqRange, freqRange)

	// Calculate current frequency
	return s.MinModFreq.Get() + freqIncrease
}

// SoftClip applies soft clipping to prevent harsh distortion
func SoftClip(sample float64) float64 {
	// Apply a hyperbolic tangent-based soft clipper
	if math.Abs(sample) > ClipThreshold {
		// Calculate how much the signal exceeds the threshold
		excess := math.Abs(sample) - ClipThreshold

		// Apply progressively stronger compression as the signal gets louder
		compressionFactor := 1.0 - math.Min(1.0, excess/(ClipHardLimit-ClipThreshold))

		// Determine the sign of the original sample
		sign := 1.0
		if sample < 0 {
			sign = -1.0
		}

		// Apply the compression
		return sign * (ClipThreshold + excess*compressionFactor)
	}
	return sample
}

// AudioCallback processes audio samples
func (s *Synth) AudioCallback(out []float32) {
	// Process audio
	for i := range out {
		t := s.timeIndex + float64(i)/SampleRate

		// Generate carrier signal
		carrier := math.Sin(2 * math.Pi * s.CarrierFreq.Get() * t)

		// Calculate modulator wave
		modFreq := s.CalculateModulatorFreq(t)
		modulator := math.Sin(2 * math.Pi * modFreq * t)

		// Apply amplitude modulation
		sample := carrier * (1 + s.ModIndex.Get()*modulator)

		// Apply soft clipping to prevent distortion
		sample = SoftClip(sample)

		// Apply volume control and store in buffer
		s.buffer[i] = float32(sample * s.Volume.Get())
	}

	// Copy buffer to output
	copy(out, s.buffer[:len(out)])

	s.timeIndex += float64(len(out)) / SampleRate
}

// Start initializes and starts the synthesizer
func (s *Synth) Start() error {
	// Initialize PortAudio
	err := portaudio.Initialize()
	if err != nil {
		return err
	}

	// Try to initialize MIDI, but continue even if it fails
	ports := midi.GetInPorts()
	if len(ports) > 0 {
		inPort, err := midi.InPort(0)
		if err == nil {
			// Set up MIDI message handling
			stopListening, err := midi.ListenTo(inPort, func(msg midi.Message, timestampms int32) {
				var channel, key, velocity uint8
				if msg.GetNoteStart(&channel, &key, &velocity) {
					s.CarrierFreq.Set(MIDINoteToFreq(key))
				}
			})
			if err == nil {
				s.stopMIDI = stopListening
			}
		}
	}

	// Get default output device
	defaultDevice, err := portaudio.DefaultOutputDevice()
	if err != nil {
		return err
	}

	// Set up high-priority audio stream with optimal buffer size
	streamParams := portaudio.StreamParameters{
		Output: portaudio.StreamDeviceParameters{
			Device:   defaultDevice,
			Channels: 1,
			Latency:  defaultDevice.DefaultHighOutputLatency,
		},
		SampleRate:      SampleRate,
		FramesPerBuffer: AudioBufferSize,
	}

	// Open audio stream with optimized parameters
	stream, err := portaudio.OpenStream(streamParams, func(out []float32) {
		s.AudioCallback(out)
	})
	if err != nil {
		return err
	}
	s.stream = stream

	return stream.Start()
}

// Stop cleans up and stops the synthesizer
func (s *Synth) Stop() error {
	if s.stopMIDI != nil {
		s.stopMIDI()
	}
	if s.stream != nil {
		if err := s.stream.Close(); err != nil {
			return err
		}
	}
	return portaudio.Terminate()
}

// GetTimeIndex returns the current time index
func (s *Synth) GetTimeIndex() float64 {
	return s.timeIndex
}
