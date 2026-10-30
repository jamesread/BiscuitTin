package app

import (
	"embed"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go.uber.org/zap"
)

// Menu ticks generated with ElevenLabs, trimmed to a short fade.
//
//go:embed assets/sounds/*.wav
var soundWAVs embed.FS

type uiSounds struct {
	move    rl.Sound
	confirm rl.Sound
	back    rl.Sound
}

func loadSounds() uiSounds {
	return uiSounds{
		move:    loadSound("move.wav"),
		confirm: loadSound("confirm.wav"),
		back:    loadSound("back.wav"),
	}
}

func loadSound(name string) rl.Sound {
	raw, err := soundWAVs.ReadFile("assets/sounds/" + name)
	if err != nil {
		lg().Warn("sound missing", zap.String("name", name), zap.Error(err))
		return rl.Sound{}
	}
	wave := rl.LoadWaveFromMemory(".wav", raw, int32(len(raw)))
	if wave.FrameCount == 0 {
		lg().Warn("sound unreadable", zap.String("name", name))
		return rl.Sound{}
	}
	snd := rl.LoadSoundFromWave(wave)
	rl.UnloadWave(wave)
	return snd
}

func (s uiSounds) unload() {
	unloadSound(s.move)
	unloadSound(s.confirm)
	unloadSound(s.back)
}

func unloadSound(s rl.Sound) {
	if s.FrameCount == 0 {
		return
	}
	rl.UnloadSound(s)
}

func soundForAction(a action, s uiSounds) rl.Sound {
	switch a {
	case actionUp, actionDown, actionLeft, actionRight:
		return s.move
	case actionActivate:
		return s.confirm
	case actionBack, actionExit:
		return s.back
	}
	return rl.Sound{}
}

func (u *ui) playAction(a action) {
	if u.menuSoundsMuted() && !(u.screen == screenAudio && a == actionActivate) {
		return
	}
	u.playSound(soundForAction(a, u.sounds))
}

// menuSoundsMuted is the Settings choice; the Audio diagnostics test sound
// plays regardless.
func (u *ui) menuSoundsMuted() bool {
	return u.settings != nil && u.settings.MuteMenuSounds
}

func (u *ui) playSound(s rl.Sound) {
	if s.FrameCount == 0 {
		return
	}
	rl.PlaySound(s)
}
