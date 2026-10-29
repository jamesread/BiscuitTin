package tray

import (
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

const (
	menuShow int32 = 1
	menuExit int32 = 2
)

type menu struct {
	emit func(Event)
}

type menuNode struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}

type menuProps struct {
	ID         int32
	Properties map[string]dbus.Variant
}

type menuEvent struct {
	ID        int32
	EventID   string
	Data      dbus.Variant
	Timestamp uint32
}

func exportMenu(conn *dbus.Conn, m *menu) error {
	props, err := prop.Export(conn, menuPath, menuPropSpec())
	if err != nil {
		return err
	}
	if err := conn.Export(m, menuPath, menuIface); err != nil {
		return err
	}
	return exportIntrospect(conn, menuPath, menuIface, m, props)
}

func menuPropSpec() prop.Map {
	return prop.Map{
		menuIface: {
			"Version":       ro(uint32(3)),
			"TextDirection": ro("ltr"),
			"Status":        ro("normal"),
			"IconThemePath": ro([]string{}),
		},
	}
}

func (m *menu) GetLayout(_, _ int32, _ []string) (uint32, menuNode, *dbus.Error) {
	return 1, menuRoot(), nil
}

func (m *menu) GetGroupProperties(ids []int32, _ []string) ([]menuProps, *dbus.Error) {
	var out []menuProps
	for _, id := range ids {
		out = appendMenuProps(out, id)
	}
	return out, nil
}

func appendMenuProps(out []menuProps, id int32) []menuProps {
	p, ok := propsFor(id)
	if !ok {
		return out
	}
	return append(out, p)
}

func (m *menu) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	p, ok := propsFor(id)
	if !ok {
		return dbus.MakeVariant(""), nil
	}
	v, found := p.Properties[name]
	if !found {
		return dbus.MakeVariant(""), nil
	}
	return v, nil
}

func (m *menu) Event(id int32, eventId string, _ dbus.Variant, _ uint32) *dbus.Error {
	if eventId != "clicked" {
		return nil
	}
	m.clicked(id)
	return nil
}

func (m *menu) EventGroup(events []menuEvent) ([]int32, *dbus.Error) {
	for _, ev := range events {
		_ = m.Event(ev.ID, ev.EventID, ev.Data, ev.Timestamp)
	}
	return nil, nil
}

func (*menu) AboutToShow(_ int32) (bool, *dbus.Error) { return false, nil }

func (*menu) AboutToShowGroup(_ []int32) ([]int32, []int32, *dbus.Error) {
	return nil, nil, nil
}

func (m *menu) clicked(id int32) {
	if id == menuExit {
		m.emit(EventQuit)
		return
	}
	if id == menuShow {
		m.emit(EventShow)
	}
}

func menuRoot() menuNode {
	return menuNode{
		ID: 0,
		Properties: map[string]dbus.Variant{
			"children-display": dbus.MakeVariant("submenu"),
		},
		Children: []dbus.Variant{
			dbus.MakeVariant(menuLeaf(menuShow, "Show BiscuitTin")),
			dbus.MakeVariant(menuLeaf(menuExit, "Exit")),
		},
	}
}

func menuLeaf(id int32, label string) menuNode {
	return menuNode{ID: id, Properties: standardItem(label), Children: []dbus.Variant{}}
}

func propsFor(id int32) (menuProps, bool) {
	label, ok := menuLabel(id)
	if !ok {
		return menuProps{}, false
	}
	return menuProps{ID: id, Properties: standardItem(label)}, true
}

func menuLabel(id int32) (string, bool) {
	switch id {
	case menuShow:
		return "Show BiscuitTin", true
	case menuExit:
		return "Exit", true
	}
	return "", false
}

func standardItem(label string) map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"type":  dbus.MakeVariant("standard"),
		"label": dbus.MakeVariant(label),
	}
}
