package hotkey

import hk "golang.design/x/hotkey"

var (
	modAlt   = hk.Mod1 // Alt on most X11 configurations
	modSuper = hk.Mod4 // Super/Win on most X11 configurations
)

// letterKeys maps letter offset (A=0..Z=25) to X11 keysym.
var letterKeys = [26]hk.Key{
	hk.KeyA, hk.KeyB, hk.KeyC, hk.KeyD, hk.KeyE, hk.KeyF, hk.KeyG,
	hk.KeyH, hk.KeyI, hk.KeyJ, hk.KeyK, hk.KeyL, hk.KeyM, hk.KeyN,
	hk.KeyO, hk.KeyP, hk.KeyQ, hk.KeyR, hk.KeyS, hk.KeyT, hk.KeyU,
	hk.KeyV, hk.KeyW, hk.KeyX, hk.KeyY, hk.KeyZ,
}

// digitKeys maps digit offset ('0'=0..'9'=9) to X11 keysym.
var digitKeys = [10]hk.Key{
	hk.Key0, hk.Key1, hk.Key2, hk.Key3, hk.Key4,
	hk.Key5, hk.Key6, hk.Key7, hk.Key8, hk.Key9,
}
