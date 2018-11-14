package imagebuilder

import (
	"testing"
)

func TestGetEnv(t *testing.T) {
	sw := &shellWord{envs: nil}

	sw.envs = []string{}
	if sw.getEnv("foo") != "" {
		t.Fatal("2 - 'foo' should map to ''")
	}

	sw.envs = []string{"foo"}
	if sw.getEnv("foo") != "" {
		t.Fatal("3 - 'foo' should map to ''")
	}

	sw.envs = []string{"foo="}
	if sw.getEnv("foo") != "" {
		t.Fatal("4 - 'foo' should map to ''")
	}

	sw.envs = []string{"foo=bar"}
	if sw.getEnv("foo") != "bar" {
		t.Fatal("5 - 'foo' should map to 'bar'")
	}

	sw.envs = []string{"foo=bar", "car=hat"}
	if sw.getEnv("foo") != "bar" {
		t.Fatal("6 - 'foo' should map to 'bar'")
	}
	if sw.getEnv("car") != "hat" {
		t.Fatal("7 - 'car' should map to 'hat'")
	}

	// Make sure we grab the first 'car' in the list
	sw.envs = []string{"foo=bar", "car=hat", "car=bike"}
	if sw.getEnv("car") != "hat" {
		t.Fatal("8 - 'car' should map to 'hat'")
	}
}

func TestProcessWords(t *testing.T) {
	test := "some content 'x foo x' \"a string arg\""
	words, err := ProcessWords(test, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if words[0] != "some" {
		t.Fatalf("%q != %q", words[0], "some")
	}
	if words[1] != "content" {
		t.Fatalf("%q != %q", words[1], "content")
	}
	if words[2] != "'x foo x'" {
		t.Fatalf("%q != %q", words[2], "'x foo x'")
	}
	if words[3] != "\"a string arg\"" {
		t.Fatalf("%q != %q", words[3], "\"a string arg\"")
	}
}
