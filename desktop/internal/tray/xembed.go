package tray

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"

	"github.com/jamesread/biscuittin/internal/icons"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

const (
	xembedMapped   = 1
	xembedTraySize = 24
	dockOpcode     = 0
)

type xembed struct {
	emit       func(Event)
	x          *xgb.Conn
	screen     *xproto.ScreenInfo
	img        *image.RGBA
	owner      xproto.Window
	opcode     xproto.Atom
	xembedInfo xproto.Atom
	gc         xproto.Gcontext
	win        xproto.Window
	w          uint16
	h          uint16
	depth      byte
}

func (h *Host) serveXEmbed() error {
	x, err := xgb.NewConn()
	if err != nil {
		return fmt.Errorf("x11: %w", err)
	}
	xe := &xembed{x: x, emit: h.emit}
	if err := xe.dock(); err != nil {
		x.Close()
		return err
	}
	h.xe = xe
	go xe.loop()
	return nil
}

func (xe *xembed) close() {
	if xe == nil || xe.x == nil {
		return
	}
	xe.x.Close()
	xe.x = nil
}

func (xe *xembed) dock() error {
	if err := xe.prepare(); err != nil {
		return err
	}
	return xe.requestDock()
}

func (xe *xembed) prepare() error {
	if err := xe.init(); err != nil {
		return err
	}
	if err := xe.findOwner(); err != nil {
		return err
	}
	return xe.createWindow()
}

func (xe *xembed) init() error {
	setup := xproto.Setup(xe.x)
	xe.screen = setup.DefaultScreen(xe.x)
	xe.depth = xe.screen.RootDepth
	if err := xe.internAtoms(); err != nil {
		return err
	}
	img, err := icons.RasterizeSVG(iconSVG, iconSize)
	if err != nil {
		return fmt.Errorf("tray icon: %w", err)
	}
	xe.img = img
	return nil
}

func (xe *xembed) internAtoms() error {
	var err error
	xe.opcode, err = internAtom(xe.x, "_NET_SYSTEM_TRAY_OPCODE")
	if err != nil {
		return err
	}
	xe.xembedInfo, err = internAtom(xe.x, "_XEMBED_INFO")
	return err
}

func internAtom(x *xgb.Conn, name string) (xproto.Atom, error) {
	reply, err := xproto.InternAtom(x, false, uint16(len(name)), name).Reply()
	if err != nil {
		return 0, err
	}
	return reply.Atom, nil
}

func (xe *xembed) findOwner() error {
	name := fmt.Sprintf("_NET_SYSTEM_TRAY_S%d", xe.x.DefaultScreen)
	owner, err := selectionOwner(xe.x, name)
	if err != nil {
		return err
	}
	if owner == 0 {
		return errors.New("no xembed tray")
	}
	xe.owner = owner
	return nil
}

func selectionOwner(x *xgb.Conn, name string) (xproto.Window, error) {
	sel, err := internAtom(x, name)
	if err != nil {
		return 0, err
	}
	reply, err := xproto.GetSelectionOwner(x, sel).Reply()
	if err != nil {
		return 0, err
	}
	return reply.Owner, nil
}

func (xe *xembed) createWindow() error {
	win, err := xproto.NewWindowId(xe.x)
	if err != nil {
		return err
	}
	xe.win = win
	xe.w, xe.h = xembedTraySize, xembedTraySize
	if err := xe.createWinChecked(); err != nil {
		return err
	}
	return xe.setupWindow()
}

func (xe *xembed) createWinChecked() error {
	s := xe.screen
	mask := uint32(xproto.CwBackPixmap | xproto.CwBorderPixel | xproto.CwEventMask)
	vals := []uint32{
		xproto.BackPixmapParentRelative,
		s.BlackPixel,
		xproto.EventMaskExposure | xproto.EventMaskButtonPress | xproto.EventMaskStructureNotify,
	}
	return xproto.CreateWindowChecked(xe.x, s.RootDepth, xe.win, s.Root, 0, 0, xe.w, xe.h, 0, xproto.WindowClassInputOutput, s.RootVisual, mask, vals).Check()
}

func (xe *xembed) setupWindow() error {
	xe.setProps()
	gc, err := xproto.NewGcontextId(xe.x)
	if err != nil {
		return err
	}
	xe.gc = gc
	return xproto.CreateGCChecked(xe.x, gc, xproto.Drawable(xe.win), 0, nil).Check()
}

func (xe *xembed) setProps() {
	name := "BiscuitTin"
	_ = xproto.ChangePropertyChecked(xe.x, xproto.PropModeReplace, xe.win, xproto.AtomWmName, xproto.AtomString, 8, uint32(len(name)), []byte(name)).Check()
	class := []byte("biscuittin\x00BiscuitTin\x00")
	_ = xproto.ChangePropertyChecked(xe.x, xproto.PropModeReplace, xe.win, xproto.AtomWmClass, xproto.AtomString, 8, uint32(len(class)), class).Check()
	_ = xproto.ChangePropertyChecked(xe.x, xproto.PropModeReplace, xe.win, xe.xembedInfo, xe.xembedInfo, 32, 2, card32s(0, xembedMapped)).Check()
}

func card32s(vals ...uint32) []byte {
	b := make([]byte, 4*len(vals))
	for i, v := range vals {
		binary.LittleEndian.PutUint32(b[i*4:], v)
	}
	return b
}

func (xe *xembed) requestDock() error {
	ev := xproto.ClientMessageEvent{
		Format: 32,
		Window: xe.owner,
		Type:   xe.opcode,
		Data:   xproto.ClientMessageDataUnionData32New([]uint32{0, dockOpcode, uint32(xe.win), 0, 0}),
	}
	if err := xproto.SendEventChecked(xe.x, false, xe.owner, xproto.EventMaskNoEvent, string(ev.Bytes())).Check(); err != nil {
		return err
	}
	_, _ = xproto.GetInputFocus(xe.x).Reply()
	return nil
}

func (xe *xembed) loop() {
	for {
		ev, err := xe.x.WaitForEvent()
		if ev == nil && err == nil {
			return
		}
		if ev != nil {
			xe.handle(ev)
		}
	}
}

func (xe *xembed) handle(ev xgb.Event) {
	switch e := ev.(type) {
	case xproto.ExposeEvent:
		xe.onExpose(e)
	case xproto.ConfigureNotifyEvent:
		xe.onConfigure(e)
	case xproto.ButtonPressEvent:
		xe.emit(EventShow)
	}
}

func (xe *xembed) onExpose(e xproto.ExposeEvent) {
	if e.Count == 0 {
		xe.redraw()
	}
}

func (xe *xembed) onConfigure(e xproto.ConfigureNotifyEvent) {
	xe.w, xe.h = e.Width, e.Height
	xe.redraw()
}
