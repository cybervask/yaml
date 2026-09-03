package yaml

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestEncoderRoundTrip verifies that the streaming Encoder serializes values
// with configured indentation into the output stream.
func TestEncoderRoundTrip(t *testing.T) {
	type Config struct {
		Host    string        `yaml:"host" description:"Listen host"`
		Timeout time.Duration `yaml:"timeout" description:"Request timeout"`
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.SetIndent(4)

	if err := enc.Encode(Config{Host: "localhost", Timeout: 10 * time.Second}); err != nil {
		t.Fatalf("expected successful encode, got: %v", err)
	}

	if err := enc.Close(); err != nil {
		t.Fatalf("expected successful close, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "host: localhost") {
		t.Errorf("expected host field in output, got:\n%s", out)
	}
}

// TestEncoderSeqIndentModes verifies both sequence indentation modes
// of the streaming Encoder.
func TestEncoderSeqIndentModes(t *testing.T) {
	type Config struct {
		Items []string `yaml:"items" description:"Item list"`
	}

	compact := func() string {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		enc.CompactSeqIndent()
		if err := enc.Encode(Config{Items: []string{"a", "b"}}); err != nil {
			t.Fatal(err)
		}

		if err := enc.Close(); err != nil {
			t.Fatal(err)
		}

		return buf.String()
	}()

	standard := func() string {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		enc.DefaultSeqIndent()
		if err := enc.Encode(Config{Items: []string{"a", "b"}}); err != nil {
			t.Fatal(err)
		}

		if err := enc.Close(); err != nil {
			t.Fatal(err)
		}

		return buf.String()
	}()

	if compact == standard {
		t.Errorf("expected different layouts for compact and default seq indent, got identical:\n%s", compact)
	}
}

// TestDecoderRoundTrip verifies the streaming Decoder: defaults are applied,
// the document is decoded, and validation runs afterwards.
func TestDecoderRoundTrip(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" default:"localhost" description:"Listen host"`
		Port int    `yaml:"port" validate:"min=1,max=65535" description:"Listen port"`
	}

	dec := NewDecoder(strings.NewReader("port: 9443\n"))

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		t.Fatalf("expected successful decode, got: %v", err)
	}

	if cfg.Host != "localhost" || cfg.Port != 9443 {
		t.Errorf("unexpected decoded state: %+v", cfg)
	}
}

// TestDecoderKnownFields verifies that the Decoder reports unknown keys
// when KnownFields is enabled.
func TestDecoderKnownFields(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" description:"Listen host"`
	}

	dec := NewDecoder(strings.NewReader("unknown_key: 1\n"))
	dec.KnownFields(true)

	err := dec.Decode(&Config{})
	if err == nil {
		t.Fatal("expected unknown field error, got nil")
	}
}

// TestDumpAndMarshal verifies the Dump/Marshal pair producing standard YAML
// output for plain structures.
func TestDumpAndMarshal(t *testing.T) {
	type Config struct {
		Host string `yaml:"host" description:"Listen host"`
		Port int    `yaml:"port" description:"Listen port"`
	}

	dumped, err := Dump(Config{Host: "example.com", Port: 80})
	if err != nil {
		t.Fatalf("expected successful dump, got: %v", err)
	}

	if !strings.Contains(string(dumped), "host: example.com") {
		t.Errorf("expected host field in dump, got:\n%s", dumped)
	}

	marshaled, err := Marshal(Config{Host: "example.com", Port: 80})
	if err != nil {
		t.Fatalf("expected successful marshal, got: %v", err)
	}

	if !bytes.Equal(marshaled, dumped) {
		t.Errorf("expected Marshal to match Dump output, got:\n%s\nvs\n%s", marshaled, dumped)
	}
}

// TestDumperStream verifies the streaming Dumper writing documents
// sequentially to an output stream.
func TestDumperStream(t *testing.T) {
	type Config struct {
		Name string `yaml:"name" description:"Service name"`
	}

	var buf bytes.Buffer
	dumper, err := NewDumper(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if err := dumper.Dump(Config{Name: "billing"}); err != nil {
		t.Fatalf("expected successful dump, got: %v", err)
	}

	if err := dumper.Close(); err != nil {
		t.Fatalf("expected successful close, got: %v", err)
	}

	if !strings.Contains(buf.String(), "name: billing") {
		t.Errorf("expected name field in stream, got:\n%s", buf.String())
	}
}
