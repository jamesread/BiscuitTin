package tray

import (
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

type sni struct {
	emit func(Event)
}

func (i *sni) Activate(_, _ int32) *dbus.Error {
	i.emit(EventShow)
	return nil
}

func (i *sni) SecondaryActivate(_, _ int32) *dbus.Error {
	i.emit(EventShow)
	return nil
}

func (*sni) ContextMenu(_, _ int32) *dbus.Error { return nil }

func (*sni) Scroll(_ int32, _ string) *dbus.Error { return nil }

func exportItem(conn *dbus.Conn, item *sni, pix []iconPixmap) error {
	props, err := prop.Export(conn, itemPath, itemProps(pix))
	if err != nil {
		return err
	}
	if err := conn.Export(item, itemPath, itemIface); err != nil {
		return err
	}
	return exportIntrospect(conn, itemPath, itemIface, item, props)
}

func exportIntrospect(conn *dbus.Conn, path dbus.ObjectPath, iface string, obj any, props *prop.Properties) error {
	n := &introspect.Node{
		Name: string(path),
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{
				Name:       iface,
				Methods:    introspect.Methods(obj),
				Properties: props.Introspection(iface),
			},
		},
	}
	return conn.Export(introspect.NewIntrospectable(n), path, "org.freedesktop.DBus.Introspectable")
}

func itemProps(pix []iconPixmap) prop.Map {
	return prop.Map{
		itemIface: {
			"Category":      ro("ApplicationStatus"),
			"Id":            ro("biscuittin"),
			"Title":         ro("BiscuitTin"),
			"Status":        ro("Active"),
			"WindowId":      ro(int32(0)),
			"IconName":      ro("io.github.jamesread.biscuittin"),
			"IconPixmap":    ro(pix),
			"IconThemePath": ro(""),
			"ToolTip":       ro(toolTip{Title: "BiscuitTin", Text: "Full-screen app launcher"}),
			"ItemIsMenu":    ro(false),
			"Menu":          ro(menuPath),
		},
	}
}

func ro(v any) *prop.Prop {
	return &prop.Prop{Value: v, Writable: false, Emit: prop.EmitConst}
}

type toolTip struct {
	Icon  string
	Icons []iconPixmap
	Title string
	Text  string
}
