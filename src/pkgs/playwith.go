package pkgs

import (
	"fmt"
	"log"
	"os/exec"
)

type PlayerType int

const (
	VLC PlayerType = iota
	MPV
	Browser
	Unknown PlayerType = -1
)

type PlayerFunc func(urlOrPath string) PlayerType

func WithVLC(urlOrPath string) PlayerType {
	cmd := exec.Command("vlc", "--fullscreen", urlOrPath)
	if err := cmd.Start(); err != nil {
		log.Printf("players: vlc failed to start: %v", err)
		return Unknown
	}
	log.Printf("players: vlc started (pid=%d) path=%q", cmd.Process.Pid, urlOrPath)
	return VLC
}

func WithMPV(urlOrPath string) PlayerType {
	cmd := exec.Command("mpv", "--fs", urlOrPath)
	if err := cmd.Start(); err != nil {
		log.Printf("players: mpv failed to start: %v", err)
		return Unknown
	}
	log.Printf("players: mpv started (pid=%d) path=%q", cmd.Process.Pid, urlOrPath)
	return MPV
}

func WithBrowser(urlOrPath string) PlayerType {
	return Browser
}

type PlayResult struct {
	Type PlayerType
	Link string
}

func Play(urlOrPath string, fn PlayerFunc) (*PlayResult, error) {
	pt := fn(urlOrPath)

	switch pt {
	case Browser:
		return &PlayResult{Type: Browser, Link: urlOrPath}, nil
	case VLC, MPV:
		return &PlayResult{Type: pt}, nil
	default:
		return nil, fmt.Errorf("players: failed to launch player for %q", urlOrPath)
	}
}
