package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os/exec"

	"github.com/faiface/beep"
	"github.com/ghetzel/argonaut"
	"github.com/ghetzel/go-stockutil/log"
)

var i2fFactor = (1.0 / 256)

type GlobalOptions struct {
	LogLevel   string `argonaut:"v,short"`
	HideBanner bool   `argonaut:"hide_banner"`
	TimeLimit  int    `argonaut:"timelimit,short"`
	Overwrite  bool   `argonaut:"y,short"`
}

type CodecOptions struct {
	ArgName    argonaut.ArgName       `argonaut:"codec,short"`
	Stream     string                 `argonaut:",suffixprev,delimiters=[:]"`
	Codec      string                 `argonaut:",skipname"`
	Parameters map[string]interface{} `argonaut:",short"`
}

type MetadataValue struct {
	Metadata argonaut.ArgName `argonaut:"metadata,short"`
	Stream   string           `argonaut:",suffixprev,delimiters=[:]"`
	Key      string
	Value    interface{}
}

type FilterOptions struct {
	ArgName argonaut.ArgName `argonaut:"filter,short"`
	Stream  string           `argonaut:",suffixprev,delimiters=[:]"`
	Graph   []string         `argonaut:",positional"`
}

type MapOptions struct {
	ArgName     argonaut.ArgName `argonaut:"map,short"`
	InputFileID int              `argonaut:",positional"`
	Stream      string           `argonaut:",suffixprev,delimiters=[:]"`
	MapStream   int              `argonaut:",suffixprev,delimiters=[:],required"`
}

type CommonOptions struct {
	Codecs        []CodecOptions
	Duration      string                 `argonaut:"t"`
	SeekStart     string                 `argonaut:"ss"`
	Format        string                 `argonaut:"f"`
	FormatOptions map[string]interface{} `argonaut:",short"`
}

type InputOptions struct {
	CommonOptions
	Metadata []MetadataValue
	URL      string `argonaut:"i,required"`
}

type OutputOptions struct {
	CommonOptions
	Filters    []FilterOptions
	Maps       []MapOptions
	Parameters []string `argonaut:",positional"`
	URL        string   `argonaut:",positional,required"`
}

type ffmpeg struct {
	Command argonaut.CommandName `argonaut:"ffmpeg"`
	Global  *GlobalOptions       `argonaut:",label=global_options"`
	Input   *InputOptions        `argonaut:",label=input_file_options"`
	Output  *OutputOptions       `argonaut:",label=output_file_options"`
}

type decode struct {
	cmd    *exec.Cmd
	source io.ReadCloser
	cmdout io.ReadCloser
	cmdin  io.WriteCloser
	err    error
	pos    int
}

func newDecodeStream(cmd *exec.Cmd, source io.ReadCloser) (*decode, beep.Format, error) {
	decoder := &decode{
		cmd:    cmd,
		source: source,
	}

	// wire up file input
	if in, err := cmd.StdinPipe(); err == nil {
		decoder.cmdin = in
	} else {
		return nil, beep.Format{}, err
	}

	// wire up decoded output
	if out, err := cmd.StdoutPipe(); err == nil {
		decoder.cmdout = out
	} else {
		return nil, beep.Format{}, err
	}

	if err := decoder.start(); err == nil {
		return decoder, beep.Format{
			SampleRate:  44100,
			NumChannels: 2,
			Precision:   2,
		}, nil
	} else {
		return nil, beep.Format{}, err
	}
}

func (self *decode) start() error {
	log.Debugf("starting %v", self.cmd.Args)

	go func() {
		defer self.source.Close()
		io.Copy(self.cmdin, self.source)
	}()

	return self.cmd.Start()
}

func (self *decode) Stream(samples [][2]float64) (int, bool) {
	// allocate for samples*bytesize*channels bytes
	sz := len(samples) * 2 * 2
	data := make([]byte, sz)

	if n, err := self.cmdout.Read(data); err == nil {
		if n == sz {
			return self.populateSamples(samples, data)
		} else {
			self.err = fmt.Errorf("invalid read: expected %d bytes, got %d", sz, n)
			return 0, false
		}
	} else if err == io.EOF {
		return self.populateSamples(samples, data)
	} else {
		self.err = err
		return 0, false
	}
}

func (self *decode) populateSamples(samples [][2]float64, data []byte) (int, bool) {
	if len(data)%4 != 0 {
		log.Warningf("Expected datalen%4, got %d", len(data))
		return 0, false
	}

	if len(samples) != (len(data) / 4) {
		log.Warningf("Incorrect samples for %d bytes of data, expected %d, got %d", len(data), len(data)/4, len(samples))
		return 0, false
	}

	var si int

	for i := 0; i < len(data); i += 4 {
		amplitudeL := binary.LittleEndian.Uint16(data[i:])
		amplitudeR := binary.LittleEndian.Uint16(data[i+2:])

		samples[si][0] = float64(amplitudeL) / 65536
		samples[si][1] = float64(amplitudeR) / 65536
		si += 1

	}

	self.pos += len(samples)
	log.Debugf("pos now %d", self.pos)

	return len(samples), true
}

func (self *decode) Err() error {
	return self.err
}

func (self *decode) Len() int {
	return 44100 * 10
}

func (self *decode) Position() int {
	return self.pos
}

func (self *decode) Seek(p int) error {
	return fmt.Errorf("not seekable")
}

func (self *decode) Close() error {
	return self.cmd.Process.Kill()
}

func ffmpegDecode(readCloser io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
	if cmd, err := argonaut.Command(&ffmpeg{
		Global: &GlobalOptions{
			LogLevel:  `24`,
			Overwrite: true,
		},
		Input: &InputOptions{
			URL: `pipe:0`,
		},
		Output: &OutputOptions{
			CommonOptions: CommonOptions{
				Codecs: []CodecOptions{
					{
						Stream: `a`,
						Codec:  `pcm_u16le`,
					},
				},
				Format: `u16le`,
				FormatOptions: map[string]interface{}{
					`ac`: 2,
					`ar`: 44100,
				},
			},
			Parameters: []string{`-strict`, `-2`},
			URL:        `pipe:1`,
		},
	}); err == nil {
		return newDecodeStream(cmd, readCloser)
	} else {
		return nil, beep.Format{}, err
	}
}
